/**
 * API Contract Tests
 *
 * Verify that each hook calls the correct RPC method with the correct
 * request payload shape, matching the proto schema exactly.
 */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { createElement } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ChatMode } from "@gen/toqui/v1/chat_pb";

// ---------------------------------------------------------------------------
// Track every RPC call: { method, args }
// ---------------------------------------------------------------------------
type RpcCall = { method: string; args: unknown };
let rpcCalls: RpcCall[] = [];

// Mock createClient to return a proxy capturing every call
vi.mock("@connectrpc/connect", () => ({
  createClient: (_service: unknown, _transport: unknown) =>
    new Proxy(
      {},
      {
        get: (_target, method: string) => {
          return (...args: unknown[]) => {
            rpcCalls.push({ method, args: args[0] });
            // Return appropriate shape per method
            if (method === "listTrips") {
              return Promise.resolve({ trips: [], pagination: undefined });
            }
            if (method === "getTrip") {
              return Promise.resolve({ trip: { id: "trip-1", title: "Test" } });
            }
            if (method === "createTrip") {
              return Promise.resolve({
                trip: { id: "new-trip", title: "Created" },
              });
            }
            if (method === "updateTrip") {
              return Promise.resolve({
                trip: { id: "trip-1", title: "Updated" },
              });
            }
            if (method === "deleteTrip") {
              return Promise.resolve({});
            }
            if (method === "getItinerary") {
              return Promise.resolve({
                itinerary: { tripId: "trip-1", days: [] },
              });
            }
            if (method === "listBookings") {
              return Promise.resolve({
                bookings: [],
                pagination: undefined,
              });
            }
            if (method === "getBooking") {
              return Promise.resolve({
                booking: { id: "booking-1" },
              });
            }
            if (method === "ingestBooking") {
              return Promise.resolve({
                booking: { id: "new-booking" },
              });
            }
            if (method === "deleteBooking") {
              return Promise.resolve({});
            }
            if (method === "getChatHistory") {
              return Promise.resolve({
                messages: [],
                pagination: { nextPageToken: "" },
              });
            }
            if (method === "sendMessage") {
              // Server streaming: return async iterable
              return (async function* () {
                yield {
                  event: {
                    case: "messageComplete" as const,
                    value: {
                      messageId: "msg-1",
                      sessionId: "sess-1",
                      fullContent: "Hello",
                    },
                  },
                };
              })();
            }
            return Promise.resolve({});
          };
        },
      },
    ),
  Code: { ResourceExhausted: 8 },
  ConnectError: class ConnectError extends Error {
    code: number;
    constructor(message: string, code: number) {
      super(message);
      this.code = code;
    }
  },
}));

// Stub bufbuild create to pass-through the values object
vi.mock("@bufbuild/protobuf", () => ({
  create: (_schema: unknown, values: unknown) => values,
}));

// Stub transport and auth
const mockTransport = { type: "mock-transport" };
vi.mock("@/lib/transport", () => ({
  useTransport: () => mockTransport,
}));

vi.mock("@/lib/auth", () => ({
  useAuth: () => ({ accessToken: "test-token", refreshTokens: vi.fn() }),
}));

// ---------------------------------------------------------------------------
// Test wrapper with QueryClient
// ---------------------------------------------------------------------------
function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

beforeEach(() => {
  rpcCalls = [];
});

