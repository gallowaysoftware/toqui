/**
 * Hook that returns trip data from the offline cache when the device is
 * offline, falling back to network-sourced data when online.
 *
 * Consumers get a unified view of trip, itinerary, bookings, and messages
 * regardless of connectivity.
 */
import { useState, useEffect } from "react";
import { useNetworkStatus } from "@/lib/hooks/useNetworkStatus";
import {
  offlineStorage,
  bundleKey,
  syncMetaKey,
  type OfflineTripBundle,
  type OfflineSyncMeta,
} from "./offlineStorage";

export interface OfflineTripData {
  /** The cached trip bundle, if available */
  bundle: OfflineTripBundle | null;
  /** Whether the device is currently offline */
  isOffline: boolean;
  /** Whether we have cached data available for this trip */
  hasCachedData: boolean;
  /** Whether we're currently loading cached data */
  isLoadingCache: boolean;
  /** ISO timestamp of when data was last synced */
  lastSyncedAt: string | null;
}

export function useOfflineTrip(tripId: string | undefined): OfflineTripData {
  const { isConnected } = useNetworkStatus();
  const [bundle, setBundle] = useState<OfflineTripBundle | null>(null);
  const [isLoadingCache, setIsLoadingCache] = useState(true);
  const [lastSyncedAt, setLastSyncedAt] = useState<string | null>(null);

  useEffect(() => {
    if (!tripId) {
      setBundle(null);
      setIsLoadingCache(false);
      return;
    }

    let cancelled = false;
    setIsLoadingCache(true);

    void (async () => {
      const [cached, meta] = await Promise.all([
        offlineStorage.get<OfflineTripBundle>(bundleKey(tripId)),
        offlineStorage.get<OfflineSyncMeta>(syncMetaKey(tripId)),
      ]);
      if (!cancelled) {
        setBundle(cached);
        setLastSyncedAt(meta?.lastSyncedAt ?? null);
        setIsLoadingCache(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [tripId]);

  return {
    bundle,
    isOffline: !isConnected,
    hasCachedData: bundle !== null,
    isLoadingCache,
    lastSyncedAt,
  };
}
