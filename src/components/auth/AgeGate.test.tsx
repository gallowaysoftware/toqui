import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AgeGate } from "./AgeGate";

// Mock next/navigation (already mocked in setup.tsx, but override usePathname per-test)
const mockPathname = vi.fn(() => "/");
vi.mock("next/navigation", async () => {
  const actual = await vi.importActual("next/navigation");
  return {
    ...actual,
    usePathname: () => mockPathname(),
  };
});

function fillDOB(month: string, day: string, year: string) {
  fireEvent.change(screen.getByLabelText("Month"), { target: { value: month } });
  fireEvent.change(screen.getByLabelText("Day"), { target: { value: day } });
  fireEvent.change(screen.getByLabelText("Year"), { target: { value: year } });
}

function submitForm() {
  const form = document.querySelector("form")!;
  fireEvent.submit(form);
}

describe("AgeGate", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.useRealTimers();
    mockPathname.mockReturnValue("/");
  });

  it("shows age gate when not verified", () => {
    render(
      <AgeGate>
        <div>App Content</div>
      </AgeGate>,
    );
    expect(screen.getByText("Welcome to Toqui")).toBeInTheDocument();
    expect(screen.queryByText("App Content")).not.toBeInTheDocument();
  });

  it("shows children when already verified", () => {
    localStorage.setItem("toqui_age_verified", "true");
    render(
      <AgeGate>
        <div>App Content</div>
      </AgeGate>,
    );
    expect(screen.getByText("App Content")).toBeInTheDocument();
    expect(screen.queryByText("Welcome to Toqui")).not.toBeInTheDocument();
  });

  it("allows user aged 18+ to proceed", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 2, 8)); // March 8, 2026

    render(
      <AgeGate>
        <div>App Content</div>
      </AgeGate>,
    );

    fillDOB("3", "8", "2008");
    submitForm();

    expect(screen.getByText("App Content")).toBeInTheDocument();
    expect(localStorage.getItem("toqui_age_verified")).toBe("true");
  });

  it("denies user under 18", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 2, 8));

    render(
      <AgeGate>
        <div>App Content</div>
      </AgeGate>,
    );

    fillDOB("3", "9", "2008");
    submitForm();

    expect(screen.getByText("Age Requirement Not Met")).toBeInTheDocument();
    expect(screen.queryByText("App Content")).not.toBeInTheDocument();
    expect(localStorage.getItem("toqui_age_verified")).toBeNull();
  });

  it("shows error for incomplete form", () => {
    render(
      <AgeGate>
        <div>App Content</div>
      </AgeGate>,
    );

    submitForm();
    expect(screen.getByRole("alert")).toHaveTextContent("Please enter your complete date of birth.");
  });

  it("shows error for unreasonably old date", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 2, 8));

    const { container } = render(
      <AgeGate>
        <div>App Content</div>
      </AgeGate>,
    );

    fillDOB("1", "1", "1800");
    fireEvent.submit(container.querySelector("form")!);

    expect(screen.getByRole("alert")).toHaveTextContent("Please enter a valid date of birth.");
  });

  it("shows error for invalid date (Feb 30)", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 2, 8));

    const { container } = render(
      <AgeGate>
        <div>App Content</div>
      </AgeGate>,
    );

    fillDOB("2", "30", "2000");
    fireEvent.submit(container.querySelector("form")!);

    expect(screen.getByRole("alert")).toHaveTextContent("Please enter a valid date.");
  });

  it("shows error for Feb 29 in non-leap year", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 2, 8));

    const { container } = render(
      <AgeGate>
        <div>App Content</div>
      </AgeGate>,
    );

    fillDOB("2", "29", "2007"); // 2007 is not a leap year
    fireEvent.submit(container.querySelector("form")!);

    expect(screen.getByRole("alert")).toHaveTextContent("Please enter a valid date.");
  });

  it("accepts Feb 29 in leap year", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 2, 8));

    render(
      <AgeGate>
        <div>App Content</div>
      </AgeGate>,
    );

    fillDOB("2", "29", "2008"); // 2008 is a leap year, person is 17 turning 18
    submitForm();

    // Feb 29 2008 → person is 17 on March 8, 2026 (birthday hasn't occurred yet in 2026)
    // Actually, Feb 29 already passed by March 8, so they are 18
    expect(screen.getByText("App Content")).toBeInTheDocument();
  });

  it("exempts /privacy from age gate", () => {
    mockPathname.mockReturnValue("/privacy");
    render(
      <AgeGate>
        <div>Privacy Content</div>
      </AgeGate>,
    );
    expect(screen.getByText("Privacy Content")).toBeInTheDocument();
    expect(screen.queryByText("Welcome to Toqui")).not.toBeInTheDocument();
  });

  it("exempts /terms from age gate", () => {
    mockPathname.mockReturnValue("/terms");
    render(
      <AgeGate>
        <div>Terms Content</div>
      </AgeGate>,
    );
    expect(screen.getByText("Terms Content")).toBeInTheDocument();
    expect(screen.queryByText("Welcome to Toqui")).not.toBeInTheDocument();
  });

  it("exempts /auth/callback from age gate", () => {
    mockPathname.mockReturnValue("/auth/callback");
    render(
      <AgeGate>
        <div>Auth Callback</div>
      </AgeGate>,
    );
    expect(screen.getByText("Auth Callback")).toBeInTheDocument();
    expect(screen.queryByText("Welcome to Toqui")).not.toBeInTheDocument();
  });

  it("exempts /waitlist from age gate", () => {
    mockPathname.mockReturnValue("/waitlist");
    render(
      <AgeGate>
        <div>Waitlist Content</div>
      </AgeGate>,
    );
    expect(screen.getByText("Waitlist Content")).toBeInTheDocument();
    expect(screen.queryByText("Welcome to Toqui")).not.toBeInTheDocument();
  });

  it("does NOT exempt /trips from age gate", () => {
    mockPathname.mockReturnValue("/trips");
    render(
      <AgeGate>
        <div>Trips Content</div>
      </AgeGate>,
    );
    expect(screen.getByText("Welcome to Toqui")).toBeInTheDocument();
    expect(screen.queryByText("Trips Content")).not.toBeInTheDocument();
  });

  it("links to Terms of Service and Privacy Policy", () => {
    render(
      <AgeGate>
        <div>App Content</div>
      </AgeGate>,
    );

    expect(screen.getByText("Terms of Service").closest("a")).toHaveAttribute("href", "/terms");
    expect(screen.getByText("Privacy Policy").closest("a")).toHaveAttribute("href", "/privacy");
  });
});