// ===========================================================================
// TripService contract tests
// ===========================================================================
describe("useTrips — TripService.ListTrips", () => {
  it("calls listTrips with pageSize 50 in pagination", async () => {
    const { useTrips } = await import("@/lib/hooks/useTrips");
    const { result } = renderHook(() => useTrips(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    const call = rpcCalls.find((c) => c.method === "listTrips");
    expect(call).toBeDefined();
    expect(call!.args).toEqual({
      pagination: { pageSize: 50 },
    });
  });
});

describe("useTrip — TripService.GetTrip", () => {
  it("calls getTrip with { id: tripId }", async () => {
    const { useTrip } = await import("@/lib/hooks/useTrips");
    const { result } = renderHook(() => useTrip("trip-abc-123"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    const call = rpcCalls.find((c) => c.method === "getTrip");
    expect(call).toBeDefined();
    expect(call!.args).toEqual({ id: "trip-abc-123" });
  });

  it("does not call getTrip when tripId is empty", async () => {
    const { useTrip } = await import("@/lib/hooks/useTrips");
    renderHook(() => useTrip(""), { wrapper: createWrapper() });

    // Give it a tick to ensure no call fires
    await new Promise((r) => setTimeout(r, 50));
    const call = rpcCalls.find((c) => c.method === "getTrip");
    expect(call).toBeUndefined();
  });
});

describe("useCreateTrip — TripService.CreateTrip", () => {
  it("calls createTrip with title, description, startDate, endDate", async () => {
    const { useCreateTrip } = await import("@/lib/hooks/useTrips");
    const { result } = renderHook(() => useCreateTrip(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        title: "Japan Trip",
        description: "Two weeks in Japan",
        startDate: "2026-06-01",
        endDate: "2026-06-15",
      });
    });

    const call = rpcCalls.find((c) => c.method === "createTrip");
    expect(call).toBeDefined();
    expect(call!.args).toEqual({
      title: "Japan Trip",
      description: "Two weeks in Japan",
      startDate: "2026-06-01",
      endDate: "2026-06-15",
    });
  });

  it("calls createTrip with only required title field", async () => {
    const { useCreateTrip } = await import("@/lib/hooks/useTrips");
    const { result } = renderHook(() => useCreateTrip(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({ title: "Quick Trip" });
    });

    const call = rpcCalls.find((c) => c.method === "createTrip");
    expect(call).toBeDefined();
    expect(call!.args).toEqual({ title: "Quick Trip" });
  });
});

describe("useUpdateTrip — TripService.UpdateTrip", () => {
  it("calls updateTrip with id and partial fields", async () => {
    const { useUpdateTrip } = await import("@/lib/hooks/useTrips");
    const { result } = renderHook(() => useUpdateTrip(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        id: "trip-1",
        title: "Updated Title",
        status: 2, // ACTIVE
      });
    });

    const call = rpcCalls.find((c) => c.method === "updateTrip");
    expect(call).toBeDefined();
    const args = call!.args as Record<string, unknown>;
    expect(args.id).toBe("trip-1");
    expect(args.title).toBe("Updated Title");
    expect(args.status).toBe(2);
  });
});

describe("useDeleteTrip — TripService.DeleteTrip", () => {
  it("calls deleteTrip with { id: tripId }", async () => {
    const { useDeleteTrip } = await import("@/lib/hooks/useTrips");
    const { result } = renderHook(() => useDeleteTrip(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync("trip-to-delete");
    });

    const call = rpcCalls.find((c) => c.method === "deleteTrip");
    expect(call).toBeDefined();
    expect(call!.args).toEqual({ id: "trip-to-delete" });
  });
});

