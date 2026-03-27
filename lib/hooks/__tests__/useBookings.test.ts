import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import type { Transport } from "@connectrpc/connect";

// ---------- mocks ----------

const mockListBookings = vi.fn();
const mockGetBooking = vi.fn();
const mockIngestBooking = vi.fn();
const mockDeleteBooking = vi.fn();

vi.mock("@connectrpc/connect", () => ({
  createClient: () => ({
    listBookings: mockListBookings,
    getBooking: mockGetBooking,
    ingestBooking: mockIngestBooking,
    deleteBooking: mockDeleteBooking,
  }),
}));

vi.mock("@/lib/transport", () => ({
  useTransport: (): Transport => ({} as Transport),
}));

const mockAuth = { accessToken: "test-token" };
vi.mock("@/lib/auth", () => ({
  useAuth: () => mockAuth,
}));

// Stub protobuf create() — the hooks use create(Schema, data) which we can pass through
vi.mock("@bufbuild/protobuf", () => ({
  create: (_schema: unknown, data: unknown) => data,
}));

vi.mock("@gen/toqui/v1/booking_pb", () => ({
  BookingService: {},
  IngestBookingRequestSchema: {},
  ListBookingsRequestSchema: {},
  GetBookingRequestSchema: {},
  DeleteBookingRequestSchema: {},
}));

import { useBookings, useBooking, useIngestBooking, useDeleteBooking } from "../useBookings";

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

// ---------- useBookings ----------

describe("useBookings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("fetches bookings for a trip when authenticated", async () => {
    const bookings = [{ id: "b1", tripId: "t1", title: "Hilton" }];
    mockListBookings.mockResolvedValue({ bookings });
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useBookings("t1"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.bookings).toEqual(bookings);
  });

  it("does NOT fetch when accessToken is null", async () => {
    mockAuth.accessToken = null as unknown as string;
    const { wrapper } = createWrapper();

    renderHook(() => useBookings("t1"), { wrapper });

    await new Promise((r) => setTimeout(r, 50));
    expect(mockListBookings).not.toHaveBeenCalled();
  });

  it("does NOT fetch when tripId is empty string", async () => {
    const { wrapper } = createWrapper();

    renderHook(() => useBookings(""), { wrapper });

    await new Promise((r) => setTimeout(r, 50));
    expect(mockListBookings).not.toHaveBeenCalled();
  });

  it("uses query key ['bookings', tripId] — different trips have independent caches", async () => {
    mockListBookings
      .mockResolvedValueOnce({ bookings: [{ id: "b1" }] })
      .mockResolvedValueOnce({ bookings: [{ id: "b2" }] });
    const { wrapper } = createWrapper();

    const { result: r1 } = renderHook(() => useBookings("trip-a"), { wrapper });
    const { result: r2 } = renderHook(() => useBookings("trip-b"), { wrapper });

    await waitFor(() => {
      expect(r1.current.bookings).toEqual([{ id: "b1" }]);
      expect(r2.current.bookings).toEqual([{ id: "b2" }]);
    });
  });

  it("defaults to empty array when query has not loaded", () => {
    mockAuth.accessToken = null as unknown as string;
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useBookings("t1"), { wrapper });
    expect(result.current.bookings).toEqual([]);
  });
});

// ---------- useBooking ----------

describe("useBooking", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("fetches a single booking by id", async () => {
    const booking = { id: "b1", title: "Flight to Paris" };
    mockGetBooking.mockResolvedValue({ booking });
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useBooking("b1"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.booking).toEqual(booking);
  });

  it("does NOT fetch when bookingId is empty", async () => {
    const { wrapper } = createWrapper();
    renderHook(() => useBooking(""), { wrapper });

    await new Promise((r) => setTimeout(r, 50));
    expect(mockGetBooking).not.toHaveBeenCalled();
  });

  it("does NOT fetch when accessToken is null", async () => {
    mockAuth.accessToken = null as unknown as string;
    const { wrapper } = createWrapper();
    renderHook(() => useBooking("b1"), { wrapper });

    await new Promise((r) => setTimeout(r, 50));
    expect(mockGetBooking).not.toHaveBeenCalled();
  });
});

// ---------- useIngestBooking ----------

