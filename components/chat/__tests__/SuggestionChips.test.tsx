import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SuggestionChips } from "../SuggestionChips";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      surface: "#ffffff",
      accent: "#e8654a",
    },
  }),
}));

// Create mock icon components
const MockIcon1 = () => <span data-testid="icon-1" />;
const MockIcon2 = () => <span data-testid="icon-2" />;
const MockIcon3 = () => <span data-testid="icon-3" />;

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("SuggestionChips", () => {
  const suggestions = [
    { key: "restaurants", icon: MockIcon1 as any, label: "Find restaurants" },
    { key: "hotels", icon: MockIcon2 as any, label: "Find hotels" },
    { key: "activities", icon: MockIcon3 as any, label: "Find activities" },
  ];

  it("renders all suggestion chips", () => {
    render(<SuggestionChips suggestions={suggestions} onSelect={vi.fn()} />);
    expect(screen.getByText("Find restaurants")).toBeInTheDocument();
    expect(screen.getByText("Find hotels")).toBeInTheDocument();
    expect(screen.getByText("Find activities")).toBeInTheDocument();
  });

  it("renders icons for each chip", () => {
    render(<SuggestionChips suggestions={suggestions} onSelect={vi.fn()} />);
    expect(screen.getByTestId("icon-1")).toBeInTheDocument();
    expect(screen.getByTestId("icon-2")).toBeInTheDocument();
    expect(screen.getByTestId("icon-3")).toBeInTheDocument();
  });

  it("calls onSelect with the chip label when a chip is pressed", () => {
    const onSelect = vi.fn();
    render(<SuggestionChips suggestions={suggestions} onSelect={onSelect} />);

    fireEvent.click(screen.getByText("Find restaurants"));
    expect(onSelect).toHaveBeenCalledWith("Find restaurants");

    fireEvent.click(screen.getByText("Find hotels"));
    expect(onSelect).toHaveBeenCalledWith("Find hotels");
  });

  it("renders empty when suggestions array is empty", () => {
    const { container } = render(
      <SuggestionChips suggestions={[]} onSelect={vi.fn()} />,
    );
    // ScrollView is still rendered but no chips inside
    expect(screen.queryByText("Find restaurants")).toBeNull();
  });

  it("calls onSelect only once per click", () => {
    const onSelect = vi.fn();
    render(<SuggestionChips suggestions={suggestions} onSelect={onSelect} />);

    fireEvent.click(screen.getByText("Find activities"));
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect).toHaveBeenCalledWith("Find activities");
  });
});
