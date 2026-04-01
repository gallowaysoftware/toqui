import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { TypingIndicator } from "../TypingIndicator";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      assistantBubble: "#ffffff",
      assistantBubbleBorder: "#e5e7eb",
      textTertiary: "#5f6673",
      border: "#e5e7eb",
    },
  }),
}));

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("TypingIndicator", () => {
  it("renders without crashing", () => {
    const { container } = render(<TypingIndicator />);
    expect(container.firstChild).not.toBeNull();
  });

  it("shows dots when no toolName is provided", () => {
    const { container } = render(<TypingIndicator />);
    // Should not show any text label
    expect(screen.queryByText(/Using/)).toBeNull();
    // Should have accessibility label "AI is typing"
    expect(container.querySelector('[aria-label="AI is typing"]')).not.toBeNull();
  });

  it("shows tool name when toolName is provided", () => {
    render(<TypingIndicator toolName="search_flights" />);
    expect(screen.getByText("Using search_flights...")).toBeInTheDocument();
  });

  it("has accessibility label with tool name", () => {
    const { container } = render(<TypingIndicator toolName="search_hotels" />);
    expect(
      container.querySelector('[aria-label="Using search_hotels"]'),
    ).not.toBeNull();
  });

  it("shows dots when toolName is null", () => {
    const { container } = render(<TypingIndicator toolName={null} />);
    expect(screen.queryByText(/Using/)).toBeNull();
    expect(container.querySelector('[aria-label="AI is typing"]')).not.toBeNull();
  });

  it("shows dots when toolName is undefined", () => {
    const { container } = render(<TypingIndicator toolName={undefined} />);
    expect(screen.queryByText(/Using/)).toBeNull();
    expect(container.querySelector('[aria-label="AI is typing"]')).not.toBeNull();
  });
});
