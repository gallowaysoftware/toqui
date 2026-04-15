import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
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

const mockAccessToken = "test-token";
vi.mock("@/lib/auth", () => ({
  useAuth: () => ({ accessToken: mockAccessToken }),
}));

let mockIsConnected = true;
vi.mock("@/lib/hooks/useNetworkStatus", () => ({
  useNetworkStatus: () => ({
    isConnected: mockIsConnected,
    isInternetReachable: mockIsConnected,
  }),
}));

vi.mock("@/lib/config", () => ({
  getConfig: () => ({ apiUrl: "http://localhost:8090" }),
}));

const mockFetch = vi.fn();
vi.mock("@/lib/authFetch", () => ({
  authFetch: (...args: unknown[]) => mockFetch(...args),
}));

import { useOfflineSync } from "../useOfflineSync";

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => {
  mockIsConnected = true;
  localStorage.clear();
  mockFetch.mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useOfflineSync", () => {
  it("does not sync when offline", () => {
    mockIsConnected = false;

    renderHook(() => useOfflineSync("trip-1"));

    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("does not sync when tripId is undefined", () => {
    renderHook(() => useOfflineSync(undefined));

    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("gracefully handles 404 when endpoint does not exist", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      json: () => Promise.resolve({}),
    });

    const { result } = renderHook(() => useOfflineSync("trip-1"));

    await waitFor(() => {
      // Fetch was called, and no error is set for 404
      expect(mockFetch).toHaveBeenCalled();
    });

    await waitFor(() => {
      expect(result.current.isSyncing).toBe(false);
    });

    expect(result.current.syncError).toBeNull();
  });

  it("stores bundle and updates lastSyncedAt on successful sync", async () => {
    const bundle = {
      trip: { id: "t1", title: "Test Trip", description: "", startDate: "", endDate: "", status: 1 },
      itinerary: null,
      bookings: [],
      messages: [],
      lastModified: "2024-06-01T12:00:00Z",
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve(bundle),
    });

    const { result } = renderHook(() => useOfflineSync("trip-1"));

    await waitFor(() => {
      expect(result.current.lastSyncedAt).not.toBeNull();
    });

    // Should have stored the bundle
    const stored = localStorage.getItem("toqui_offline_bundle_trip-1");
    expect(stored).not.toBeNull();
    const parsed = JSON.parse(stored!);
    expect(parsed.trip.title).toBe("Test Trip");
  });

  it("sets syncError on fetch failure", async () => {
    mockFetch.mockRejectedValue(new Error("Network error"));

    const { result } = renderHook(() => useOfflineSync("trip-1"));

    await waitFor(() => {
      expect(result.current.syncError).toBe("Network error");
    });
  });

  it("provides a syncNow function", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({
        trip: { id: "t1", title: "Manual", description: "", startDate: "", endDate: "", status: 1 },
        itinerary: null,
        bookings: [],
        messages: [],
        lastModified: "2024-06-01T12:00:00Z",
      }),
    });

    const { result } = renderHook(() => useOfflineSync("trip-1"));

    await waitFor(() => {
      expect(result.current.lastSyncedAt).not.toBeNull();
    });

    expect(typeof result.current.syncNow).toBe("function");
  });

  it("handles 304 not modified response", async () => {
    // First call returns a bundle, second returns 304
    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: () => Promise.resolve({
          trip: { id: "t1", title: "Test", description: "", startDate: "", endDate: "", status: 1 },
          itinerary: null,
          bookings: [],
          messages: [],
          lastModified: "2024-06-01T12:00:00Z",
        }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 304,
      });

    const { result } = renderHook(() => useOfflineSync("trip-1"));

    await waitFor(() => {
      expect(result.current.lastSyncedAt).not.toBeNull();
    });

    // Second sync (manual)
    await result.current.syncNow();

    // Should still have lastSyncedAt set
    expect(result.current.lastSyncedAt).not.toBeNull();
    expect(result.current.syncError).toBeNull();
  });
});