describe("useIngestBooking", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("calls ingestBooking and invalidates bookings for the correct trip", async () => {
    const newBooking = { id: "b-new", tripId: "t1", title: "New Hotel" };
    mockIngestBooking.mockResolvedValue({ booking: newBooking });

    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useIngestBooking(), { wrapper });

    await act(async () => {
      const returned = await result.current.mutateAsync({
        tripId: "t1",
        type: 2, // HOTEL
        rawText: "Hilton confirmation...",
      });
      expect(returned).toEqual(newBooking);
    });

    // Must invalidate bookings for the specific trip from the variables, not the response
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["bookings", "t1"] });
  });

  it("invalidates using variables.tripId, not the returned booking (correct behavior)", async () => {
    // Even if the booking response had a different tripId (edge case),
    // the invalidation uses variables.tripId which is the right thing to do
    const booking = { id: "b-new", tripId: "t-different", title: "Rerouted" };
    mockIngestBooking.mockResolvedValue({ booking });

    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useIngestBooking(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        tripId: "t1",
        type: 1,
        rawText: "some text",
      });
    });

    // Invalidates the trip we sent the request for, not whatever the server returned
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["bookings", "t1"] });
    // Does NOT invalidate for the response's tripId
    const wrongCalls = invalidateSpy.mock.calls.filter(
      ([arg]) => {
        const key = (arg as { queryKey: unknown[] }).queryKey;
        return Array.isArray(key) && key[1] === "t-different";
      },
    );
    expect(wrongCalls).toHaveLength(0);
  });

  it("does NOT invalidate the individual booking query", async () => {
    mockIngestBooking.mockResolvedValue({ booking: { id: "b-new" } });

    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useIngestBooking(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ tripId: "t1", type: 1, rawText: "text" });
    });

    // Should only invalidate the list, not a specific booking entry
    const bookingCalls = invalidateSpy.mock.calls.filter(
      ([arg]) => {
        const key = (arg as { queryKey: unknown[] }).queryKey;
        return Array.isArray(key) && key[0] === "booking";
      },
    );
    expect(bookingCalls).toHaveLength(0);
  });
});

// ---------- useDeleteBooking ----------

describe("useDeleteBooking", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("calls deleteBooking with the correct id", async () => {
    mockDeleteBooking.mockResolvedValue({});
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useDeleteBooking(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "b1", tripId: "t1" });
    });

    // The hook passes only { id } to the RPC, not the full params
    expect(mockDeleteBooking).toHaveBeenCalledWith({ id: "b1" });
  });

  it("invalidates the bookings list for the correct trip on success", async () => {
    mockDeleteBooking.mockResolvedValue({});
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useDeleteBooking(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "b1", tripId: "t1" });
    });

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ["bookings", "t1"] });
  });

  it("does NOT invalidate bookings for a different trip", async () => {
    mockDeleteBooking.mockResolvedValue({});
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useDeleteBooking(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "b1", tripId: "t1" });
    });

    // Only invalidates bookings for t1, not any other trip
    const allInvalidations = invalidateSpy.mock.calls.map(
      ([arg]) => (arg as { queryKey: unknown[] }).queryKey,
    );
    expect(allInvalidations).toEqual([["bookings", "t1"]]);
  });

  it("passes tripId through the mutation return so onSuccess can access it via variables", async () => {
    // The mutationFn returns params (including tripId) so onSuccess can use variables.tripId
    // If someone refactors mutationFn to not return params, onSuccess still works because
    // it uses the `variables` argument, not `_result`. This test verifies the flow.
    mockDeleteBooking.mockResolvedValue({});
    const { wrapper } = createWrapper();

    const { result } = renderHook(() => useDeleteBooking(), { wrapper });

    let mutationResult: unknown;
    await act(async () => {
      mutationResult = await result.current.mutateAsync({ id: "b1", tripId: "t1" });
    });

    // mutationFn returns params
    expect(mutationResult).toEqual({ id: "b1", tripId: "t1" });
  });

  it("does NOT invalidate the individual booking query (no stale single-booking cache removal)", async () => {
    // Similar pattern to the useDeleteTrip bug — after deleting a booking,
    // any component with useBooking("b1") still has stale cache.
    // Unlike useDeleteTrip, this may be intentional since the list invalidation
    // is trip-scoped. But it's worth documenting.
    mockDeleteBooking.mockResolvedValue({});
    const { wrapper, queryClient } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useDeleteBooking(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "b1", tripId: "t1" });
    });

    const individualBookingCalls = invalidateSpy.mock.calls.filter(
      ([arg]) => {
        const key = (arg as { queryKey: unknown[] }).queryKey;
        return Array.isArray(key) && key[0] === "booking";
      },
    );
    expect(individualBookingCalls).toHaveLength(0);
  });
});
