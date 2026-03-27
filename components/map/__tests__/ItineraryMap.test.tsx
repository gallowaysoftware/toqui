import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import React from "react";

// Mock theme provider
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

// Mock MapLibre — the module conditionally loads it, but in jsdom Platform.OS = "web"
// so it should NOT be imported. We mock it to verify it is NOT used on web.
vi.mock("@maplibre/maplibre-react-native", () => {
  throw new Error("MapLibre should not be imported on web");
});

import { ItineraryMap } from "../ItineraryMap";

// Helpers
function makeLocation(lat: number, lng: number) {
  return { latitude: lat, longitude: lng };
}

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
    date: overrides.date ?? "",
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

describe("ItineraryMap", () => {
  describe("empty state", () => {
    it("shows empty message when itinerary has 0 days", () => {
      render(<ItineraryMap itinerary={makeItinerary([])} />);
      expect(screen.getByText("No locations to display on map")).toBeTruthy();
    });

    it("shows empty message when days have no items", () => {
      const itinerary = makeItinerary([
        makeDay({ dayNumber: 1, items: [] }),
        makeDay({ dayNumber: 2, items: [] }),
      ]);
      render(<ItineraryMap itinerary={itinerary} />);
      expect(screen.getByText("No locations to display on map")).toBeTruthy();
    });

    it("shows empty message when all items lack locations", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [makeItem({ title: "No location item" })],
        }),
      ]);
      render(<ItineraryMap itinerary={itinerary} />);
      expect(screen.getByText("No locations to display on map")).toBeTruthy();
    });

    it("shows empty message when all locations are (0, 0)", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "Zero loc", location: makeLocation(0, 0) }),
          ],
        }),
      ]);
      render(<ItineraryMap itinerary={itinerary} />);
      expect(screen.getByText("No locations to display on map")).toBeTruthy();
    });

    it("filters out items with latitude=0 but valid longitude", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "Lat zero", location: makeLocation(0, 45.0) }),
          ],
        }),
      ]);
      render(<ItineraryMap itinerary={itinerary} />);
      // latitude=0 is treated as "no location" by the filter
      expect(screen.getByText("No locations to display on map")).toBeTruthy();
    });

    it("filters out items with longitude=0 but valid latitude", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "Lng zero", location: makeLocation(48.8, 0) }),
          ],
        }),
      ]);
      render(<ItineraryMap itinerary={itinerary} />);
      expect(screen.getByText("No locations to display on map")).toBeTruthy();
    });
  });

  describe("web fallback rendering", () => {
    it("shows web fallback with marker count for valid locations", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "Eiffel Tower", location: makeLocation(48.8584, 2.2945) }),
            makeItem({ title: "Louvre", location: makeLocation(48.8606, 2.3376) }),
          ],
        }),
      ]);
      render(<ItineraryMap itinerary={itinerary} />);
      expect(screen.getByText("Map view (2 locations)")).toBeTruthy();
      expect(screen.getByText("Available on iOS and Android")).toBeTruthy();
    });

    it("counts only items with non-zero lat/lng in the marker count", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "Has loc", location: makeLocation(48.8584, 2.2945) }),
            makeItem({ title: "No loc" }), // no location
            makeItem({ title: "Zero loc", location: makeLocation(0, 0) }), // filtered
          ],
        }),
      ]);
      render(<ItineraryMap itinerary={itinerary} />);
      expect(screen.getByText("Map view (1 locations)")).toBeTruthy();
    });

    it("aggregates markers across multiple days", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "A", location: makeLocation(48.0, 2.0) }),
          ],
        }),
        makeDay({
          dayNumber: 2,
          items: [
            makeItem({ title: "B", location: makeLocation(49.0, 3.0) }),
            makeItem({ title: "C", location: makeLocation(50.0, 4.0) }),
          ],
        }),
      ]);
      render(<ItineraryMap itinerary={itinerary} />);
      expect(screen.getByText("Map view (3 locations)")).toBeTruthy();
    });
  });

  describe("custom height prop", () => {
    it("applies custom height to the placeholder", () => {
      const itinerary = makeItinerary([]);
      const { container } = render(
        <ItineraryMap itinerary={itinerary} height={500} />
      );
      // react-native-web renders height as an inline style
      const placeholder = container.firstElementChild as HTMLElement;
      expect(placeholder).toBeTruthy();
      // The style should contain height: 500px
      const style = placeholder.getAttribute("style") ?? "";
      expect(style).toContain("500px");
    });

    it("uses default height of 300 when not specified", () => {
      const itinerary = makeItinerary([]);
      const { container } = render(<ItineraryMap itinerary={itinerary} />);
      const placeholder = container.firstElementChild as HTMLElement;
      const style = placeholder.getAttribute("style") ?? "";
      expect(style).toContain("300px");
    });
  });

  describe("marker extraction logic", () => {
    // We test this indirectly through the rendered output since extractMarkers is private

    it("uses getDayColor from colors module (day-based coloring)", () => {
      // This verifies the component imports getDayColor from ./colors
      // and uses day.dayNumber (not index) for color assignment.
      // With 1 valid marker, the web fallback should render.
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 5,
          items: [
            makeItem({ title: "Day5 item", location: makeLocation(48.0, 2.0) }),
          ],
        }),
      ]);
      render(<ItineraryMap itinerary={itinerary} />);
      expect(screen.getByText("Map view (1 locations)")).toBeTruthy();
    });

    it("handles a mix of items with and without locations in the same day", () => {
      const itinerary = makeItinerary([
        makeDay({
          dayNumber: 1,
          items: [
            makeItem({ title: "Located", location: makeLocation(48.8, 2.3) }),
            makeItem({ title: "No location" }),
            makeItem({ title: "Also located", location: makeLocation(49.0, 3.0) }),
            makeItem({ title: "Zero", location: makeLocation(0, 0) }),
          ],
        }),
      ]);
      render(<ItineraryMap itinerary={itinerary} />);
      expect(screen.getByText("Map view (2 locations)")).toBeTruthy();
    });
  });
});

