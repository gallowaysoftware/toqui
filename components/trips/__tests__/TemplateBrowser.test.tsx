import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { TemplateBrowser } from "../TemplateBrowser";

const mockPush = vi.fn();

vi.mock("expo-router", () => ({
  useRouter: () => ({ push: mockPush }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (key === "templates.duration") return `${opts?.count} days`;
      // Return the full key so each template has a unique text
      return key;
    },
  }),
}));

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      surface: "#fff",
      surfaceSecondary: "#f5f5f5",
      surfaceTertiary: "#f0f0f0",
      inputBg: "#fff",
      inputBorder: "#e0e0e0",
      textPrimary: "#333",
      textSecondary: "#666",
      textTertiary: "#999",
      border: "#e0e0e0",
      accent: "#BF4028",
      accentSoft: "#fef3f0",
    },
  }),
}));

vi.mock("lucide-react-native", () => {
  const Stub = () => null;
  return {
    Heart: Stub,
    Landmark: Stub,
    TreePalm: Stub,
    Building2: Stub,
    Car: Stub,
    Sun: Stub,
    Backpack: Stub,
    Users: Stub,
    Sailboat: Stub,
    Compass: Stub,
    Search: Stub,
    Clock: Stub,
    MapPin: Stub,
  };
});

describe("TemplateBrowser", () => {
  beforeEach(() => {
    mockPush.mockClear();
  });

  it("renders all template cards", () => {
    render(<TemplateBrowser />);
    // Each template title renders with its full i18n key
    expect(screen.getByText("templates.items.parisWeekend.title")).toBeTruthy();
    expect(screen.getByText("templates.items.tokyoExplorer.title")).toBeTruthy();
    expect(screen.getByText("templates.items.moroccoDiscovery.title")).toBeTruthy();
    // 10 templates + 6 category chips = 16 buttons
    const buttons = screen.getAllByRole("button");
    expect(buttons.length).toBe(16);
  });

  it("navigates to new trip with template param on card tap", () => {
    render(<TemplateBrowser />);
    const parisCard = screen.getByRole("button", {
      name: "templates.items.parisWeekend.title",
    });
    fireEvent.click(parisCard);
    expect(mockPush).toHaveBeenCalledWith({
      pathname: "/trips/new",
      params: { template: "paris-weekend" },
    });
  });

  it("filters templates by category when chip is tapped", () => {
    render(<TemplateBrowser />);
    const romanticChip = screen.getByRole("button", {
      name: "templates.categories.romantic",
    });
    fireEvent.click(romanticChip);
    // Should show only romantic templates (Paris Weekend, Greek Islands)
    const buttons = screen.getAllByRole("button");
    // 2 romantic templates + 6 category chips = 8
    expect(buttons.length).toBe(8);
  });

  it("filters templates by search query", () => {
    render(<TemplateBrowser />);
    const searchInput = screen.getByLabelText("Search templates");
    fireEvent.change(searchInput, { target: { value: "Japan" } });
    // Only Tokyo Explorer matches (Japan is the country)
    expect(screen.getByText("templates.items.tokyoExplorer.title")).toBeTruthy();
    expect(screen.queryByText("templates.items.parisWeekend.title")).toBeNull();
  });

  it("shows no results message for unmatched search", () => {
    render(<TemplateBrowser />);
    const searchInput = screen.getByLabelText("Search templates");
    fireEvent.change(searchInput, { target: { value: "Antarctica" } });
    expect(screen.getByText("templates.noResults")).toBeTruthy();
  });

  it("renders compact mode without search and categories", () => {
    render(<TemplateBrowser compact />);
    expect(screen.queryByLabelText("Search templates")).toBeNull();
    // All templates should still render as compact cards
    const buttons = screen.getAllByRole("button");
    expect(buttons.length).toBe(10);
  });
});
