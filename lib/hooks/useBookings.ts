import { useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { useTransport } from "@/lib/transport";
import { useAuth } from "@/lib/auth";
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
  const client = useMemo(() => createClient(BookingService, transport), [transport]);

  return useMutation({
    mutationFn: async (params: { tripId: string; type: BookingType; rawText: string }) => {
      const res = await client.ingestBooking(create(IngestBookingRequestSchema, params));
      return res.booking;
    },
    onSuccess: (_booking, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["bookings", variables.tripId] });
    },
  });
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
