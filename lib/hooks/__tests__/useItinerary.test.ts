import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import type { Transport } from "@connectrpc/connect";

// useItinerary fetches an itinerary via ConnectRPC and computes
// `coveredDays` — the count of distinct days that actually have
// scheduled items. The trip detail screen uses this to display the
// "X of Y days planned" progress badge. A bug here either:
//   - over-counts (hides the "still need to plan" prompt mid-trip), or
//   - under-counts (wrongly nags the user about days they did plan)
//
// The interesting logic is the dayNumber-primary, date-fallback key
// used to dedupe — pin both paths.

const mockGetItinerary = vi.fn();

vi.mock("@connectrpc/connect", () => ({
  createClient: () => ({ getItinerary: mockGetItinerary }),
}));

vi.mock("@/lib/transport", () => ({
  useTransport: (): Transport => ({}) as Transport,
}));

const mockAuth = { accessToken: "test-token" as string | null };
vi.mock("@/lib/auth", () => ({
  useAuth: () => mockAuth,
}));

vi.mock("@gen/toqui/v1/trip_pb", () => ({
  TripService: {},
}));

import { useItinerary } from "../useItinerary";

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockAuth.accessToken = "test-token";
});

describe("useItinerary", () => {
  it("does not fetch when token is missing", () => {
    mockAuth.accessToken = null;
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useItinerary("trip-1"), { wrapper });

    expect(mockGetItinerary).not.toHaveBeenCalled();
    expect(result.current.itinerary).toBeUndefined();
    expect(result.current.coveredDays).toBe(0);
  });

  it("does not fetch when tripId is empty", () => {
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useItinerary(""), { wrapper });

    expect(mockGetItinerary).not.toHaveBeenCalled();
    expect(result.current.coveredDays).toBe(0);
  });

  it("fetches when authenticated with a tripId", async () => {
    mockGetItinerary.mockResolvedValueOnce({
      itinerary: { days: [] },
    });
    const { wrapper } = makeWrapper();
    renderHook(() => useItinerary("trip-1"), { wrapper });

    await waitFor(() => {
      expect(mockGetItinerary).toHaveBeenCalledWith({ tripId: "trip-1" });
    });
  });

  it("returns coveredDays = 0 when itinerary is undefined", () => {
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useItinerary("trip-1"), { wrapper });
    expect(result.current.coveredDays).toBe(0);
  });

  it("counts days with items (dayNumber-keyed)", async () => {
    // 3 days, only 2 have items — coveredDays = 2.
    mockGetItinerary.mockResolvedValueOnce({
      itinerary: {
        days: [
          { dayNumber: 1, date: "2026-05-01", items: [{}, {}] },
          { dayNumber: 2, date: "2026-05-02", items: [] },
          { dayNumber: 3, date: "2026-05-03", items: [{}] },
        ],
      },
    });
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useItinerary("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.itinerary).toBeDefined());
    expect(result.current.coveredDays).toBe(2);
  });

  it("dedupes when multiple ItineraryDay entries share the same dayNumber", async () => {
    // Defensive: backend should never emit duplicate dayNumber, but if it
    // did (split-day workflow, race in the move-item RPC), the count
    // must reflect distinct days, not entries. Pin the Set semantics.
    mockGetItinerary.mockResolvedValueOnce({
      itinerary: {
        days: [
          { dayNumber: 1, date: "2026-05-01", items: [{}] },
          { dayNumber: 1, date: "2026-05-01", items: [{}] }, // dup
          { dayNumber: 2, date: "2026-05-02", items: [{}] },
        ],
      },
    });
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useItinerary("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.itinerary).toBeDefined());
    expect(result.current.coveredDays).toBe(2);
  });

  it("falls back to date when dayNumber is 0/missing", async () => {
    // The hook's key fn does `d.dayNumber ? d.dayNumber : d.date`. A
    // dayNumber of 0 (unset/zero-value on the proto) falls through to
    // the date string. Pin this so a backend response that omits
    // dayNumber still gets correctly counted.
    mockGetItinerary.mockResolvedValueOnce({
      itinerary: {
        days: [
          { dayNumber: 0, date: "2026-05-01", items: [{}] },
          { dayNumber: 0, date: "2026-05-02", items: [{}] },
          { dayNumber: 0, date: "2026-05-03", items: [{}] },
        ],
      },
    });
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useItinerary("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.itinerary).toBeDefined());
    expect(result.current.coveredDays).toBe(3);
  });

  it("filters out falsy keys (no dayNumber AND no date)", async () => {
    // If both dayNumber and date are missing/empty, the entry is
    // dropped from the count. Pin so a malformed backend response
    // doesn't inflate coveredDays with a bogus entry.
    mockGetItinerary.mockResolvedValueOnce({
      itinerary: {
        days: [
          { dayNumber: 1, date: "2026-05-01", items: [{}] },
          { dayNumber: 0, date: "", items: [{}] }, // both falsy
          { dayNumber: 2, date: "2026-05-02", items: [{}] },
        ],
      },
    });
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useItinerary("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.itinerary).toBeDefined());
    expect(result.current.coveredDays).toBe(2);
  });
});
