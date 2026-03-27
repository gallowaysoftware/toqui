import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MessageBubble } from "../MessageBubble";
import type { ChatMessage } from "@/lib/hooks/useChat";

/** Normalize hex (#rrggbb) to rgb(r, g, b) for comparison with computed styles */
function hexToRgb(hex: string): string {
  const r = parseInt(hex.slice(1, 3), 16);
  const g = parseInt(hex.slice(3, 5), 16);
  const b = parseInt(hex.slice(5, 7), 16);
  return `rgb(${r}, ${g}, ${b})`;
}

function expectBgColor(el: HTMLElement, hex: string) {
  const actual = el.style.backgroundColor;
  // React Native Web may return hex or rgb
  expect(actual === hex || actual === hexToRgb(hex)).toBe(true);
}

// Use vi.hoisted so mockColors is available in hoisted vi.mock factories
const mockColors = vi.hoisted(() => ({
  surface: "#ffffff",
  surfaceSecondary: "#f9fafb",
  surfaceTertiary: "#f3f4f6",
  border: "#e5e7eb",
  borderStrong: "#d1d5db",
  textPrimary: "#111827",
  textSecondary: "#4b5563",
  textTertiary: "#5f6673",
  accent: "#e8654a",
  accentHover: "#c44a32",
  accentSoft: "#fef2f0",
  error: "#dc2626",
  errorBg: "#fef2f2",
  success: "#16a34a",
  successBg: "#f0fdf4",
  userBubble: "#e8654a",
  userBubbleText: "#ffffff",
  assistantBubble: "#ffffff",
  assistantBubbleText: "#1f2937",
  assistantBubbleBorder: "#e5e7eb",
  inputBg: "#ffffff",
  inputBorder: "#d1d5db",
}));

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({ colors: mockColors, mode: "light", isDark: false, setMode: () => {} }),
}));

// Mock react-native-markdown-display to render children as plain HTML
vi.mock("react-native-markdown-display", () => ({
  __esModule: true,
  default: ({ children }: { children: string }) => (
    <div data-testid="markdown-content">{children}</div>
  ),
}));

function makeMessage(overrides: Partial<ChatMessage>): ChatMessage {
  return {
    id: "msg-1",
    role: "user",
    content: "Hello world",
    ...overrides,
  };
}

