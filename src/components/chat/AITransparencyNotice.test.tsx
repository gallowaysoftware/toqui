import { describe, it, expect, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AITransparencyNotice } from "./AITransparencyNotice";

describe("AITransparencyNotice", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("renders the AI notice on first visit", () => {
    render(<AITransparencyNotice />);
    expect(screen.getByText(/Toqui uses AI to generate responses/)).toBeInTheDocument();
  });

  it("has a dismiss button with accessible label", () => {
    render(<AITransparencyNotice />);
    expect(screen.getByLabelText("Dismiss AI notice")).toBeInTheDocument();
  });

  it("has the note role for accessibility", () => {
    render(<AITransparencyNotice />);
    expect(screen.getByRole("note")).toBeInTheDocument();
  });

  it("hides notice when dismiss button is clicked", () => {
    render(<AITransparencyNotice />);
    fireEvent.click(screen.getByLabelText("Dismiss AI notice"));
    expect(screen.queryByText(/Toqui uses AI to generate responses/)).not.toBeInTheDocument();
  });

  it("persists dismissal to localStorage", () => {
    render(<AITransparencyNotice />);
    fireEvent.click(screen.getByLabelText("Dismiss AI notice"));
    expect(localStorage.getItem("toqui_ai_notice_dismissed")).toBe("1");
  });

  it("does not render if previously dismissed", () => {
    localStorage.setItem("toqui_ai_notice_dismissed", "1");
    render(<AITransparencyNotice />);
    expect(screen.queryByText(/Toqui uses AI to generate responses/)).not.toBeInTheDocument();
  });

  it("includes verification guidance in the notice text", () => {
    render(<AITransparencyNotice />);
    expect(
      screen.getByText(/Verify important details before making travel decisions/),
    ).toBeInTheDocument();
  });
});
