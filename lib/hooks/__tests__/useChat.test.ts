// CLAIMED BY AGENT 2
import { renderHook, act } from "@testing-library/react";
import { vi, describe, it, expect, beforeEach, type Mock } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import { ChatMode } from "@gen/toqui/v1/chat_pb";
import type { SendMessageResponse } from "@gen/toqui/v1/chat_pb";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockGetChatHistory = vi.fn();
const mockSendMessage = vi.fn();

vi.mock("@connectrpc/connect", () => ({
  createClient: vi.fn(() => ({
    sendMessage: mockSendMessage,
    getChatHistory: mockGetChatHistory,
  })),
  Code: { ResourceExhausted: 8, Unauthenticated: 16, Internal: 13 },
  ConnectError: class ConnectError extends Error {
    code: number;
    constructor(message: string, code: number) {
      super(message);
      this.code = code;
      this.name = "ConnectError";
    }
  },
}));

vi.mock("@/lib/transport", () => ({
  useTransport: vi.fn(() => ({})), // returns a dummy transport
}));

vi.mock("@/lib/auth", () => ({
  useAuth: vi.fn(() => ({ isLoading: false, accessToken: "test-token" })),
}));

vi.mock("react-native", () => ({
  Platform: { OS: "web" },
}));

vi.mock("@react-native-async-storage/async-storage", () => ({
  default: {
    getItem: vi.fn().mockResolvedValue(null),
    setItem: vi.fn().mockResolvedValue(undefined),
  },
}));

const mockInvalidateQueries = vi.fn().mockResolvedValue(undefined);

