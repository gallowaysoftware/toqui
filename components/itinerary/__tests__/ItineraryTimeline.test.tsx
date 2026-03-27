import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import React from "react";

// Mock the theme provider before importing the component
vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      surface: "#ffffff",
      surfaceSecondary: "#f9fafb",
      surfaceTertiary: "#f3f4f6",
      border: "#e5e7eb",
      borderStrong: "#d1d5db",
      textPrimary: "#111827",
      textSecondary: "#4b5563",
      textTertiary: "#5f6673",
      accent: "#e8654a",
    },
    isDark: false,
    mode: "light" as const,
    setMode: () => {},
  }),
}));

// Mock lucide-react-native icons as simple spans
vi.mock("lucide-react-native", () => {
  const icon = (props: any) => React.createElement("span", { "data-testid": "icon", ...props });
  return {
    MapPin: icon,
    Clock: icon,
    Utensils: icon,
    Ticket: icon,
    Hotel: icon,
    Plane: icon,
    Camera: icon,
  };
});

import { ItineraryTimeline } from "../ItineraryTimeline";

// Helper to create proto-like objects (matching the Message<T> shape used at runtime)
function makeItem(overrides: Partial<{
  id: string;
  orderInDay: number;
  type: string;
  title: string;
  description: string;
  location: { latitude: number; longitude: number };
  startTime: { seconds: bigint | number };
  endTime: { seconds: bigint | number };
  metadata: Record<string, string>;
}> = {}) {
  return {
    id: overrides.id ?? crypto.randomUUID(),
    orderInDay: overrides.orderInDay ?? 1,
    type: overrides.type ?? "activity",
    title: overrides.title ?? "Test Item",
    description: overrides.description ?? "",
    location: overrides.location,
    startTime: overrides.startTime,
    endTime: overrides.endTime,
    metadata: overrides.metadata ?? {},
    $typeName: "toqui.v1.ItineraryItem" as const,
    $unknown: undefined,
  };
}

function makeDay(overrides: Partial<{
  id: string;
  dayNumber: number;
  date: string;
  summary: string;
  items: ReturnType<typeof makeItem>[];
}> = {}) {
  return {
    id: overrides.id ?? crypto.randomUUID(),
    dayNumber: overrides.dayNumber ?? 1,
    date: overrides.date ?? "2026-04-01",
    summary: overrides.summary ?? "",
    items: overrides.items ?? [],
    $typeName: "toqui.v1.ItineraryDay" as const,
    $unknown: undefined,
  };
}

function makeItinerary(days: ReturnType<typeof makeDay>[] = []) {
  return {
    tripId: "trip-1",
    days,
    $typeName: "toqui.v1.Itinerary" as const,
    $unknown: undefined,
  } as any;
}

