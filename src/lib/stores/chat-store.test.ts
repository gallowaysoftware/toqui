import { describe, it, expect, beforeEach } from "vitest";
import { useChatStore } from "./chat-store";

describe("useChatStore", () => {
  beforeEach(() => {
    // Reset store to initial state between tests
    useChatStore.setState({
      messages: [],
      streamingText: "",
      isStreaming: false,
      sessionId: null,
    });
  });

  it("has correct initial state", () => {
    const state = useChatStore.getState();
    expect(state.messages).toEqual([]);
    expect(state.streamingText).toBe("");
    expect(state.isStreaming).toBe(false);
    expect(state.sessionId).toBeNull();
  });

  it("adds a message", () => {
    const msg = {
      id: "msg-1",
      role: "user",
      content: "Hello",
      createdAt: new Date("2026-01-01"),
    };

    useChatStore.getState().addMessage(msg);

    const state = useChatStore.getState();
    expect(state.messages).toHaveLength(1);
    expect(state.messages[0]).toEqual(msg);
  });

  it("adds multiple messages in order", () => {
    const msg1 = {
      id: "msg-1",
      role: "user",
      content: "Hello",
      createdAt: new Date("2026-01-01"),
    };
    const msg2 = {
      id: "msg-2",
      role: "assistant",
      content: "Hi there!",
      createdAt: new Date("2026-01-01"),
    };

    useChatStore.getState().addMessage(msg1);
    useChatStore.getState().addMessage(msg2);

    const state = useChatStore.getState();
    expect(state.messages).toHaveLength(2);
    expect(state.messages[0].id).toBe("msg-1");
    expect(state.messages[1].id).toBe("msg-2");
  });

  it("sets streaming text", () => {
    useChatStore.getState().setStreamingText("Generating response...");
    expect(useChatStore.getState().streamingText).toBe("Generating response...");
  });

  it("sets isStreaming flag", () => {
    useChatStore.getState().setIsStreaming(true);
    expect(useChatStore.getState().isStreaming).toBe(true);

    useChatStore.getState().setIsStreaming(false);
    expect(useChatStore.getState().isStreaming).toBe(false);
  });

  it("sets session ID", () => {
    useChatStore.getState().setSessionId("session-abc");
    expect(useChatStore.getState().sessionId).toBe("session-abc");
  });

  it("clears all messages and resets state", () => {
    // Set up some state
    useChatStore.getState().addMessage({
      id: "msg-1",
      role: "user",
      content: "Hello",
      createdAt: new Date(),
    });
    useChatStore.getState().setStreamingText("partial");
    useChatStore.getState().setSessionId("session-abc");

    // Clear
    useChatStore.getState().clearMessages();

    const state = useChatStore.getState();
    expect(state.messages).toEqual([]);
    expect(state.streamingText).toBe("");
    expect(state.sessionId).toBeNull();
  });
});
