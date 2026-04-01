import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { PersonaIntroCard } from "../PersonaIntroCard";
import type { PersonaIntroData } from "@/lib/hooks/useChat";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      surface: "#ffffff",
      textPrimary: "#111827",
      textSecondary: "#4b5563",
      accent: "#e8654a",
    },
  }),
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makePersona(overrides?: Partial<PersonaIntroData>): PersonaIntroData {
  return {
    name: "Chef Marco",
    specialties: ["Italian cuisine", "Wine pairing"],
    accentColor: "#ff6b35",
    avatarUrl: "",
    handoffMessage: "I can help you find the best restaurants in Rome.",
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("PersonaIntroCard", () => {
  it("renders persona name", () => {
    render(<PersonaIntroCard persona={makePersona()} />);
    expect(screen.getByText("Chef Marco")).toBeInTheDocument();
  });

  it("renders the meet label", () => {
    render(<PersonaIntroCard persona={makePersona()} />);
    expect(screen.getByText("Meet your expert")).toBeInTheDocument();
  });

  it("renders handoff message", () => {
    render(<PersonaIntroCard persona={makePersona()} />);
    expect(
      screen.getByText("I can help you find the best restaurants in Rome."),
    ).toBeInTheDocument();
  });

  it("renders specialties joined with dot separator", () => {
    render(<PersonaIntroCard persona={makePersona()} />);
    expect(screen.getByText("Italian cuisine \u00B7 Wine pairing")).toBeInTheDocument();
  });

  it("does not render specialties when empty", () => {
    render(<PersonaIntroCard persona={makePersona({ specialties: [] })} />);
    expect(screen.queryByText(/\u00B7/)).toBeNull();
  });

  it("shows avatar fallback with first letter when no avatarUrl", () => {
    render(<PersonaIntroCard persona={makePersona({ avatarUrl: "" })} />);
    expect(screen.getByText("C")).toBeInTheDocument();
  });

  it("uses uppercase first letter for avatar fallback", () => {
    render(<PersonaIntroCard persona={makePersona({ name: "guide anna" })} />);
    expect(screen.getByText("G")).toBeInTheDocument();
  });

  it("renders image when avatarUrl is provided", () => {
    const { container } = render(
      <PersonaIntroCard
        persona={makePersona({ avatarUrl: "https://example.com/avatar.png" })}
      />,
    );
    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    expect(img!.src).toBe("https://example.com/avatar.png");
  });

  it("uses theme accent color when persona has no accentColor", () => {
    render(
      <PersonaIntroCard persona={makePersona({ accentColor: "" })} />,
    );
    // Component falls back to colors.accent ("#e8654a") when accentColor is falsy
    // Just verify it renders without error
    expect(screen.getByText("Chef Marco")).toBeInTheDocument();
  });
});