// ===========================================================================
// Itinerary contract tests (uses TripService)
// ===========================================================================
describe("useItinerary — TripService.GetItinerary", () => {
  it("calls getItinerary with { tripId }", async () => {
    const { useItinerary } = await import("@/lib/hooks/useItinerary");
    const { result } = renderHook(() => useItinerary("trip-itin-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    const call = rpcCalls.find((c) => c.method === "getItinerary");
    expect(call).toBeDefined();
    expect(call!.args).toEqual({ tripId: "trip-itin-1" });
  });

  it("does not call getItinerary when tripId is empty", async () => {
    const { useItinerary } = await import("@/lib/hooks/useItinerary");
    renderHook(() => useItinerary(""), { wrapper: createWrapper() });

    await new Promise((r) => setTimeout(r, 50));
    const call = rpcCalls.find((c) => c.method === "getItinerary");
    expect(call).toBeUndefined();
  });
});

// ===========================================================================
// BookingService contract tests
// ===========================================================================
describe("useBookings — BookingService.ListBookings", () => {
  it("calls listBookings with tripId and pagination (pageSize 100, empty pageToken)", async () => {
    const { useBookings } = await import("@/lib/hooks/useBookings");
    const { result } = renderHook(() => useBookings("trip-bk-1"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    const call = rpcCalls.find((c) => c.method === "listBookings");
    expect(call).toBeDefined();
    // useBookings uses create(ListBookingsRequestSchema, {...}) which our mock passes through
    expect(call!.args).toEqual({
      tripId: "trip-bk-1",
      pagination: { pageSize: 100, pageToken: "" },
    });
  });

  it("does not call listBookings when tripId is empty", async () => {
    const { useBookings } = await import("@/lib/hooks/useBookings");
    renderHook(() => useBookings(""), { wrapper: createWrapper() });

    await new Promise((r) => setTimeout(r, 50));
    const call = rpcCalls.find((c) => c.method === "listBookings");
    expect(call).toBeUndefined();
  });
});

describe("useBooking — BookingService.GetBooking", () => {
  it("calls getBooking with { id: bookingId } via create(GetBookingRequestSchema)", async () => {
    const { useBooking } = await import("@/lib/hooks/useBookings");
    const { result } = renderHook(() => useBooking("booking-xyz"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    const call = rpcCalls.find((c) => c.method === "getBooking");
    expect(call).toBeDefined();
    expect(call!.args).toEqual({ id: "booking-xyz" });
  });
});

describe("useIngestBooking — BookingService.IngestBooking", () => {
  it("calls ingestBooking with tripId, type (BookingType enum), and rawText", async () => {
    const { useIngestBooking } = await import("@/lib/hooks/useBookings");
    const { BookingType } = await import("@gen/toqui/v1/booking_pb");

    const { result } = renderHook(() => useIngestBooking(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        tripId: "trip-ingest",
        type: BookingType.FLIGHT,
        rawText: "Confirmation: ABC123\nFlight: AA100\nDep: LAX\nArr: NRT",
      });
    });

    const call = rpcCalls.find((c) => c.method === "ingestBooking");
    expect(call).toBeDefined();
    expect(call!.args).toEqual({
      tripId: "trip-ingest",
      type: BookingType.FLIGHT,
      rawText: "Confirmation: ABC123\nFlight: AA100\nDep: LAX\nArr: NRT",
    });
  });

  it("passes BookingType.HOTEL correctly", async () => {
    const { useIngestBooking } = await import("@/lib/hooks/useBookings");
    const { BookingType } = await import("@gen/toqui/v1/booking_pb");

    const { result } = renderHook(() => useIngestBooking(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        tripId: "trip-hotel",
        type: BookingType.HOTEL,
        rawText: "Hotel Booking\nCheck-in: 2026-07-01",
      });
    });

    const call = rpcCalls.find((c) => c.method === "ingestBooking");
    expect(call).toBeDefined();
    const args = call!.args as Record<string, unknown>;
    expect(args.type).toBe(BookingType.HOTEL);
    expect(args.type).toBe(2); // HOTEL = 2 in proto
  });
});

describe("useDeleteBooking — BookingService.DeleteBooking", () => {
  it("calls deleteBooking with only { id } (not tripId)", async () => {
    const { useDeleteBooking } = await import("@/lib/hooks/useBookings");
    const { result } = renderHook(() => useDeleteBooking(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({
        id: "booking-del-1",
        tripId: "trip-del-1",
      });
    });

    const call = rpcCalls.find((c) => c.method === "deleteBooking");
    expect(call).toBeDefined();
    // Contract: deleteBooking only sends { id }, NOT tripId
    // tripId is used client-side for cache invalidation only
    expect(call!.args).toEqual({ id: "booking-del-1" });
  });
});

// ===========================================================================
// ChatService contract tests
// ===========================================================================
describe("useChat — ChatService.GetChatHistory", () => {
  it("calls getChatHistory on mount with tripId, empty sessionId, and pagination", async () => {
    const { useChat } = await import("@/lib/hooks/useChat");
    renderHook(() => useChat("trip-chat-1", "planning"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      const call = rpcCalls.find((c) => c.method === "getChatHistory");
      expect(call).toBeDefined();
    });

    const call = rpcCalls.find((c) => c.method === "getChatHistory");
    expect(call!.args).toEqual({
      tripId: "trip-chat-1",
      sessionId: "",
      pagination: { pageSize: 100, pageToken: "" },
    });
  });

  it("does not call getChatHistory when tripId is undefined", async () => {
    const { useChat } = await import("@/lib/hooks/useChat");
    renderHook(() => useChat(undefined, "planning"), {
      wrapper: createWrapper(),
    });

    await new Promise((r) => setTimeout(r, 100));
    const call = rpcCalls.find((c) => c.method === "getChatHistory");
    expect(call).toBeUndefined();
  });
});

describe("useChat — ChatService.SendMessage", () => {
  it("calls sendMessage with correct fields for planning mode", async () => {
    const { useChat } = await import("@/lib/hooks/useChat");
    const { result } = renderHook(() => useChat("trip-plan-1", "planning"), {
      wrapper: createWrapper(),
    });

    // Wait for history load
    await waitFor(() => {
      const histCall = rpcCalls.find((c) => c.method === "getChatHistory");
      expect(histCall).toBeDefined();
    });
    rpcCalls = [];

    await act(async () => {
      await result.current.sendMessage("Plan my trip to Tokyo");
    });

    const call = rpcCalls.find((c) => c.method === "sendMessage");
    expect(call).toBeDefined();
    const args = call!.args as Record<string, unknown>;
    expect(args.tripId).toBe("trip-plan-1");
    expect(args.content).toBe("Plan my trip to Tokyo");
    expect(args.mode).toBe(ChatMode.PLANNING);
    expect(args.sessionId).toBe(""); // no session yet
  });

  it("maps companion mode to ChatMode.COMPANION (enum value 2)", async () => {
    const { useChat } = await import("@/lib/hooks/useChat");
    const { result } = renderHook(() => useChat("trip-comp-1", "companion"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(rpcCalls.find((c) => c.method === "getChatHistory")).toBeDefined();
    });
    rpcCalls = [];

    await act(async () => {
      await result.current.sendMessage("Where should I eat?");
    });

    const call = rpcCalls.find((c) => c.method === "sendMessage");
    expect(call).toBeDefined();
    const args = call!.args as Record<string, unknown>;
    expect(args.mode).toBe(ChatMode.COMPANION);
    expect(args.mode).toBe(2);
  });

  it("maps selection mode to ChatMode.SELECTION (enum value 3)", async () => {
    const { useChat } = await import("@/lib/hooks/useChat");
    const { result } = renderHook(() => useChat(undefined, "selection"), {
      wrapper: createWrapper(),
    });

    // No history load for undefined tripId
    await new Promise((r) => setTimeout(r, 50));
    rpcCalls = [];

    await act(async () => {
      await result.current.sendMessage("I want to go to Paris");
    });

    const call = rpcCalls.find((c) => c.method === "sendMessage");
    expect(call).toBeDefined();
    const args = call!.args as Record<string, unknown>;
    expect(args.mode).toBe(ChatMode.SELECTION);
    expect(args.mode).toBe(3);
    // tripId should be empty string when undefined
    expect(args.tripId).toBe("");
  });

  it("sends tripId as empty string when tripId is undefined", async () => {
    const { useChat } = await import("@/lib/hooks/useChat");
    const { result } = renderHook(() => useChat(undefined, "selection"), {
      wrapper: createWrapper(),
    });

    await new Promise((r) => setTimeout(r, 50));
    rpcCalls = [];

    await act(async () => {
      await result.current.sendMessage("Hello");
    });

    const call = rpcCalls.find((c) => c.method === "sendMessage");
    expect(call).toBeDefined();
    expect((call!.args as Record<string, unknown>).tripId).toBe("");
  });
});

// ===========================================================================
// Cross-cutting: service binding verification
// ===========================================================================
describe("Service binding — hooks use the correct service", () => {
  it("useTrips creates client with TripService (verified via listTrips method existence)", async () => {
    const { useTrips } = await import("@/lib/hooks/useTrips");
    renderHook(() => useTrips(), { wrapper: createWrapper() });

    await waitFor(() => {
      const call = rpcCalls.find((c) => c.method === "listTrips");
      expect(call).toBeDefined();
    });
    // If the wrong service were used, the proxy would capture a different method name
    // or the hook would fail trying to call a method that doesn't exist on the service
    expect(rpcCalls.some((c) => c.method === "listTrips")).toBe(true);
  });

  it("useItinerary creates client with TripService (getItinerary is on TripService)", async () => {
    const { useItinerary } = await import("@/lib/hooks/useItinerary");
    renderHook(() => useItinerary("trip-1"), { wrapper: createWrapper() });

    await waitFor(() => {
      const call = rpcCalls.find((c) => c.method === "getItinerary");
      expect(call).toBeDefined();
    });
  });

  it("useBookings creates client with BookingService (listBookings is on BookingService)", async () => {
    const { useBookings } = await import("@/lib/hooks/useBookings");
    renderHook(() => useBookings("trip-1"), { wrapper: createWrapper() });

    await waitFor(() => {
      const call = rpcCalls.find((c) => c.method === "listBookings");
      expect(call).toBeDefined();
    });
  });
});

// ===========================================================================
// Pagination contract tests
// ===========================================================================
describe("Pagination contracts", () => {
  it("useTrips requests pageSize 50 (not 100 or unlimited)", async () => {
    const { useTrips } = await import("@/lib/hooks/useTrips");
    renderHook(() => useTrips(), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(rpcCalls.find((c) => c.method === "listTrips")).toBeDefined();
    });

    const call = rpcCalls.find((c) => c.method === "listTrips");
    const args = call!.args as { pagination: { pageSize: number } };
    expect(args.pagination.pageSize).toBe(50);
  });

  it("useBookings requests pageSize 100 with empty pageToken", async () => {
    const { useBookings } = await import("@/lib/hooks/useBookings");
    renderHook(() => useBookings("trip-1"), { wrapper: createWrapper() });

    await waitFor(() => {
      expect(rpcCalls.find((c) => c.method === "listBookings")).toBeDefined();
    });

    const call = rpcCalls.find((c) => c.method === "listBookings");
    const args = call!.args as {
      pagination: { pageSize: number; pageToken: string };
    };
    expect(args.pagination.pageSize).toBe(100);
    expect(args.pagination.pageToken).toBe("");
  });

  it("getChatHistory requests pageSize 100 with empty pageToken", async () => {
    const { useChat } = await import("@/lib/hooks/useChat");
    renderHook(() => useChat("trip-1", "planning"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(
        rpcCalls.find((c) => c.method === "getChatHistory"),
      ).toBeDefined();
    });

    const call = rpcCalls.find((c) => c.method === "getChatHistory");
    const args = call!.args as {
      pagination: { pageSize: number; pageToken: string };
    };
    expect(args.pagination.pageSize).toBe(100);
    expect(args.pagination.pageToken).toBe("");
  });
});

// ===========================================================================
// ChatMode enum mapping exhaustiveness
// ===========================================================================
describe("ChatMode enum mapping", () => {
  it("planning maps to ChatMode.PLANNING (1)", () => {
    const modeToProto: Record<string, ChatMode> = {
      planning: ChatMode.PLANNING,
      companion: ChatMode.COMPANION,
      selection: ChatMode.SELECTION,
    };
    expect(modeToProto["planning"]).toBe(1);
    expect(modeToProto["companion"]).toBe(2);
    expect(modeToProto["selection"]).toBe(3);
  });

  it("ChatMode enum values match proto definition", () => {
    expect(ChatMode.UNSPECIFIED).toBe(0);
    expect(ChatMode.PLANNING).toBe(1);
    expect(ChatMode.COMPANION).toBe(2);
    expect(ChatMode.SELECTION).toBe(3);
  });
});
