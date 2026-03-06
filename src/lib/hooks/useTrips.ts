"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { useTransport } from "@/components/providers/GrpcProvider";
import { TripService } from "@/gen/toqui/v1/trip_pb";
import { useAuth } from "@/components/providers/AuthProvider";

export function useTrips() {
  const transport = useTransport();
  const { user } = useAuth();
  const client = createClient(TripService, transport);

  const { data: trips = [], isLoading } = useQuery({
    queryKey: ["trips"],
    queryFn: async () => {
      const res = await client.listTrips({ pagination: { pageSize: 50 } });
      return res.trips;
    },
    enabled: !!user,
  });

  return { trips, isLoading };
}

export function useTrip(tripId: string) {
  const transport = useTransport();
  const { user } = useAuth();
  const client = createClient(TripService, transport);

  const { data: trip, isLoading } = useQuery({
    queryKey: ["trip", tripId],
    queryFn: async () => {
      const res = await client.getTrip({ id: tripId });
      return res.trip;
    },
    enabled: !!user && !!tripId,
  });

  return { trip, isLoading };
}

export function useUpdateTrip() {
  const transport = useTransport();
  const queryClient = useQueryClient();
  const client = createClient(TripService, transport);

  return useMutation({
    mutationFn: async (params: { id: string; title?: string; description?: string; status?: number; startDate?: string; endDate?: string }) => {
      const res = await client.updateTrip(params);
      return res.trip;
    },
    onSuccess: (trip) => {
      if (trip) {
        queryClient.invalidateQueries({ queryKey: ["trip", trip.id] });
      }
      queryClient.invalidateQueries({ queryKey: ["trips"] });
    },
  });
}

export function useCreateTrip() {
  const transport = useTransport();
  const queryClient = useQueryClient();
  const client = createClient(TripService, transport);

  return useMutation({
    mutationFn: async (params: { title: string; description?: string; startDate?: string; endDate?: string }) => {
      const res = await client.createTrip(params);
      return res.trip;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["trips"] });
    },
  });
}
