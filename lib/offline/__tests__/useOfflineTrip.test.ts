import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

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

let mockIsConnected = true;
vi.mock("@/lib/hooks/useNetworkStatus", () => ({
  useNetworkStatus: () => ({
    isConnected: mockIsConnected,
    isInternetReachable: mockIsConnected,
  }),
}));

import { useOfflineTrip } from "../useOfflineTrip";
import type { OfflineTripBundle } from "../offlineStorage";

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => {
  mockIsConnected = true;
  localStorage.clear();
});

describe("useOfflineTrip", () => {
  it("returns null bundle when no cached data exists", async () => {
    const { result } = renderHook(() => useOfflineTrip("trip-1"));

    await waitFor(() => {
      expect(result.current.isLoadingCache).toBe(false);
    });

    expect(result.current.bundle).toBeNull();
    expect(result.current.hasCachedData).toBe(false);
  });

  it("returns cached bundle when data exists in storage", async () => {
    const bundle: OfflineTripBundle = {
      trip: {
        id: "trip-1",
        title: "Cached Trip",
        description: "A cached trip",
        startDate: "2024-06-01",
        endDate: "2024-06-07",
        status: 1,
      },
      itinerary: {
        days: [{
          date: "2024-06-01",
          dayNumber: 1,
          items: [{
            id: "item-1",
            title: "Visit Beach",
            description: "Go to the beach",
            startTime: "09:00",
            endTime: "12:00",
            category: "activity",
          }],
        }],
      },
      bookings: [{
        id: "b-1",
        title: "Hotel Booking",
        type: 2,
        provider: "Hotel Corp",
        confirmationCode: "ABC123",
      }],
      messages: [{
        id: "m-1",
        role: "assistant",
        content: "Welcome to your trip!",
        metadata: {},
      }],
      lastModified: "2024-06-01T12:00:00Z",
    };

    localStorage.setItem("toqui_offline_bundle_trip-1", JSON.stringify(bundle));

    const { result } = renderHook(() => useOfflineTrip("trip-1"));

    await waitFor(() => {
      expect(result.current.isLoadingCache).toBe(false);
    });

    expect(result.current.bundle).not.toBeNull();
    expect(result.current.bundle?.trip.title).toBe("Cached Trip");
    expect(result.current.hasCachedData).toBe(true);
  });

  it("reports isOffline based on network status", async () => {
    mockIsConnected = false;

    const { result } = renderHook(() => useOfflineTrip("trip-1"));

    await waitFor(() => {
      expect(result.current.isLoadingCache).toBe(false);
    });

    expect(result.current.isOffline).toBe(true);
  });

  it("reports online when connected", async () => {
    mockIsConnected = true;

    const { result } = renderHook(() => useOfflineTrip("trip-1"));

    await waitFor(() => {
      expect(result.current.isLoadingCache).toBe(false);
    });

    expect(result.current.isOffline).toBe(false);
  });

  it("returns null bundle when tripId is undefined", async () => {
    const { result } = renderHook(() => useOfflineTrip(undefined));

    await waitFor(() => {
      expect(result.current.isLoadingCache).toBe(false);
    });

    expect(result.current.bundle).toBeNull();
    expect(result.current.hasCachedData).toBe(false);
  });

  it("loads lastSyncedAt from sync metadata", async () => {
    localStorage.setItem(
      "toqui_offline_sync_meta_trip-1",
      JSON.stringify({
        lastSyncedAt: "2024-06-01T15:00:00Z",
        lastModified: "2024-06-01T12:00:00Z",
      }),
    );

    const { result } = renderHook(() => useOfflineTrip("trip-1"));

    await waitFor(() => {
      expect(result.current.isLoadingCache).toBe(false);
    });

    expect(result.current.lastSyncedAt).toBe("2024-06-01T15:00:00Z");
  });
});