describe("MessageBubble", () => {
  describe("user messages", () => {
    it("renders user message content as plain text, not markdown", () => {
      render(<MessageBubble message={makeMessage({ role: "user", content: "Book me a flight" })} />);
      expect(screen.getByText("Book me a flight")).toBeInTheDocument();
      // User messages should NOT go through Markdown renderer
      expect(screen.queryByTestId("markdown-content")).toBeNull();
    });

    it("applies user bubble background color", () => {
      const { container } = render(
        <MessageBubble message={makeMessage({ role: "user", content: "test" })} />,
      );
      // The outermost View should have the userBubble background
      const bubble = container.firstChild as HTMLElement;
      expectBgColor(bubble, mockColors.userBubble);
    });
  });

  describe("assistant messages", () => {
    it("renders assistant message content through Markdown component", () => {
      render(
        <MessageBubble message={makeMessage({ role: "assistant", content: "**Bold** text" })} />,
      );
      const md = screen.getByTestId("markdown-content");
      expect(md).toBeInTheDocument();
      expect(md.textContent).toBe("**Bold** text");
    });

    it("applies assistant bubble background and border", () => {
      const { container } = render(
        <MessageBubble message={makeMessage({ role: "assistant", content: "hi" })} />,
      );
      const bubble = container.firstChild as HTMLElement;
      expectBgColor(bubble, mockColors.assistantBubble);
      // RN web may split borderColor into individual sides
      const hasBorder =
        bubble.style.borderColor === mockColors.assistantBubbleBorder ||
        bubble.style.borderColor === hexToRgb(mockColors.assistantBubbleBorder) ||
        bubble.style.borderTopColor !== "";
      expect(hasBorder).toBe(true);
    });
  });

  describe("system messages", () => {
    it("renders system message with italic centered text", () => {
      render(
        <MessageBubble message={makeMessage({ role: "system", content: "Trip created" })} />,
      );
      expect(screen.getByText("Trip created")).toBeInTheDocument();
    });

    it("applies surfaceTertiary background for system messages", () => {
      const { container } = render(
        <MessageBubble message={makeMessage({ role: "system", content: "info" })} />,
      );
      const bubble = container.firstChild as HTMLElement;
      expectBgColor(bubble, mockColors.surfaceTertiary);
    });

    it("does not render markdown for system messages", () => {
      render(
        <MessageBubble message={makeMessage({ role: "system", content: "**not bold**" })} />,
      );
      // System messages use plain Text, not Markdown
      expect(screen.queryByTestId("markdown-content")).toBeNull();
      expect(screen.getByText("**not bold**")).toBeInTheDocument();
    });
  });

  describe("persona header", () => {
    it("shows persona name and dot for assistant messages with personaName", () => {
      render(
        <MessageBubble
          message={makeMessage({
            role: "assistant",
            content: "I can help",
            personaName: "Chef Marco",
            personaAccentColor: "#ff0000",
          })}
        />,
      );
      expect(screen.getByText("Chef Marco")).toBeInTheDocument();
    });

    it("does not show persona header for user messages even if personaName is set", () => {
      render(
        <MessageBubble
          message={makeMessage({
            role: "user",
            content: "hi",
            personaName: "Chef Marco",
          })}
        />,
      );
      expect(screen.queryByText("Chef Marco")).toBeNull();
    });

    it("does not show persona header when personaName is absent", () => {
      const { container } = render(
        <MessageBubble message={makeMessage({ role: "assistant", content: "hi" })} />,
      );
      // No persona dot should exist
      const allText = container.textContent;
      expect(allText).toBe("hi");
    });

    it("uses accent color as fallback when personaAccentColor is undefined", () => {
      const { container } = render(
        <MessageBubble
          message={makeMessage({
            role: "assistant",
            content: "test",
            personaName: "Guide",
            personaAccentColor: undefined,
          })}
        />,
      );
      // The persona dot should fall back to the theme accent color
      // Find the dot element (small 8x8 circle before the name)
      const nameEl = screen.getByText("Guide");
      const header = nameEl.parentElement!;
      const dot = header.firstChild as HTMLElement;
      expectBgColor(dot, mockColors.accent);
    });

    it("uses personaAccentColor when provided", () => {
      const { container } = render(
        <MessageBubble
          message={makeMessage({
            role: "assistant",
            content: "test",
            personaName: "Guide",
            personaAccentColor: "#00ff00",
          })}
        />,
      );
      const nameEl = screen.getByText("Guide");
      const header = nameEl.parentElement!;
      const dot = header.firstChild as HTMLElement;
      expectBgColor(dot, "#00ff00");
    });
  });

  describe("error styling", () => {
    it("applies error background and border when isError is true", () => {
      const { container } = render(
        <MessageBubble
          message={makeMessage({
            role: "assistant",
            content: "Something went wrong",
            isError: true,
          })}
        />,
      );
      const bubble = container.firstChild as HTMLElement;
      expectBgColor(bubble, mockColors.errorBg);
      // RN web may split borderColor into individual sides
      const hasBorder =
        bubble.style.borderColor === mockColors.error ||
        bubble.style.borderColor === hexToRgb(mockColors.error) ||
        bubble.style.borderTopColor !== "";
      expect(hasBorder).toBe(true);
    });

    it("does not apply error styles when isError is false/undefined", () => {
      const { container } = render(
        <MessageBubble message={makeMessage({ role: "assistant", content: "ok" })} />,
      );
      const bubble = container.firstChild as HTMLElement;
      // Should NOT have error background
const bgColor = bubble.style.backgroundColor;
expect(bgColor !== mockColors.errorBg && bgColor !== hexToRgb(mockColors.errorBg)).toBe(true);
    });
  });
});
