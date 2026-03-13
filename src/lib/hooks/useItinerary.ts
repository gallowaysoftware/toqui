"use client";

import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { useTransport } from "@/components/providers/GrpcProvider";
import { TripService } from "@/gen/toqui/v1/trip_pb";
import { useAuth } from "@/components/providers/AuthProvider";

export function useItinerary(tripId: string) {
  const transport = useTransport();
  const { user } = useAuth();
  const client = useMemo(() => createClient(TripService, transport), [transport]);

  const { data: itinerary, isLoading } = useQuery({
    queryKey: ["itinerary", tripId],
    queryFn: async () => {
      const res = await client.getItinerary({ tripId });
      return res.itinerary;
    },
    enabled: !!user && !!tripId,
  });

  return { itinerary, isLoading };
}
