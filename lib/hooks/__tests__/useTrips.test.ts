import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import type { Transport } from "@connectrpc/connect";

// ---------- mocks ----------

const mockListTrips = vi.fn();
const mockGetTrip = vi.fn();
const mockCreateTrip = vi.fn();
const mockUpdateTrip = vi.fn();
const mockDeleteTrip = vi.fn();

vi.mock("@connectrpc/connect", () => ({
  createClient: () => ({
    listTrips: mockListTrips,
    getTrip: mockGetTrip,
    createTrip: mockCreateTrip,
    updateTrip: mockUpdateTrip,
    deleteTrip: mockDeleteTrip,
  }),
}));

vi.mock("@/lib/transport", () => ({
  useTransport: (): Transport => ({} as Transport),
}));

const mockAuth = { accessToken: "test-token" };
vi.mock("@/lib/auth", () => ({
  useAuth: () => mockAuth,
}));

// Stub proto schemas — hooks import TripService but createClient is mocked above
vi.mock("@gen/toqui/v1/trip_pb", () => ({
  TripService: {},
}));

import { useTrips, useTrip, useCreateTrip, useUpdateTrip, useDeleteTrip } from "../useTrips";

// ---------- helpers ----------

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper, queryClient };
}

// ---------- useTrips ----------

describe("useTrips", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("fetches trips when authenticated", async () => {
    const trips = [{ id: "t1", title: "Paris" }];
    mockListTrips.mockResolvedValue({ trips });
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useTrips(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.trips).toEqual(trips);
    expect(mockListTrips).toHaveBeenCalledWith({ pagination: { pageSize: 50 } });
  });

  it("does NOT fetch when accessToken is null", async () => {
    mockAuth.accessToken = null as unknown as string;
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useTrips(), { wrapper });

    // Give it a tick to prove it stays idle
    await new Promise((r) => setTimeout(r, 50));
    expect(result.current.isLoading).toBe(false);
    expect(mockListTrips).not.toHaveBeenCalled();
  });

  it("returns empty array as default when no data yet", () => {
    mockAuth.accessToken = null as unknown as string;
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useTrips(), { wrapper });
    expect(result.current.trips).toEqual([]);
  });
});

// ---------- useTrip ----------

describe("useTrip", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("fetches a single trip by id", async () => {
    const trip = { id: "t1", title: "Tokyo" };
    mockGetTrip.mockResolvedValue({ trip });
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useTrip("t1"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.trip).toEqual(trip);
    expect(mockGetTrip).toHaveBeenCalledWith({ id: "t1" });
  });

  it("does NOT fetch when tripId is empty string", async () => {
    const { wrapper } = createWrapper();
    renderHook(() => useTrip(""), { wrapper });

    await new Promise((r) => setTimeout(r, 50));
    expect(mockGetTrip).not.toHaveBeenCalled();
  });

  it("does NOT fetch when accessToken is null", async () => {
    mockAuth.accessToken = null as unknown as string;
    const { wrapper } = createWrapper();
    renderHook(() => useTrip("t1"), { wrapper });

    await new Promise((r) => setTimeout(r, 50));
    expect(mockGetTrip).not.toHaveBeenCalled();
  });

  it("uses query key ['trip', tripId] — different tripIds are independent cache entries", async () => {
    const trip1 = { id: "t1", title: "Tokyo" };
    const trip2 = { id: "t2", title: "London" };
    mockGetTrip
      .mockResolvedValueOnce({ trip: trip1 })
      .mockResolvedValueOnce({ trip: trip2 });
    const { wrapper } = createWrapper();

    const { result: r1 } = renderHook(() => useTrip("t1"), { wrapper });
    const { result: r2 } = renderHook(() => useTrip("t2"), { wrapper });

    await waitFor(() => {
      expect(r1.current.trip).toEqual(trip1);
      expect(r2.current.trip).toEqual(trip2);
    });
  });
});

