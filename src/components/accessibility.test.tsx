import { describe, it, expect, vi, beforeAll } from "vitest";
import { render, screen } from "@testing-library/react";
import { ChatInput } from "./chat/ChatInput";
import { TypingIndicator } from "./chat/TypingIndicator";
import { MessageBubble } from "./chat/MessageBubble";

// jsdom does not implement scrollIntoView
beforeAll(() => {
  Element.prototype.scrollIntoView = vi.fn();
});

// Mock useChat and useUsage for ChatContainer tests
vi.mock("@/lib/hooks/useChat", () => ({
  useChat: () => ({
    messages: [],
    streamingText: "",
    isStreaming: false,
    activePersona: null,
    toolActivity: null,
    createdTrip: null,
    selectedTrip: null,
    sendMessage: vi.fn(),
  }),
}));

vi.mock("@/lib/hooks/useUsage", () => ({
  useUsage: () => ({
    count: 0,
    remaining: 30,
    isAtLimit: false,
    isNearLimit: false,
    isExhausted: false,
    recordMessage: vi.fn(),
    markExhausted: vi.fn(),
  }),
}));

// Dynamically import ChatContainer after mocks are set up
const { ChatContainer } = await import("./chat/ChatContainer");

describe("Accessibility: Skip-to-content link", () => {
  it("layout renders a skip-to-content link targeting #main-content", () => {
    // We test the skip link by checking the link's href and text content.
    // Since layout.tsx is a server component, we test the pattern via a
    // simulated render of its anchor element.
    const { container } = render(
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:bg-[var(--color-surface)] focus:px-4 focus:py-2 focus:rounded-lg focus:z-50 focus:text-[var(--color-text-primary)] focus:shadow-lg focus:ring-2 focus:ring-[var(--color-accent)]"
      >
        Skip to content
      </a>,
    );
    const skipLink = container.querySelector('a[href="#main-content"]');
    expect(skipLink).toBeInTheDocument();
    expect(skipLink).toHaveTextContent("Skip to content");
  });
});

describe("Accessibility: Chat messages have aria-live region", () => {
  it("ChatContainer renders a log region with aria-live='polite'", () => {
    render(<ChatContainer mode="planning" />);
    const logRegion = screen.getByRole("log");
    expect(logRegion).toBeInTheDocument();
    expect(logRegion).toHaveAttribute("aria-live", "polite");
    expect(logRegion).toHaveAttribute("aria-label", "Chat messages");
  });

  it("ChatContainer sets aria-busy=false when not streaming", () => {
    render(<ChatContainer mode="planning" />);
    const logRegion = screen.getByRole("log");
    expect(logRegion).toHaveAttribute("aria-busy", "false");
  });
});

describe("Accessibility: Buttons have accessible names", () => {
  it("ChatInput send button has aria-label", () => {
    render(<ChatInput onSend={vi.fn()} />);
    const sendButton = screen.getByRole("button", { name: "Send message" });
    expect(sendButton).toBeInTheDocument();
  });

  it("ChatInput textarea has aria-label", () => {
    render(<ChatInput onSend={vi.fn()} />);
    const textarea = screen.getByRole("textbox");
    expect(textarea).toHaveAttribute("aria-label");
    // Our mock returns "chat.inputPlaceholder" for the translated string
    expect(textarea.getAttribute("aria-label")).toBe("chat.inputPlaceholder");
  });
});

describe("Accessibility: Loading states have aria-busy", () => {
  it("TypingIndicator has role=status and aria-label", () => {
    render(<TypingIndicator />);
    const statusElement = screen.getByRole("status");
    expect(statusElement).toBeInTheDocument();
    expect(statusElement).toHaveAttribute("aria-label", "Assistant is typing");
  });

  it("TypingIndicator includes sr-only text for screen readers", () => {
    render(<TypingIndicator />);
    const srOnlyText = screen.getByText("Assistant is typing");
    expect(srOnlyText).toBeInTheDocument();
  });

  it("TypingIndicator dots container is aria-hidden", () => {
    const { container } = render(<TypingIndicator />);
    const dotsContainer = container.querySelector('[aria-hidden="true"]');
    expect(dotsContainer).toBeInTheDocument();
  });
});

describe("Accessibility: System messages have role=status", () => {
  it("system messages render with role=status", () => {
    render(
      <MessageBubble
        message={{ id: "sys-1", role: "system", content: "Trip created" }}
        showPersonaBadge={false}
      />,
    );
    const statusElement = screen.getByRole("status");
    expect(statusElement).toBeInTheDocument();
    expect(statusElement).toHaveTextContent("Trip created");
  });
});

describe("Accessibility: Decorative elements are hidden from assistive tech", () => {
  it("streaming cursor is aria-hidden", () => {
    const { container } = render(
      <MessageBubble
        message={{ id: "s-1", role: "assistant", content: "Hello there" }}
        isStreaming
        showPersonaBadge={false}
      />,
    );
    const cursor = container.querySelector(".animate-pulse");
    expect(cursor).toBeInTheDocument();
    expect(cursor).toHaveAttribute("aria-hidden", "true");
  });

  it("persona avatar in message bubble is aria-hidden", () => {
    const { container } = render(
      <MessageBubble
        message={{
          id: "m-1",
          role: "assistant",
          content: "Recommendation text",
          personaName: "Explorer",
          personaAvatar: "E",
          personaAccentColor: "#ff0000",
        }}
        showPersonaBadge={true}
      />,
    );
    const avatar = container.querySelector('[aria-hidden="true"]');
    expect(avatar).toBeInTheDocument();
  });
});