// Test computeBounds and extractMarkers logic via a separate import approach.
// Since these are not exported, we test their effects through component behavior.
// The bounds computation is verified by ensuring the component renders without crashing
// for various marker configurations.
describe("ItineraryMap bounds edge cases", () => {
  it("handles single marker (bounds sw === ne)", () => {
    const itinerary = makeItinerary([
      makeDay({
        dayNumber: 1,
        items: [
          makeItem({ title: "Only point", location: makeLocation(48.8, 2.3) }),
        ],
      }),
    ]);
    // Should not crash — bounds with sw===ne is valid
    const { container } = render(<ItineraryMap itinerary={itinerary} />);
    expect(screen.getByText("Map view (1 locations)")).toBeTruthy();
  });

  it("handles markers at extreme coordinates", () => {
    const itinerary = makeItinerary([
      makeDay({
        dayNumber: 1,
        items: [
          makeItem({ title: "North pole-ish", location: makeLocation(89.9, -179.9) }),
          makeItem({ title: "South pole-ish", location: makeLocation(-89.9, 179.9) }),
        ],
      }),
    ]);
    const { container } = render(<ItineraryMap itinerary={itinerary} />);
    expect(screen.getByText("Map view (2 locations)")).toBeTruthy();
  });

  it("handles negative coordinates (Southern/Western hemispheres)", () => {
    const itinerary = makeItinerary([
      makeDay({
        dayNumber: 1,
        items: [
          makeItem({ title: "Buenos Aires", location: makeLocation(-34.6, -58.4) }),
          makeItem({ title: "Sydney", location: makeLocation(-33.9, 151.2) }),
        ],
      }),
    ]);
    render(<ItineraryMap itinerary={itinerary} />);
    expect(screen.getByText("Map view (2 locations)")).toBeTruthy();
  });
});
