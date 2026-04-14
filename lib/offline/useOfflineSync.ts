/**
 * Hook that periodically fetches the trip bundle from the backend and caches
 * it locally for offline access.
 *
 * Uses a `last_modified` timestamp for conditional fetches so we only
 * re-download when data has actually changed.
 *
 * The backend endpoint is `GET /api/trips/{tripId}/bundle`. If this endpoint
 * returns 404 (not yet deployed), the hook gracefully no-ops.
 */
import { useEffect, useRef, useCallback, useState } from "react";
import { useAuth } from "@/lib/auth";
import { useNetworkStatus } from "@/lib/hooks/useNetworkStatus";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";
import {
  offlineStorage,
  bundleKey,
  syncMetaKey,
  type OfflineTripBundle,
  type OfflineSyncMeta,
} from "./offlineStorage";

/** How often to attempt a background sync (5 minutes) */
const SYNC_INTERVAL_MS = 5 * 60 * 1000;

export interface OfflineSyncState {
  /** ISO timestamp of the last successful sync, or null if never synced */
  lastSyncedAt: string | null;
  /** Whether a sync is currently in progress */
  isSyncing: boolean;
  /** The last error encountered during sync, if any */
  syncError: string | null;
  /** Manually trigger a sync */
  syncNow: () => Promise<void>;
}

export function useOfflineSync(tripId: string | undefined): OfflineSyncState {
  const { accessToken } = useAuth();
  const { isConnected } = useNetworkStatus();
  const [lastSyncedAt, setLastSyncedAt] = useState<string | null>(null);
  const [isSyncing, setIsSyncing] = useState(false);
  const [syncError, setSyncError] = useState<string | null>(null);
  const mountedRef = useRef(true);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Load persisted sync metadata on mount
  useEffect(() => {
    if (!tripId) return;
    void offlineStorage.get<OfflineSyncMeta>(syncMetaKey(tripId)).then((meta) => {
      if (meta?.lastSyncedAt && mountedRef.current) {
        setLastSyncedAt(meta.lastSyncedAt);
      }
    });
  }, [tripId]);

  const performSync = useCallback(async () => {
    if (!tripId || !accessToken) return;

    setIsSyncing(true);
    setSyncError(null);

    try {
      // Read existing sync metadata for conditional fetch
      const existingMeta = await offlineStorage.get<OfflineSyncMeta>(syncMetaKey(tripId));
      const lastModified = existingMeta?.lastModified ?? "";

      const url = new URL(`/api/trips/${encodeURIComponent(tripId)}/bundle`, getConfig().apiUrl);
      if (lastModified) {
        url.searchParams.set("last_modified", lastModified);
      }

      const res = await authFetch(url.toString(), accessToken);

      if (res.status === 304) {
        // Not modified — update sync timestamp only
        const now = new Date().toISOString();
        const meta: OfflineSyncMeta = {
          lastSyncedAt: now,
          lastModified: existingMeta?.lastModified ?? "",
        };
        await offlineStorage.set(syncMetaKey(tripId), meta);
        if (mountedRef.current) setLastSyncedAt(now);
        return;
      }

      if (res.status === 404) {
        // Endpoint not deployed yet — silently skip
        return;
      }

      if (!res.ok) {
        throw new Error(`Bundle fetch failed (${res.status})`);
      }

      const bundle: OfflineTripBundle = await res.json();
      await offlineStorage.set(bundleKey(tripId), bundle);

      const now = new Date().toISOString();
      const meta: OfflineSyncMeta = {
        lastSyncedAt: now,
        lastModified: bundle.lastModified ?? now,
      };
      await offlineStorage.set(syncMetaKey(tripId), meta);
      if (mountedRef.current) setLastSyncedAt(now);
    } catch (err) {
      if (mountedRef.current) {
        setSyncError(err instanceof Error ? err.message : "Sync failed");
      }
    } finally {
      if (mountedRef.current) setIsSyncing(false);
    }
  }, [tripId, accessToken]);

  // Sync on mount and periodically while online
  useEffect(() => {
    mountedRef.current = true;

    if (!tripId || !accessToken || !isConnected) {
      return () => {
        mountedRef.current = false;
      };
    }

    // Initial sync
    void performSync();

    // Periodic background sync
    intervalRef.current = setInterval(() => {
      void performSync();
    }, SYNC_INTERVAL_MS);

    return () => {
      mountedRef.current = false;
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [tripId, accessToken, isConnected, performSync]);

  const syncNow = useCallback(async () => {
    await performSync();
  }, [performSync]);

  return { lastSyncedAt, isSyncing, syncError, syncNow };
}
