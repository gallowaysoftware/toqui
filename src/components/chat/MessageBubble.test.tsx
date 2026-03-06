import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MessageBubble } from "./MessageBubble";
import type { ChatMessage } from "@/lib/hooks/useChat";

// Mock react-markdown to avoid ESM/rendering complexity in tests
vi.mock("react-markdown", () => ({
  default: ({ children }: { children: string }) => <div data-testid="markdown">{children}</div>,
}));

describe("MessageBubble", () => {
  const userMessage: ChatMessage = {
    id: "msg-1",
    role: "user",
    content: "Where should I go in Tokyo?",
  };

  const assistantMessage: ChatMessage = {
    id: "msg-2",
    role: "assistant",
    content: "I recommend visiting Shibuya and Shinjuku!",
  };

  const systemMessage: ChatMessage = {
    id: "msg-3",
    role: "system",
    content: "Persona switched to local expert",
  };

  it("renders user message content", () => {
    render(<MessageBubble message={userMessage} />);
    expect(screen.getByText("Where should I go in Tokyo?")).toBeInTheDocument();
  });

  it("renders assistant message content via markdown", () => {
    render(<MessageBubble message={assistantMessage} />);
    expect(
      screen.getByText("I recommend visiting Shibuya and Shinjuku!"),
    ).toBeInTheDocument();
  });

  it("renders system message as centered text", () => {
    render(<MessageBubble message={systemMessage} />);
    const systemText = screen.getByText("Persona switched to local expert");
    expect(systemText).toBeInTheDocument();
    // System messages are rendered in a <p> tag
    expect(systemText.tagName).toBe("P");
  });

  it("aligns user messages to the right", () => {
    const { container } = render(<MessageBubble message={userMessage} />);
    const wrapper = container.firstElementChild as HTMLElement;
    expect(wrapper.className).toContain("justify-end");
  });

  it("aligns assistant messages to the left", () => {
    const { container } = render(<MessageBubble message={assistantMessage} />);
    const wrapper = container.firstElementChild as HTMLElement;
    expect(wrapper.className).toContain("justify-start");
  });

  it("aligns system messages to the center", () => {
    const { container } = render(<MessageBubble message={systemMessage} />);
    const wrapper = container.firstElementChild as HTMLElement;
    expect(wrapper.className).toContain("justify-center");
  });

  it("shows streaming cursor when isStreaming is true for assistant", () => {
    const { container } = render(
      <MessageBubble message={assistantMessage} isStreaming={true} />,
    );
    const cursor = container.querySelector(".animate-pulse");
    expect(cursor).toBeInTheDocument();
  });

  it("does not show streaming cursor when isStreaming is false", () => {
    const { container } = render(
      <MessageBubble message={assistantMessage} isStreaming={false} />,
    );
    const cursor = container.querySelector(".animate-pulse");
    expect(cursor).not.toBeInTheDocument();
  });

  it("shows persona badge when showPersonaBadge is true and persona info exists", () => {
    const personaMessage: ChatMessage = {
      id: "msg-4",
      role: "assistant",
      content: "Welcome to Tokyo!",
      personaName: "Hana",
      personaAvatar: "https://example.com/hana.png",
      personaAccentColor: "#ff6b6b",
    };

    render(<MessageBubble message={personaMessage} showPersonaBadge={true} />);
    expect(screen.getByText("Hana")).toBeInTheDocument();
  });

  it("does not show persona badge when showPersonaBadge is false", () => {
    const personaMessage: ChatMessage = {
      id: "msg-5",
      role: "assistant",
      content: "Welcome to Tokyo!",
      personaName: "Hana",
      personaAvatar: "https://example.com/hana.png",
    };

    render(<MessageBubble message={personaMessage} showPersonaBadge={false} />);
    expect(screen.queryByText("Hana")).not.toBeInTheDocument();
  });
});
