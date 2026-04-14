import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock react-native with web platform
vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Platform: { OS: "web" },
  };
});

vi.mock("@react-native-async-storage/async-storage", () => ({
  default: {
    getItem: vi.fn(),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  },
}));

import {
  offlineStorage,
  bundleKey,
  syncMetaKey,
  type OfflineTripBundle,
  type OfflineSyncMeta,
} from "../offlineStorage";

beforeEach(() => {
  localStorage.clear();
});

describe("offlineStorage", () => {
  describe("get", () => {
    it("returns null for missing key", async () => {
      const result = await offlineStorage.get<string>("nonexistent");
      expect(result).toBeNull();
    });

    it("returns parsed JSON for existing key", async () => {
      localStorage.setItem("toqui_offline_test", JSON.stringify({ foo: "bar" }));
      const result = await offlineStorage.get<{ foo: string }>("test");
      expect(result).toEqual({ foo: "bar" });
    });

    it("returns null for invalid JSON", async () => {
      localStorage.setItem("toqui_offline_bad", "not-json{");
      const result = await offlineStorage.get("bad");
      expect(result).toBeNull();
    });
  });

  describe("set", () => {
    it("serializes and stores value", async () => {
      await offlineStorage.set("key1", { hello: "world" });
      const stored = localStorage.getItem("toqui_offline_key1");
      expect(stored).toBe(JSON.stringify({ hello: "world" }));
    });
  });

  describe("remove", () => {
    it("removes the key", async () => {
      localStorage.setItem("toqui_offline_removeme", '"value"');
      await offlineStorage.remove("removeme");
      expect(localStorage.getItem("toqui_offline_removeme")).toBeNull();
    });
  });
});

describe("key helpers", () => {
  it("bundleKey returns correct key", () => {
    expect(bundleKey("trip-123")).toBe("bundle_trip-123");
  });

  it("syncMetaKey returns correct key", () => {
    expect(syncMetaKey("trip-456")).toBe("sync_meta_trip-456");
  });
});

describe("type shapes", () => {
  it("OfflineTripBundle is assignable with expected fields", () => {
    const bundle: OfflineTripBundle = {
      trip: {
        id: "t1",
        title: "Test",
        description: "A trip",
        startDate: "2024-01-01",
        endDate: "2024-01-07",
        status: 1,
      },
      itinerary: null,
      bookings: [],
      messages: [],
      lastModified: "2024-01-01T00:00:00Z",
    };
    expect(bundle.trip.id).toBe("t1");
  });

  it("OfflineSyncMeta tracks sync timestamps", () => {
    const meta: OfflineSyncMeta = {
      lastSyncedAt: "2024-01-01T12:00:00Z",
      lastModified: "2024-01-01T11:00:00Z",
    };
    expect(meta.lastSyncedAt).toBe("2024-01-01T12:00:00Z");
  });
});