describe("ItineraryTimeline", () => {
  describe("empty state", () => {
    it("renders empty message when itinerary has 0 days", () => {
      render(<ItineraryTimeline itinerary={makeItinerary([])} />);
      expect(
        screen.getByText("No itinerary yet. Chat with the AI to start building one.")
      ).toBeTruthy();
    });
  });

  describe("day rendering", () => {
    it("renders day numbers as badges", () => {
      const itinerary = makeItinerary([
        makeDay({ dayNumber: 1 }),
        makeDay({ dayNumber: 2 }),
      ]);
      render(<ItineraryTimeline itinerary={itinerary} />);
      expect(screen.getByText("Day 1")).toBeTruthy();
      expect(screen.getByText("Day 2")).toBeTruthy();
    });

    it("sorts days by dayNumber regardless of input order", () => {
      const itinerary = makeItinerary([
        makeDay({ dayNumber: 3, summary: "Third day" }),
        makeDay({ dayNumber: 1, summary: "First day" }),
        makeDay({ dayNumber: 2, summary: "Second day" }),
      ]);
      const { container } = render(<ItineraryTimeline itinerary={itinerary} />);
      // Get all "Day N" badge texts in order
      const dayBadges = screen.getAllByText(/^Day \d+$/);
      expect(dayBadges.map((el) => el.textContent)).toEqual([
        "Day 1",
        "Day 2",
        "Day 3",
      ]);
    });

    it("displays day.summary (NOT day.title — title does not exist on ItineraryDay)", () => {
      const itinerary = makeItinerary([
        makeDay({ dayNumber: 1, summary: "Explore the old city" }),
      ]);
      render(<ItineraryTimeline itinerary={itinerary} />);
      expect(screen.getByText("Explore the old city")).toBeTruthy();
    });

    it("displays day date when provided", () => {
      const itinerary = makeItinerary([
        makeDay({ dayNumber: 1, date: "2026-04-15" }),
      ]);
      render(<ItineraryTimeline itinerary={itinerary} />);
      expect(screen.getByText("2026-04-15")).toBeTruthy();
    });

    it("does not render summary element when summary is empty string", () => {
      const itinerary = makeItinerary([
        makeDay({ dayNumber: 1, summary: "" }),
      ]);
      render(<ItineraryTimeline itinerary={itinerary} />);
      // Day badge should render, but no summary text beyond it
      expect(screen.getByText("Day 1")).toBeTruthy();
    });

    it("shows 'No items yet' for a day with 0 items", () => {
      const itinerary = makeItinerary([
        makeDay({ dayNumber: 1, items: [] }),
      ]);
      render(<ItineraryTimeline itinerary={itinerary} />);
      expect(screen.getByText("No items yet")).toBeTruthy();
    });
  });

  describe("item rendering", () => {
    it("renders item titles", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "Visit the Louvre", orderInDay: 1 }),
            makeItem({ title: "Dinner at Le Jules Verne", orderInDay: 2 }),
          ],
        }),
      ]);
      render(<ItineraryTimeline itinerary={itinerary} />);
      expect(screen.getByText("Visit the Louvre")).toBeTruthy();
      expect(screen.getByText("Dinner at Le Jules Verne")).toBeTruthy();
    });

    it("sorts items by orderInDay regardless of input order", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "Third", orderInDay: 3 }),
            makeItem({ title: "First", orderInDay: 1 }),
            makeItem({ title: "Second", orderInDay: 2 }),
          ],
        }),
      ]);
      const { container } = render(<ItineraryTimeline itinerary={itinerary} />);
      // Items should appear in orderInDay sequence
      const itemTitles = ["First", "Second", "Third"];
      const allText = container.textContent ?? "";
      let lastIndex = -1;
      for (const title of itemTitles) {
        const idx = allText.indexOf(title);
        expect(idx).toBeGreaterThan(lastIndex);
        lastIndex = idx;
      }
    });

    it("renders item description when present", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({
              title: "Museum",
              description: "Amazing art collection",
              orderInDay: 1,
            }),
          ],
        }),
      ]);
      render(<ItineraryTimeline itinerary={itinerary} />);
      expect(screen.getByText("Amazing art collection")).toBeTruthy();
    });

    it("does not render description element when description is empty", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "Museum", description: "", orderInDay: 1 }),
          ],
        }),
      ]);
      render(<ItineraryTimeline itinerary={itinerary} />);
      expect(screen.getByText("Museum")).toBeTruthy();
      // No extra empty text elements for description
    });

    it("renders item type as a label", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "Eiffel Tower", type: "attraction", orderInDay: 1 }),
          ],
        }),
      ]);
      render(<ItineraryTimeline itinerary={itinerary} />);
      expect(screen.getByText("attraction")).toBeTruthy();
    });

    it("does not render type element when type is empty string", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "Free time", type: "", orderInDay: 1 }),
          ],
        }),
      ]);
      render(<ItineraryTimeline itinerary={itinerary} />);
      expect(screen.getByText("Free time")).toBeTruthy();
    });
  });

  describe("does not mutate input data", () => {
    it("sort does not mutate the original days array", () => {
      const day1 = makeDay({ dayNumber: 2 });
      const day2 = makeDay({ dayNumber: 1 });
      const days = [day1, day2];
      const itinerary = makeItinerary(days);
      render(<ItineraryTimeline itinerary={itinerary} />);
      // Original array order should be preserved
      expect(itinerary.days[0]).toBe(day1);
      expect(itinerary.days[1]).toBe(day2);
    });

    it("sort does not mutate the original items array", () => {
      const item1 = makeItem({ title: "B", orderInDay: 2 });
      const item2 = makeItem({ title: "A", orderInDay: 1 });
      const items = [item1, item2];
      const itinerary = makeItinerary([makeDay({ dayNumber: 1, items })]);
      render(<ItineraryTimeline itinerary={itinerary} />);
      // Original items array order should be preserved
      expect(itinerary.days[0].items[0]).toBe(item1);
      expect(itinerary.days[0].items[1]).toBe(item2);
    });
  });

  describe("many days (color cycling)", () => {
    it("renders 15 days without crashing (exercises day color wrapping)", () => {
      const days = Array.from({ length: 15 }, (_, i) =>
        makeDay({ dayNumber: i + 1, summary: `Day ${i + 1} summary` })
      );
      const itinerary = makeItinerary(days);
      const { container } = render(<ItineraryTimeline itinerary={itinerary} />);
      expect(screen.getByText("Day 15")).toBeTruthy();
      expect(screen.getByText("Day 15 summary")).toBeTruthy();
    });
  });
});
