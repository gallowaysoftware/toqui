import { useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { useTransport } from "@/lib/transport";
import { useAuth } from "@/lib/auth";
import { useAnalytics } from "@/lib/analytics";
import {
  BookingService,
  IngestBookingRequestSchema,
  ListBookingsRequestSchema,
  GetBookingRequestSchema,
  DeleteBookingRequestSchema,
} from "@gen/toqui/v1/booking_pb";
import type { BookingType } from "@gen/toqui/v1/booking_pb";

export function useBookings(tripId: string) {
  const transport = useTransport();
  const { accessToken } = useAuth();
  const client = useMemo(() => createClient(BookingService, transport), [transport]);

  const { data: bookings = [], isLoading, error } = useQuery({
    queryKey: ["bookings", tripId],
    queryFn: async () => {
      const res = await client.listBookings(
        create(ListBookingsRequestSchema, {
          tripId,
          pagination: { pageSize: 100, pageToken: "" },
        }),
      );
      return res.bookings;
    },
    enabled: !!accessToken && !!tripId,
  });

  return { bookings, isLoading, error };
}

export function useBooking(bookingId: string) {
  const transport = useTransport();
  const { accessToken } = useAuth();
  const client = useMemo(() => createClient(BookingService, transport), [transport]);

  const { data: booking, isLoading, error } = useQuery({
    queryKey: ["booking", bookingId],
    queryFn: async () => {
      const res = await client.getBooking(create(GetBookingRequestSchema, { id: bookingId }));
      return res.booking;
    },
    enabled: !!accessToken && !!bookingId,
  });

  return { booking, isLoading, error };
}

export function useIngestBooking() {
  const transport = useTransport();
  const queryClient = useQueryClient();
  const { track } = useAnalytics();
  const client = useMemo(() => createClient(BookingService, transport), [transport]);

  return useMutation({
    mutationFn: async (params: { tripId: string; type: BookingType; rawText: string }) => {
      const res = await client.ingestBooking(create(IngestBookingRequestSchema, params));
      return res.booking;
    },
    onSuccess: (_booking, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["bookings", variables.tripId] });
      // Funnel event — pairs with `trip_created` so the funnel shows
      // how many trips end up actually getting populated with real
      // bookings (vs trips that languish at "planning"). The booking
      // type enum is forwarded as `category` to use the existing
      // SAFE_PROPERTIES allowlist; raw rawText / parsed booking
      // content NEVER leaves the client (per CLAUDE.md privacy).
      track("booking_added", { category: bookingTypeName(variables.type) });
    },
  });
}

// bookingTypeName maps the BookingType numeric enum back to the human
// label that's safe to send to PostHog. Hard-coded rather than reflecting
// over the enum so a future enum addition is a deliberate "should this
// new type leak into analytics?" decision.
function bookingTypeName(t: BookingType): string {
  switch (t) {
    case 1: return "flight";
    case 2: return "hotel";
    case 3: return "car_rental";
    case 4: return "train";
    case 5: return "activity";
    case 6: return "restaurant";
    case 7: return "other";
    case 8: return "tour";
    case 9: return "ferry";
    case 10: return "bus";
    case 11: return "cruise";
    case 12: return "transfer";
    case 13: return "vacation_rental";
    default: return "unspecified";
  }
}

export function useDeleteBooking() {
  const transport = useTransport();
  const queryClient = useQueryClient();
  const client = useMemo(() => createClient(BookingService, transport), [transport]);

  return useMutation({
    mutationFn: async (params: { id: string; tripId: string }) => {
      await client.deleteBooking(create(DeleteBookingRequestSchema, { id: params.id }));
      return params;
    },
    onSuccess: (_result, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["bookings", variables.tripId] });
    },
  });
}