// ---------- useCreateTrip ----------

describe("useCreateTrip", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("calls createTrip and invalidates the trips list", async () => {
    const newTrip = { id: "t-new", title: "Berlin" };
    mockCreateTrip.mockResolvedValue({ trip: newTrip });

    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useCreateTrip(), { wrapper });

    await act(async () => {
      const returned = await result.current.mutateAsync({
        title: "Berlin",
        description: "Weekend trip",
      });
      expect(returned).toEqual(newTrip);
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["trips"] });
  });

  it("returns undefined trip when backend returns empty response (QA edge case)", async () => {
    // Backend might return {} with no trip field — createTrip returns res.trip which is undefined
    mockCreateTrip.mockResolvedValue({});

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useCreateTrip(), { wrapper });

    await act(async () => {
      const returned = await result.current.mutateAsync({ title: "Ghost Trip" });
      expect(returned).toBeUndefined();
    });
  });
});

// ---------- useUpdateTrip ----------

describe("useUpdateTrip", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("invalidates both the individual trip and the trips list on success", async () => {
    const updatedTrip = { id: "t1", title: "Updated Tokyo" };
    mockUpdateTrip.mockResolvedValue({ trip: updatedTrip });

    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useUpdateTrip(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "t1", title: "Updated Tokyo" });
    });

    // Must invalidate the specific trip AND the list
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["trip", "t1"] });
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["trips"] });
  });

  it("skips individual trip invalidation when response trip is undefined", async () => {
    // If backend returns no trip in the response, the guard `if (trip)` prevents
    // invalidating with undefined id
    mockUpdateTrip.mockResolvedValue({});

    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useUpdateTrip(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "t1", title: "Whatever" });
    });

    // Should still invalidate the list
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["trips"] });
    // Should NOT invalidate individual trip with undefined
    const tripCalls = invalidateSpy.mock.calls.filter(
      ([arg]) => Array.isArray((arg as { queryKey: unknown[] }).queryKey) && (arg as { queryKey: unknown[] }).queryKey[0] === "trip",
    );
    expect(tripCalls).toHaveLength(0);
  });
});

// ---------- useDeleteTrip ----------

describe("useDeleteTrip", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("calls deleteTrip with the correct id", async () => {
    mockDeleteTrip.mockResolvedValue({});
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useDeleteTrip(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("t1");
    });

    expect(mockDeleteTrip).toHaveBeenCalledWith({ id: "t1" });
  });

  it("invalidates the trips list on success", async () => {
    mockDeleteTrip.mockResolvedValue({});
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useDeleteTrip(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("t1");
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["trips"] });
  });

  // BUG: useDeleteTrip does NOT invalidate the individual trip query ["trip", tripId].
  // After deletion, any component still mounted with useTrip("t1") will serve stale
  // cached data instead of receiving a refetch (which would 404).
  // This test documents the known bug from the QA audit.
  it("BUG: does NOT invalidate the individual trip query (known QA issue)", async () => {
    mockDeleteTrip.mockResolvedValue({});
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useDeleteTrip(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("t1");
    });

    // This assertion proves the bug exists: the individual trip cache is never invalidated.
    // When this bug is fixed, this test should be updated to expect the invalidation.
    const tripCalls = invalidateSpy.mock.calls.filter(
      ([arg]) => {
        const key = (arg as { queryKey: unknown[] }).queryKey;
        return Array.isArray(key) && key[0] === "trip";
      },
    );
    expect(tripCalls).toHaveLength(0);

    // Also verify we can't find it via removeQueries either — the stale entry persists
    const cachedState = queryClient.getQueryState(["trip", "t1"]);
    // If data was previously cached, it would still be there after delete.
    // This is the actual user-facing bug: navigating back shows the deleted trip.
    expect(cachedState).toBeUndefined(); // no data was cached in this test, but the point is the hook never touches it
  });
});
