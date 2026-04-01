import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { DatePicker } from "../DatePicker";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      textPrimary: "#111827",
      textTertiary: "#5f6673",
      inputBg: "#ffffff",
      inputBorder: "#d1d5db",
    },
  }),
}));

// ---------------------------------------------------------------------------
// Tests (web platform — jsdom renders the web variant)
// ---------------------------------------------------------------------------

describe("DatePicker", () => {
  it("renders without crashing", () => {
    const { container } = render(<DatePicker value="" onChange={vi.fn()} />);
    expect(container.querySelector('input[type="date"]')).not.toBeNull();
  });

  it("renders a date input with the provided value", () => {
    render(<DatePicker value="2025-06-15" onChange={vi.fn()} />);
    const input = screen.getByDisplayValue("2025-06-15") as HTMLInputElement;
    expect(input.type).toBe("date");
  });

  it("renders with an empty value", () => {
    const { container } = render(<DatePicker value="" onChange={vi.fn()} />);
    const input = container.querySelector('input[type="date"]') as HTMLInputElement;
    expect(input.value).toBe("");
  });

  it("calls onChange when date is selected", () => {
    const onChange = vi.fn();
    const { container } = render(<DatePicker value="" onChange={onChange} />);
    const input = container.querySelector('input[type="date"]') as HTMLInputElement;
    fireEvent.change(input, { target: { value: "2025-07-20" } });
    expect(onChange).toHaveBeenCalledWith("2025-07-20");
  });

  it("renders label when provided", () => {
    render(<DatePicker value="" onChange={vi.fn()} label="Start Date" />);
    expect(screen.getByText("Start Date")).toBeInTheDocument();
  });

  it("does not render label when not provided", () => {
    const { container } = render(<DatePicker value="" onChange={vi.fn()} />);
    // Only the input should be present, no label text
    const texts = container.querySelectorAll("div");
    // Just verify no "Start Date" text
    expect(screen.queryByText("Start Date")).toBeNull();
  });

  it("passes placeholder to the input", () => {
    const { container } = render(
      <DatePicker value="" onChange={vi.fn()} placeholder="Pick a date" />,
    );
    const input = container.querySelector('input[type="date"]') as HTMLInputElement;
    expect(input.placeholder).toBe("Pick a date");
  });
});
