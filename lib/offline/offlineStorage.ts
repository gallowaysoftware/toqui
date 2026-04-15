/**
 * Cross-platform offline storage abstraction.
 *
 * Uses AsyncStorage on native (backed by SQLite / SharedPreferences) and
 * localStorage on web. All values are JSON-serialized before storage.
 *
 * Keys are prefixed with `toqui_offline_` to avoid collisions.
 */
import { Platform } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";

const KEY_PREFIX = "toqui_offline_";

function prefixedKey(key: string): string {
  return `${KEY_PREFIX}${key}`;
}

export const offlineStorage = {
  async get<T>(key: string): Promise<T | null> {
    const fullKey = prefixedKey(key);
    if (Platform.OS === "web") {
      const raw = localStorage.getItem(fullKey);
      if (raw === null) return null;
      try {
        return JSON.parse(raw) as T;
      } catch {
        return null;
      }
    }
    const raw = await AsyncStorage.getItem(fullKey);
    if (raw === null) return null;
    try {
      return JSON.parse(raw) as T;
    } catch {
      return null;
    }
  },

  async set<T>(key: string, value: T): Promise<void> {
    const fullKey = prefixedKey(key);
    const serialized = JSON.stringify(value);
    if (Platform.OS === "web") {
      localStorage.setItem(fullKey, serialized);
      return;
    }
    await AsyncStorage.setItem(fullKey, serialized);
  },

  async remove(key: string): Promise<void> {
    const fullKey = prefixedKey(key);
    if (Platform.OS === "web") {
      localStorage.removeItem(fullKey);
      return;
    }
    await AsyncStorage.removeItem(fullKey);
  },
};

// ---------------------------------------------------------------------------
// Trip bundle types — matches expected backend response shape from
// GET /api/trips/{tripId}/bundle
// ---------------------------------------------------------------------------

export interface OfflineTripBundle {
  trip: {
    id: string;
    title: string;
    description: string;
    startDate: string;
    endDate: string;
    status: number;
    destinationCountry?: string;
    isUnlocked?: boolean;
    userId?: string;
  };
  itinerary: {
    days: Array<{
      date: string;
      dayNumber: number;
      items: Array<{
        id: string;
        title: string;
        description: string;
        startTime: string;
        endTime: string;
        location?: {
          name: string;
          latitude: number;
          longitude: number;
        };
        category: string;
      }>;
    }>;
  } | null;
  bookings: Array<{
    id: string;
    title: string;
    type: number;
    provider: string;
    confirmationCode: string;
  }>;
  messages: Array<{
    id: string;
    role: string;
    content: string;
    metadata: Record<string, string>;
  }>;
  lastModified: string;
}

export interface OfflineSyncMeta {
  lastSyncedAt: string; // ISO timestamp
  lastModified: string; // Server-provided last_modified
}

/** Storage keys for a given trip */
export function bundleKey(tripId: string): string {
  return `bundle_${tripId}`;
}

export function syncMetaKey(tripId: string): string {
  return `sync_meta_${tripId}`;
}
