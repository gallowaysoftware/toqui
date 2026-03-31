import { useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { useTransport } from "@/lib/transport";
import { useAuth } from "@/lib/auth";
import { TripService } from "@gen/toqui/v1/trip_pb";

export function useTrips() {
  const transport = useTransport();
  const { accessToken } = useAuth();
  const client = useMemo(() => createClient(TripService, transport), [transport]);

  const { data: trips = [], isLoading, error, isError } = useQuery({
    queryKey: ["trips"],
    queryFn: async () => {
      const res = await client.listTrips({ pagination: { pageSize: 50 } });
      return res.trips;
    },
    enabled: !!accessToken,
  });

  return { trips, isLoading, error, isError };
}

export function useTrip(tripId: string) {
  const transport = useTransport();
  const { accessToken } = useAuth();
  const client = useMemo(() => createClient(TripService, transport), [transport]);

  const { data: trip, isLoading, error } = useQuery({
    queryKey: ["trip", tripId],
    queryFn: async () => {
      const res = await client.getTrip({ id: tripId });
      return res.trip;
    },
    enabled: !!accessToken && !!tripId,
  });

  return { trip, isLoading, error };
}

export function useCreateTrip() {
  const transport = useTransport();
  const queryClient = useQueryClient();
  const client = useMemo(() => createClient(TripService, transport), [transport]);

  return useMutation({
    mutationFn: async (params: {
      title: string;
      description?: string;
      startDate?: string;
      endDate?: string;
    }) => {
      const res = await client.createTrip(params);
      return res.trip;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["trips"] });
    },
  });
}

export function useUpdateTrip() {
  const transport = useTransport();
  const queryClient = useQueryClient();
  const client = useMemo(() => createClient(TripService, transport), [transport]);

  return useMutation({
    mutationFn: async (params: {
      id: string;
      title?: string;
      description?: string;
      status?: number;
      startDate?: string;
      endDate?: string;
    }) => {
      const res = await client.updateTrip(params);
      return res.trip;
    },
    onSuccess: (trip) => {
      if (trip) {
        void queryClient.invalidateQueries({ queryKey: ["trip", trip.id] });
      }
      void queryClient.invalidateQueries({ queryKey: ["trips"] });
    },
  });
}

export function useDeleteTrip() {
  const transport = useTransport();
  const queryClient = useQueryClient();
  const client = useMemo(() => createClient(TripService, transport), [transport]);

  return useMutation({
    mutationFn: async (tripId: string) => {
      await client.deleteTrip({ id: tripId });
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["trips"] });
    },
  });
}
