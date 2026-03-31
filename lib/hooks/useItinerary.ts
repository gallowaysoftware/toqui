import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { createClient } from "@connectrpc/connect";
import { useTransport } from "@/lib/transport";
import { useAuth } from "@/lib/auth";
import { TripService } from "@gen/toqui/v1/trip_pb";

export function useItinerary(tripId: string) {
  const transport = useTransport();
  const { accessToken } = useAuth();
  const client = useMemo(() => createClient(TripService, transport), [transport]);

  const { data: itinerary, isLoading } = useQuery({
    queryKey: ["itinerary", tripId],
    queryFn: async () => {
      const res = await client.getItinerary({ tripId });
      return res.itinerary;
    },
    enabled: !!accessToken && !!tripId,
  });

  const coveredDays = useMemo(() => {
    if (!itinerary?.days) return 0;
    const dates = new Set(itinerary.days.map((d) => d.date).filter(Boolean));
    return dates.size;
  }, [itinerary]);

  return { itinerary, isLoading, coveredDays };
}