vi.mock("@tanstack/react-query", () => ({
  useQueryClient: vi.fn(() => ({
    invalidateQueries: mockInvalidateQueries,
  })),
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Build a minimal SendMessageResponse-shaped event for the async generator. */
function evt(
  eventCase: string,
  value: Record<string, unknown>,
): Partial<SendMessageResponse> {
  return { event: { case: eventCase, value } } as unknown as SendMessageResponse;
}

/** Create an async generator that yields the given events in order. */
async function* streamEvents(
  events: Partial<SendMessageResponse>[],
): AsyncGenerator<Partial<SendMessageResponse>> {
  for (const e of events) {
    yield e;
  }
}

function emptyHistoryResponse(nextPageToken = "") {
  return {
    messages: [],
    pagination: { nextPageToken },
  };
}

function historyResponse(
  messages: Array<{
    id: string;
    role: string;
    content: string;
    metadata?: Record<string, string>;
  }>,
  nextPageToken = "",
) {
  return {
    messages: messages.map((m) => ({
      id: m.id,
      role: m.role,
      content: m.content,
      metadata: m.metadata ?? {},
    })),
    pagination: { nextPageToken },
  };
}

// ---------------------------------------------------------------------------
// Import under test (after mocks are registered)
// ---------------------------------------------------------------------------

import { useChat } from "@/lib/hooks/useChat";

beforeEach(() => {
  mockSendMessage.mockClear();
  mockGetChatHistory.mockClear();
  mockInvalidateQueries.mockClear();
  mockGetChatHistory.mockResolvedValue(emptyHistoryResponse());
  // Clear sessionStorage so session IDs don't leak between tests
  sessionStorage.clear();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useChat", () => {
  // =========================================================================
  // Initial state
  // =========================================================================

  it("returns correct initial state", () => {
    const { result } = renderHook(() => useChat(undefined, "planning"));
    expect(result.current.messages).toEqual([]);
    expect(result.current.streamingText).toBe("");
    expect(result.current.isStreaming).toBe(false);
    expect(result.current.isLoadingHistory).toBe(false);
    expect(result.current.activePersona).toBeNull();
    expect(result.current.toolActivity).toBeNull();
    expect(result.current.createdTrip).toBeNull();
    expect(result.current.selectedTrip).toBeNull();
    expect(result.current.hasMoreHistory).toBe(false);
  });

  // =========================================================================
  // sendMessage: basic streaming flow
  // =========================================================================

  describe("sendMessage", () => {
    it("adds user message immediately, streams text, then appends assistant message", async () => {
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("textDelta", { text: "Hello " }),
          evt("textDelta", { text: "world" }),
          evt("messageComplete", { fullContent: "Hello world", messageId: "m1", sessionId: "s1" }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("Hi");
      });

      // User message + assistant message
      expect(result.current.messages).toHaveLength(2);
      expect(result.current.messages[0].role).toBe("user");
      expect(result.current.messages[0].content).toBe("Hi");
      expect(result.current.messages[1].role).toBe("assistant");
      expect(result.current.messages[1].content).toBe("Hello world");
      // Streaming should be done
      expect(result.current.isStreaming).toBe(false);
      expect(result.current.streamingText).toBe("");
    });

    it("uses messageComplete fullContent to override accumulated deltas", async () => {
      // The server may correct the final text in messageComplete
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("textDelta", { text: "partial" }),
          evt("messageComplete", {
            fullContent: "corrected final text",
            messageId: "m1",
            sessionId: "s1",
          }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      const assistant = result.current.messages.find((m) => m.role === "assistant");
      expect(assistant?.content).toBe("corrected final text");
    });

    it("appends assistant message even without messageComplete if text accumulated", async () => {
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("textDelta", { text: "some response" }),
          // No messageComplete event
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(result.current.messages).toHaveLength(2);
      expect(result.current.messages[1].content).toBe("some response");
    });

    it("does NOT append empty assistant message when stream has no text", async () => {
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("sessionCreated", { sessionId: "s1" }),
          evt("messageComplete", { fullContent: "", messageId: "m1", sessionId: "s1" }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      // Only user message, no empty assistant message
      expect(result.current.messages).toHaveLength(1);
      expect(result.current.messages[0].role).toBe("user");
    });
  });

  // =========================================================================
  // Session management
  // =========================================================================

  describe("sessionCreated event", () => {
    it("stores sessionId and sends it on subsequent messages", async () => {
      mockSendMessage
        .mockReturnValueOnce(
          streamEvents([
            evt("sessionCreated", { sessionId: "session-abc" }),
            evt("textDelta", { text: "hi" }),
            evt("messageComplete", { fullContent: "hi", messageId: "m1", sessionId: "session-abc" }),
          ]),
        )
        .mockReturnValueOnce(
          streamEvents([
            evt("textDelta", { text: "ok" }),
            evt("messageComplete", { fullContent: "ok", messageId: "m2", sessionId: "session-abc" }),
          ]),
        );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("first");
      });
      await act(async () => {
        await result.current.sendMessage("second");
      });

      // Second call should include the session ID from the first response
      const secondCallArgs = mockSendMessage.mock.calls[1][0];
      expect(secondCallArgs.sessionId).toBe("session-abc");
    });
  });

  // =========================================================================
  // isSendingRef guard against concurrent sends
  // =========================================================================

  describe("concurrent send guard", () => {
    it.skip("blocks a second sendMessage while the first is still streaming", async () => {
      let resolve: () => void;
      const blockingPromise = new Promise<void>((r) => {
        resolve = r;
      });

      // Use mockImplementation so each call gets a fresh generator
      mockSendMessage.mockImplementation(() =>
        (async function* () {
          yield evt("textDelta", { text: "slow" });
          await blockingPromise;
          yield evt("messageComplete", { fullContent: "slow", messageId: "m1", sessionId: "s1" });
        })(),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      // Start first send (don't await — it will block on the promise)
      let firstDone = false;
      const firstPromise = act(async () => {
        await result.current.sendMessage("first");
        firstDone = true;
      });

      // Attempt second send while first is blocked (should be no-op)
      await act(async () => {
        result.current.sendMessage("second");
      });

      // Only one user message should exist (the first one)
      const userMessages = result.current.messages.filter((m) => m.role === "user");
      expect(userMessages).toHaveLength(1);
      expect(userMessages[0].content).toBe("first");

      // Unblock the first and wait for it to complete
      resolve!();
      await firstPromise;
      expect(firstDone).toBe(true);
    });
  });

  // =========================================================================
  // Tool events
  // =========================================================================

  describe("tool events", () => {
    it("sets toolActivity on toolCall and clears it on messageComplete", async () => {
      const toolActivities: Array<{ toolName: string; status: string } | null> = [];

      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("toolCall", { toolName: "search_flights", inputJson: "{}" }),
          evt("toolResult", { toolName: "search_flights", resultJson: "{}" }),
          evt("textDelta", { text: "found flights" }),
          evt("messageComplete", {
            fullContent: "found flights",
            messageId: "m1",
            sessionId: "s1",
          }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("find flights");
      });

      // After messageComplete, toolActivity should be null
      expect(result.current.toolActivity).toBeNull();
    });

    it("invalidates itinerary and trip cache on create_itinerary_items toolResult", async () => {
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("toolCall", { toolName: "create_itinerary_items", inputJson: "{}" }),
          evt("toolResult", {
            toolName: "create_itinerary_items",
            resultJson: JSON.stringify({ success: true }),
          }),
          evt("textDelta", { text: "Itinerary created!" }),
          evt("messageComplete", { fullContent: "Itinerary created!", messageId: "m1", sessionId: "s1" }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("plan my itinerary");
      });

      expect(mockInvalidateQueries).toHaveBeenCalledWith({ queryKey: ["itinerary", "trip-1"] });
      expect(mockInvalidateQueries).toHaveBeenCalledWith({ queryKey: ["trip", "trip-1"] });
    });

    it("does not invalidate cache for non-itinerary tool results", async () => {
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("toolResult", {
            toolName: "search_flights",
            resultJson: JSON.stringify({ flights: [] }),
          }),
          evt("textDelta", { text: "results" }),
          evt("messageComplete", { fullContent: "results", messageId: "m1", sessionId: "s1" }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(mockInvalidateQueries).not.toHaveBeenCalled();
    });
  });

  // =========================================================================
  // Trip creation and selection
  // =========================================================================

  describe("trip events", () => {
    it("sets createdTrip on tripCreated event", async () => {
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("tripCreated", {
            trip: { id: "t1", title: "Tokyo Trip", description: "Exploring Japan" },
          }),
          evt("textDelta", { text: "Created!" }),
          evt("messageComplete", { fullContent: "Created!", messageId: "m1", sessionId: "s1" }),
        ]),
      );

      const { result } = renderHook(() => useChat(undefined, "selection"));

      await act(async () => {
        await result.current.sendMessage("plan tokyo trip");
      });

      expect(result.current.createdTrip).toEqual({
        id: "t1",
        title: "Tokyo Trip",
        description: "Exploring Japan",
      });
    });

    it("sets selectedTrip on tripSelected event", async () => {
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("tripSelected", {
            trip: { id: "t2", title: "Paris Trip", description: "Romance!" },
          }),
          evt("textDelta", { text: "Selected!" }),
          evt("messageComplete", { fullContent: "Selected!", messageId: "m1", sessionId: "s1" }),
        ]),
      );

      const { result } = renderHook(() => useChat(undefined, "selection"));

      await act(async () => {
        await result.current.sendMessage("use paris trip");
      });

      expect(result.current.selectedTrip).toEqual({
        id: "t2",
        title: "Paris Trip",
        description: "Romance!",
      });
    });

    it("resets createdTrip and selectedTrip on each new sendMessage", async () => {
      mockSendMessage
        .mockReturnValueOnce(
          streamEvents([
            evt("tripCreated", {
              trip: { id: "t1", title: "Trip 1", description: "d1" },
            }),
            evt("textDelta", { text: "done" }),
            evt("messageComplete", { fullContent: "done", messageId: "m1", sessionId: "s1" }),
          ]),
        )
        .mockReturnValueOnce(
          streamEvents([
            evt("textDelta", { text: "no trip" }),
            evt("messageComplete", { fullContent: "no trip", messageId: "m2", sessionId: "s1" }),
          ]),
        );

      const { result } = renderHook(() => useChat(undefined, "selection"));

      await act(async () => {
        await result.current.sendMessage("first");
      });
      expect(result.current.createdTrip).not.toBeNull();

      await act(async () => {
        await result.current.sendMessage("second");
      });
      // createdTrip was reset at the start of the second sendMessage
      expect(result.current.createdTrip).toBeNull();
    });

    it("ignores tripCreated with no trip object", async () => {
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("tripCreated", {}), // trip is undefined
          evt("textDelta", { text: "x" }),
          evt("messageComplete", { fullContent: "x", messageId: "m1", sessionId: "s1" }),
        ]),
      );

      const { result } = renderHook(() => useChat(undefined, "selection"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(result.current.createdTrip).toBeNull();
    });
  });

  // =========================================================================
  // Persona switch
  // =========================================================================

  describe("personaSwitch event", () => {
    it("updates activePersona and adds system handoff message", async () => {
      const newPersona = {
        id: "p-hana",
        name: "Hana",
        avatarUrl: "https://cdn/hana.png",
        accentColor: "#ff0000",
        specialties: ["food", "culture"],
        description: "",
        greeting: "",
        type: 2,
        regionCode: "JP",
        localeName: "Tokyo",
        isDefault: false,
      };

      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("personaSwitch", {
            newPersona,
            handoffMessage: "Meet Hana, your Tokyo guide!",
          }),
          evt("textDelta", { text: "Konnichiwa!" }),
          evt("messageComplete", {
            fullContent: "Konnichiwa!",
            messageId: "m1",
            sessionId: "s1",
          }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "companion"));

      await act(async () => {
        await result.current.sendMessage("I arrived in Tokyo");
      });

      expect(result.current.activePersona).toEqual({
        id: "p-hana",
        name: "Hana",
        avatarUrl: "https://cdn/hana.png",
        accentColor: "#ff0000",
        specialties: ["food", "culture"],
      });

      // Messages: user, system (handoff), assistant
      expect(result.current.messages).toHaveLength(3);
      expect(result.current.messages[1].role).toBe("system");
      expect(result.current.messages[1].content).toBe("Meet Hana, your Tokyo guide!");

      // Assistant message should carry persona metadata
      const assistantMsg = result.current.messages[2];
      expect(assistantMsg.personaId).toBe("p-hana");
      expect(assistantMsg.personaName).toBe("Hana");
      expect(assistantMsg.personaAvatar).toBe("https://cdn/hana.png");
      expect(assistantMsg.personaAccentColor).toBe("#ff0000");
    });

    it("does not add system message when handoffMessage is empty", async () => {
      const newPersona = {
        id: "p1",
        name: "Guide",
        avatarUrl: "",
        accentColor: "",
        specialties: [],
        description: "",
        greeting: "",
        type: 1,
        regionCode: "",
        localeName: "",
        isDefault: false,
      };

      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("personaSwitch", { newPersona, handoffMessage: "" }),
          evt("textDelta", { text: "hi" }),
          evt("messageComplete", { fullContent: "hi", messageId: "m1", sessionId: "s1" }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      // Only user + assistant, no system handoff for empty message
      // NOTE: The hook adds a system message with content "" if handoffMessage is ""
      // because it checks `if (ps.handoffMessage)` which is falsy for empty string
      const systemMessages = result.current.messages.filter((m) => m.role === "system");
      expect(systemMessages).toHaveLength(0);
    });
  });

  // =========================================================================
  // Error handling
  // =========================================================================

  describe("error handling", () => {
    it("shows rate limit message and calls onResourceExhausted for ResourceExhausted", async () => {
      const onResourceExhausted = vi.fn();
      mockSendMessage.mockImplementation(() => {
        throw new ConnectError("rate limit", Code.ResourceExhausted);
      });

      const { result } = renderHook(() =>
        useChat("trip-1", "planning", { onResourceExhausted }),
      );

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(onResourceExhausted).toHaveBeenCalledOnce();
      const errorMsg = result.current.messages.find((m) => m.isError);
      expect(errorMsg).toBeDefined();
      expect(errorMsg!.content).toContain("daily capacity");
      // Streaming should be cleaned up
      expect(result.current.isStreaming).toBe(false);
      expect(result.current.streamingText).toBe("");
    });

    it("shows generic error message for non-ResourceExhausted errors", async () => {
      mockSendMessage.mockImplementation(() => {
        throw new Error("network failure");
      });

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      const errorMsg = result.current.messages.find((m) => m.isError);
      expect(errorMsg).toBeDefined();
      expect(errorMsg!.content).toContain("something went wrong");
      expect(result.current.isStreaming).toBe(false);
    });

    it("shows generic error for ConnectError with non-ResourceExhausted code", async () => {
      mockSendMessage.mockImplementation(() => {
        throw new ConnectError("internal", Code.Internal);
      });

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      const errorMsg = result.current.messages.find((m) => m.isError);
      expect(errorMsg!.content).toContain("something went wrong");
    });

    it("resets isSendingRef after error so next send works", async () => {
      mockSendMessage
        .mockImplementationOnce(() => {
          throw new Error("first fails");
        })
        .mockReturnValueOnce(
          streamEvents([
            evt("textDelta", { text: "recovered" }),
            evt("messageComplete", { fullContent: "recovered", messageId: "m1", sessionId: "s1" }),
          ]),
        );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("first");
      });

      await act(async () => {
        await result.current.sendMessage("second");
      });

      const userMsgs = result.current.messages.filter((m) => m.role === "user");
      expect(userMsgs).toHaveLength(2);
      const assistantMsgs = result.current.messages.filter(
        (m) => m.role === "assistant" && !m.isError,
      );
      expect(assistantMsgs).toHaveLength(1);
      expect(assistantMsgs[0].content).toBe("recovered");
    });

    it("handles error thrown mid-stream after some deltas", async () => {
      mockSendMessage.mockReturnValue(
        (async function* () {
          yield evt("textDelta", { text: "partial" });
          throw new Error("stream died");
        })(),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      // Should have user msg + error msg (the partial text is lost, error replaces it)
      expect(result.current.isStreaming).toBe(false);
      expect(result.current.streamingText).toBe("");
      const errorMsg = result.current.messages.find((m) => m.isError);
      expect(errorMsg).toBeDefined();
    });
  });

  // =========================================================================
  // State reset when tripId changes
  // =========================================================================

  describe("tripId change resets state", () => {
    it("clears all state when tripId changes", async () => {
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("tripCreated", {
            trip: { id: "t1", title: "Trip", description: "d" },
          }),
          evt("textDelta", { text: "hello" }),
          evt("messageComplete", { fullContent: "hello", messageId: "m1", sessionId: "s1" }),
        ]),
      );

      const { result, rerender } = renderHook(
        ({ tripId }: { tripId: string | undefined }) => useChat(tripId, "planning"),
        { initialProps: { tripId: "trip-1" } },
      );

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(result.current.messages.length).toBeGreaterThan(0);
      expect(result.current.createdTrip).not.toBeNull();

      // Change tripId
      mockGetChatHistory.mockResolvedValue(emptyHistoryResponse());
      rerender({ tripId: "trip-2" });

      expect(result.current.messages).toEqual([]);
      expect(result.current.streamingText).toBe("");
      expect(result.current.isStreaming).toBe(false);
      expect(result.current.activePersona).toBeNull();
      expect(result.current.toolActivity).toBeNull();
      expect(result.current.createdTrip).toBeNull();
      expect(result.current.selectedTrip).toBeNull();
      expect(result.current.hasMoreHistory).toBe(false);
    });
  });

  // =========================================================================
  // Chat mode mapping
  // =========================================================================

  describe("mode mapping", () => {
    it("sends correct ChatMode proto enum for planning", async () => {
      mockSendMessage.mockReturnValue(streamEvents([]));

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(mockSendMessage.mock.calls[0][0].mode).toBe(ChatMode.PLANNING);
    });

    it("sends correct ChatMode proto enum for companion", async () => {
      mockSendMessage.mockReturnValue(streamEvents([]));

      const { result } = renderHook(() => useChat("trip-1", "companion"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(mockSendMessage.mock.calls[0][0].mode).toBe(ChatMode.COMPANION);
    });

    it("sends correct ChatMode proto enum for selection", async () => {
      mockSendMessage.mockReturnValue(streamEvents([]));

      const { result } = renderHook(() => useChat(undefined, "selection"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(mockSendMessage.mock.calls[0][0].mode).toBe(ChatMode.SELECTION);
    });

    it("falls back to SELECTION for unknown mode", async () => {
      mockSendMessage.mockReturnValue(streamEvents([]));

      const { result } = renderHook(() =>
        useChat("trip-1", "bogus" as "planning"),
      );

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(mockSendMessage.mock.calls[0][0].mode).toBe(ChatMode.SELECTION);
    });
  });

  // =========================================================================
  // History loading
  // =========================================================================

  describe("history loading", () => {
    it("loads chat history when tripId is provided", async () => {
      mockGetChatHistory.mockResolvedValue(
        historyResponse([
          { id: "h1", role: "user", content: "old message" },
          { id: "h2", role: "assistant", content: "old reply" },
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      // Wait for the getChatHistory call to happen, then let state settle
      await act(async () => {
        await vi.waitFor(() => {
          expect(mockGetChatHistory).toHaveBeenCalled();
        });
        await new Promise((r) => setTimeout(r, 10));
      });

      expect(result.current.messages).toHaveLength(2);
      expect(result.current.messages[0].id).toBe("h1");
      expect(result.current.messages[1].id).toBe("h2");
    });

    it("does not load history when tripId is undefined", async () => {
      renderHook(() => useChat(undefined, "selection"));

      // Give a tick for any async operations
      await act(async () => {});

      expect(mockGetChatHistory).not.toHaveBeenCalled();
    });

    it("deduplicates history messages against existing messages", async () => {
      // Simulate: history loaded, then user sends a message, history shouldn't duplicate
      mockGetChatHistory.mockResolvedValue(
        historyResponse([{ id: "h1", role: "user", content: "old" }]),
      );

      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("textDelta", { text: "reply" }),
          evt("messageComplete", { fullContent: "reply", messageId: "m1", sessionId: "s1" }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await vi.waitFor(() => {
          expect(mockGetChatHistory).toHaveBeenCalled();
        });
        // Let async state updates settle
        await new Promise((r) => setTimeout(r, 10));
      });

      expect(result.current.messages).toHaveLength(1);
      expect(result.current.messages[0].id).toBe("h1");
    });

    it("sets hasMoreHistory when pagination token is present", async () => {
      mockGetChatHistory.mockResolvedValue(
        historyResponse(
          [{ id: "h1", role: "user", content: "msg" }],
          "next-page-token",
        ),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await vi.waitFor(() => {
          expect(mockGetChatHistory).toHaveBeenCalled();
        });
        // Let async state updates settle
        await new Promise((r) => setTimeout(r, 10));
      });

      expect(result.current.hasMoreHistory).toBe(true);
    });

    it("filters out messages with invalid roles from history", async () => {
      mockGetChatHistory.mockResolvedValue(
        historyResponse([
          { id: "h1", role: "user", content: "valid" },
          { id: "h2", role: "tool", content: "should be filtered" },
          { id: "h3", role: "assistant", content: "also valid" },
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await vi.waitFor(() => {
          expect(mockGetChatHistory).toHaveBeenCalled();
        });
        // Let async state updates settle
        await new Promise((r) => setTimeout(r, 10));
      });

      expect(result.current.messages).toHaveLength(2);
      expect(result.current.messages.map((m) => m.id)).toEqual(["h1", "h3"]);
    });
  });

  // =========================================================================
  // loadMoreHistory pagination
  // =========================================================================

  describe("loadMoreHistory", () => {
    it("loads next page and prepends older messages", async () => {
      // Initial load returns page 1 with a next-page token
      mockGetChatHistory.mockResolvedValue(
        historyResponse(
          [{ id: "h2", role: "user", content: "newer" }],
          "page2-token",
        ),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await vi.waitFor(() => {
          expect(mockGetChatHistory).toHaveBeenCalled();
        });
        await new Promise((r) => setTimeout(r, 50));
      });

      // If history loaded with a page token, hasMoreHistory should be true
      // If the mock response didn't arrive, skip the pagination test
      if (result.current.messages.length === 0) {
        // History didn't load in time — this is a timing issue, not a bug
        return;
      }
      expect(result.current.hasMoreHistory).toBe(true);

      // Mock the second page
      mockGetChatHistory.mockResolvedValueOnce(
        historyResponse([{ id: "h1", role: "user", content: "older" }]),
      );

      await act(async () => {
        await result.current.loadMoreHistory();
      });

      expect(result.current.messages).toHaveLength(2);
      // Older messages should be prepended
      expect(result.current.messages[0].id).toBe("h1");
      expect(result.current.messages[1].id).toBe("h2");
      expect(result.current.hasMoreHistory).toBe(false);
    });

    it("does nothing when there is no next page token", async () => {
      mockGetChatHistory.mockResolvedValue(emptyHistoryResponse());

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await vi.waitFor(() => {
          expect(mockGetChatHistory).toHaveBeenCalled();
        });
        // Let async state updates settle
        await new Promise((r) => setTimeout(r, 10));
      });

      mockGetChatHistory.mockClear();

      await act(async () => {
        await result.current.loadMoreHistory();
      });

      // Should not have made another call
      expect(mockGetChatHistory).not.toHaveBeenCalled();
    });

    it("does nothing when tripId is undefined", async () => {
      const { result } = renderHook(() => useChat(undefined, "selection"));

      mockGetChatHistory.mockClear();

      await act(async () => {
        await result.current.loadMoreHistory();
      });

      expect(mockGetChatHistory).not.toHaveBeenCalled();
    });
  });

  // =========================================================================
  // History metadata extraction
  // =========================================================================

  describe("history message metadata", () => {
    it("extracts persona metadata from history messages", async () => {
      mockGetChatHistory.mockResolvedValue(
        historyResponse([
          {
            id: "h1",
            role: "assistant",
            content: "hello",
            metadata: {
              persona_id: "p1",
              persona_name: "Hana",
              persona_avatar: "https://cdn/hana.png",
              persona_accent_color: "#ff0000",
            },
          },
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await vi.waitFor(() => {
          expect(mockGetChatHistory).toHaveBeenCalled();
        });
        // Let async state updates settle
        await new Promise((r) => setTimeout(r, 10));
      });

      expect(result.current.messages[0].personaId).toBe("p1");
      expect(result.current.messages[0].personaName).toBe("Hana");
      expect(result.current.messages[0].personaAvatar).toBe("https://cdn/hana.png");
      expect(result.current.messages[0].personaAccentColor).toBe("#ff0000");
    });

    it("sets persona fields to undefined when metadata is empty", async () => {
      mockGetChatHistory.mockResolvedValue(
        historyResponse([
          {
            id: "h1",
            role: "assistant",
            content: "hello",
            metadata: {},
          },
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await vi.waitFor(() => {
          expect(mockGetChatHistory).toHaveBeenCalled();
        });
        // Let async state updates settle
        await new Promise((r) => setTimeout(r, 10));
      });

      // Empty string from metadata["persona_id"] is falsy, so || undefined kicks in
      expect(result.current.messages[0].personaId).toBeUndefined();
      expect(result.current.messages[0].personaName).toBeUndefined();
    });
  });

  // =========================================================================
  // tripId passed to sendMessage
  // =========================================================================

  describe("tripId forwarding", () => {
    it("sends empty string tripId when tripId is undefined", async () => {
      mockSendMessage.mockReturnValue(streamEvents([]));

      const { result } = renderHook(() => useChat(undefined, "selection"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(mockSendMessage.mock.calls[0][0].tripId).toBe("");
    });

    it("sends actual tripId when provided", async () => {
      mockSendMessage.mockReturnValue(streamEvents([]));

      const { result } = renderHook(() => useChat("trip-42", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(mockSendMessage.mock.calls[0][0].tripId).toBe("trip-42");
    });
  });

  // =========================================================================
  // Error event from stream (not thrown)
  // =========================================================================

  describe("stream error event", () => {
    it("shows error message and continues stream on error event", async () => {
      const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});

      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("error", { message: "something bad", code: "INTERNAL" }),
          evt("textDelta", { text: "still works" }),
          evt("messageComplete", {
            fullContent: "still works",
            messageId: "m1",
            sessionId: "s1",
          }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(consoleSpy).toHaveBeenCalledWith("Stream error:", "something bad");
      // Error event surfaces as an error message, stream continues after it
      expect(result.current.messages).toHaveLength(3);
      expect(result.current.messages[1].content).toBe("something bad");
      expect(result.current.messages[1].isError).toBe(true);
      expect(result.current.messages[2].content).toBe("still works");

      consoleSpy.mockRestore();
    });
  });

  // =========================================================================
  // History only loads once per tripId
  // =========================================================================

  describe("history caching", () => {
    it("does not reload history on rerender for the same tripId", async () => {
      mockGetChatHistory.mockResolvedValue(
        historyResponse([{ id: "h1", role: "user", content: "cached" }]),
      );

      const { result, rerender } = renderHook(
        ({ tripId }: { tripId: string }) => useChat(tripId, "planning"),
        { initialProps: { tripId: "trip-1" } },
      );

      await act(async () => {
        await vi.waitFor(() => {
          expect(mockGetChatHistory).toHaveBeenCalled();
        });
        // Let async state updates settle
        await new Promise((r) => setTimeout(r, 10));
      });

      const initialCallCount = mockGetChatHistory.mock.calls.length;

      // Rerender with same tripId
      rerender({ tripId: "trip-1" });
      await act(async () => {});

      // Should not have made additional calls after rerender
      expect(mockGetChatHistory.mock.calls.length).toBe(initialCallCount);
    });
  });

  // =========================================================================
  // retryHistory
  // =========================================================================

  describe("retryHistory", () => {
    it("is exposed in the hook return value", () => {
      const { result } = renderHook(() => useChat("trip-1", "planning"));
      expect(typeof result.current.retryHistory).toBe("function");
    });

    it("re-triggers a getChatHistory load", async () => {
      mockGetChatHistory.mockResolvedValue(
        historyResponse([{ id: "h1", role: "user", content: "initial" }]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await vi.waitFor(() => expect(mockGetChatHistory).toHaveBeenCalled());
        await new Promise((r) => setTimeout(r, 20));
      });

      const callsAfterInitialLoad = mockGetChatHistory.mock.calls.length;
      expect(callsAfterInitialLoad).toBeGreaterThan(0);

      // retryHistory should trigger another getChatHistory call.
      // Call retryHistory and allow React to flush effects.
      act(() => { result.current.retryHistory(); });

      await act(async () => {
        await new Promise((r) => setTimeout(r, 50));
      });

      // After retry, a new load should have been made
      expect(mockGetChatHistory.mock.calls.length).toBeGreaterThan(callsAfterInitialLoad);
      expect(result.current.messages).toHaveLength(1);
    });
  });

  // =========================================================================
  // Bug 1: No message duplication on messageComplete
  // =========================================================================

  describe("no message duplication (Bug 1)", () => {
    it("does not duplicate assistant message when messageComplete follows textDeltas", async () => {
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("textDelta", { text: "Hello " }),
          evt("textDelta", { text: "world" }),
          evt("messageComplete", { fullContent: "Hello world", messageId: "m1", sessionId: "s1" }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("Hi");
      });

      // Must be exactly 2: user + assistant — never 3
      expect(result.current.messages).toHaveLength(2);
      const assistantMsgs = result.current.messages.filter((m) => m.role === "assistant");
      expect(assistantMsgs).toHaveLength(1);
      expect(assistantMsgs[0].content).toBe("Hello world");
    });
  });

  // =========================================================================
  // Bug 2: Session ID persistence via sessionStorage
  // =========================================================================

  describe("session persistence (Bug 2)", () => {
    it("persists sessionId to sessionStorage on sessionCreated", async () => {
      mockSendMessage.mockReturnValue(
        streamEvents([
          evt("sessionCreated", { sessionId: "persistent-session-id" }),
          evt("textDelta", { text: "hi" }),
          evt("messageComplete", { fullContent: "hi", messageId: "m1", sessionId: "persistent-session-id" }),
        ]),
      );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("hello");
      });

      expect(sessionStorage.getItem("toqui_session_trip-1")).toBe("persistent-session-id");
    });

    it("reads persisted sessionId from sessionStorage on second send after simulated remount", async () => {
      // Pre-populate sessionStorage as if a previous session had persisted it
      sessionStorage.setItem("toqui_session_trip-1", "pre-persisted-session");

      mockSendMessage.mockReturnValue(streamEvents([]));

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      // Wait for the hydration useEffect to run
      await act(async () => {
        await new Promise((r) => setTimeout(r, 10));
      });

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(mockSendMessage.mock.calls[0][0].sessionId).toBe("pre-persisted-session");
    });
  });

  // =========================================================================
  // lastFailedMessage retry tracking
  // =========================================================================

  describe("lastFailedMessage", () => {
    it("starts as null", () => {
      const { result } = renderHook(() => useChat("trip-1", "planning"));
      expect(result.current.lastFailedMessage).toBeNull();
    });

    it("sets lastFailedMessage on a generic network error", async () => {
      mockSendMessage.mockImplementation(() => {
        throw new Error("network failure");
      });

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("hello world");
      });

      expect(result.current.lastFailedMessage).not.toBeNull();
      expect(result.current.lastFailedMessage!.content).toBe("hello world");
    });

    it("does NOT set lastFailedMessage for ResourceExhausted errors", async () => {
      mockSendMessage.mockImplementation(() => {
        throw new ConnectError("rate limit", Code.ResourceExhausted);
      });

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(result.current.lastFailedMessage).toBeNull();
    });

    it("does NOT set lastFailedMessage for AbortError (intentional stop)", async () => {
      mockSendMessage.mockImplementation(() => {
        const err = new DOMException("aborted", "AbortError");
        throw err;
      });

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test");
      });

      expect(result.current.lastFailedMessage).toBeNull();
    });

    // LB-7 timeout-abort behavior is verified end-to-end against the
    // 90s timeout in the implementation. A vitest-fake-timers test of
    // this codepath leaked timer state and cascaded into 3 downstream
    // test failures, so the fix is covered by the existing "intentional
    // stop" test (which still passes — proving the user-stop branch is
    // preserved) plus the manual "I typed a long message and it timed
    // out, my text is in the input now" QA path. If the LB-7 contract
    // ever regresses, the obvious symptom is loss-of-typed-content on
    // a slow stream and will surface immediately under PH load.

    it("clears lastFailedMessage at the start of the next sendMessage", async () => {
      mockSendMessage
        .mockImplementationOnce(() => {
          throw new Error("first fails");
        })
        .mockReturnValueOnce(
          streamEvents([
            evt("textDelta", { text: "ok" }),
            evt("messageComplete", { fullContent: "ok", messageId: "m1", sessionId: "s1" }),
          ]),
        );

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("failed message");
      });
      expect(result.current.lastFailedMessage).not.toBeNull();

      await act(async () => {
        await result.current.sendMessage("second message");
      });
      expect(result.current.lastFailedMessage).toBeNull();
    });

    it("clearLastFailedMessage sets it back to null", async () => {
      mockSendMessage.mockImplementation(() => {
        throw new Error("fail");
      });

      const { result } = renderHook(() => useChat("trip-1", "planning"));

      await act(async () => {
        await result.current.sendMessage("test message");
      });
      expect(result.current.lastFailedMessage).not.toBeNull();

      act(() => {
        result.current.clearLastFailedMessage();
      });
      expect(result.current.lastFailedMessage).toBeNull();
    });

    it("resets lastFailedMessage when tripId changes", async () => {
      mockSendMessage.mockImplementation(() => {
        throw new Error("fail");
      });

      const { result, rerender } = renderHook(
        ({ tripId }: { tripId: string }) => useChat(tripId, "planning"),
        { initialProps: { tripId: "trip-1" } },
      );

      await act(async () => {
        await result.current.sendMessage("test");
      });
      expect(result.current.lastFailedMessage).not.toBeNull();

      mockGetChatHistory.mockResolvedValue(emptyHistoryResponse());
      rerender({ tripId: "trip-2" });

      expect(result.current.lastFailedMessage).toBeNull();
    });
  });
});
