import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ItineraryMap } from "./ItineraryMap";
import { getDayColor, DAY_COLORS } from "./colors";
import { create } from "@bufbuild/protobuf";
import { ItinerarySchema, ItineraryDaySchema, ItineraryItemSchema } from "@/gen/toqui/v1/trip_pb";
import { LatLngSchema } from "@/gen/toqui/v1/common_pb";

// Mock maplibre-gl since it requires WebGL/DOM APIs not available in jsdom
vi.mock("maplibre-gl", () => {
  const MockMap = vi.fn().mockImplementation(() => ({
    addControl: vi.fn(),
    on: vi.fn(),
    remove: vi.fn(),
    fitBounds: vi.fn(),
    flyTo: vi.fn(),
  }));

  const MockMarker = vi.fn().mockImplementation(() => ({
    setLngLat: vi.fn().mockReturnThis(),
    setPopup: vi.fn().mockReturnThis(),
    addTo: vi.fn().mockReturnThis(),
    remove: vi.fn(),
  }));

  const MockPopup = vi.fn().mockImplementation(() => ({
    setHTML: vi.fn().mockReturnThis(),
  }));

  const MockNavigationControl = vi.fn();

  const MockLngLatBounds = vi.fn().mockImplementation(() => ({
    extend: vi.fn(),
  }));

  return {
    default: {
      Map: MockMap,
      Marker: MockMarker,
      Popup: MockPopup,
      NavigationControl: MockNavigationControl,
      LngLatBounds: MockLngLatBounds,
    },
    Map: MockMap,
    Marker: MockMarker,
    Popup: MockPopup,
    NavigationControl: MockNavigationControl,
    LngLatBounds: MockLngLatBounds,
  };
});

// Mock the CSS import
vi.mock("maplibre-gl/dist/maplibre-gl.css", () => ({}));

// Mock lucide-react
vi.mock("lucide-react", () => ({
  MapPin: (props: Record<string, unknown>) => (
    <svg data-testid="map-pin-icon" {...(props as React.SVGAttributes<SVGElement>)} />
  ),
}));

function makeItineraryWithLocations() {
  return create(ItinerarySchema, {
    tripId: "trip-1",
    days: [
      create(ItineraryDaySchema, {
        id: "day-1",
        dayNumber: 1,
        date: "2026-04-01",
        summary: "Exploring Tokyo",
        items: [
          create(ItineraryItemSchema, {
            id: "item-1",
            orderInDay: 1,
            type: "attraction",
            title: "Senso-ji Temple",
            description: "Historic Buddhist temple in Asakusa",
            location: create(LatLngSchema, {
              latitude: 35.7148,
              longitude: 139.7967,
            }),
          }),
          create(ItineraryItemSchema, {
            id: "item-2",
            orderInDay: 2,
            type: "restaurant",
            title: "Sushi Dai",
            description: "Famous sushi at Tsukiji Market",
            location: create(LatLngSchema, {
              latitude: 35.6654,
              longitude: 139.7707,
            }),
          }),
        ],
      }),
      create(ItineraryDaySchema, {
        id: "day-2",
        dayNumber: 2,
        date: "2026-04-02",
        summary: "Kyoto day trip",
        items: [
          create(ItineraryItemSchema, {
            id: "item-3",
            orderInDay: 1,
            type: "attraction",
            title: "Fushimi Inari Shrine",
            description: "Thousands of vermillion torii gates",
            location: create(LatLngSchema, {
              latitude: 34.9671,
              longitude: 135.7727,
            }),
          }),
        ],
      }),
    ],
  });
}

function makeItineraryWithoutLocations() {
  return create(ItinerarySchema, {
    tripId: "trip-2",
    days: [
      create(ItineraryDaySchema, {
        id: "day-1",
        dayNumber: 1,
        date: "2026-04-01",
        summary: "Planning day",
        items: [
          create(ItineraryItemSchema, {
            id: "item-1",
            orderInDay: 1,
            type: "activity",
            title: "Research restaurants",
            description: "Find good places to eat",
            // no location
          }),
        ],
      }),
    ],
  });
}

describe("getDayColor", () => {
  it("returns blue for day 1", () => {
    expect(getDayColor(1)).toBe("#3B82F6");
  });

  it("returns green for day 2", () => {
    expect(getDayColor(2)).toBe("#10B981");
  });

  it("returns amber for day 3", () => {
    expect(getDayColor(3)).toBe("#F59E0B");
  });

  it("returns red for day 4", () => {
    expect(getDayColor(4)).toBe("#EF4444");
  });

  it("returns violet for day 5", () => {
    expect(getDayColor(5)).toBe("#8B5CF6");
  });

  it("returns pink for day 6", () => {
    expect(getDayColor(6)).toBe("#EC4899");
  });

  it("wraps around after exhausting the palette", () => {
    // 10 colors, so day 11 wraps to first color
    expect(getDayColor(DAY_COLORS.length + 1)).toBe(DAY_COLORS[0]);
  });

  it("has at least 8 distinct colors", () => {
    expect(DAY_COLORS.length).toBeGreaterThanOrEqual(8);
  });

  it("contains only valid hex color strings", () => {
    for (const color of DAY_COLORS) {
      expect(color).toMatch(/^#[0-9A-Fa-f]{6}$/);
    }
  });

  it("has no duplicate colors", () => {
    const unique = new Set(DAY_COLORS);
    expect(unique.size).toBe(DAY_COLORS.length);
  });
});

describe("ItineraryMap", () => {
  it("shows loading state when isLoading is true", () => {
    render(<ItineraryMap isLoading={true} />);
    expect(screen.getByTestId("map-loading")).toBeInTheDocument();
    expect(screen.getByText("Loading map...")).toBeInTheDocument();
  });

  it("shows empty state when itinerary has no locatable items", () => {
    render(<ItineraryMap itinerary={makeItineraryWithoutLocations()} />);
    expect(screen.getByTestId("map-empty")).toBeInTheDocument();
    expect(screen.getByText("No locations yet")).toBeInTheDocument();
    expect(
      screen.getByText("Chat with the AI to add places to your itinerary"),
    ).toBeInTheDocument();
  });

  it("shows empty state when itinerary is undefined", () => {
    render(<ItineraryMap />);
    expect(screen.getByTestId("map-empty")).toBeInTheDocument();
  });

  it("renders map container when itinerary has locatable items", () => {
    render(<ItineraryMap itinerary={makeItineraryWithLocations()} />);
    expect(screen.getByTestId("map-container")).toBeInTheDocument();
  });

  it("shows day legend for days with locatable items", () => {
    render(<ItineraryMap itinerary={makeItineraryWithLocations()} />);
    expect(screen.getByText("Day 1")).toBeInTheDocument();
    expect(screen.getByText("Day 2")).toBeInTheDocument();
  });

  it("applies custom className", () => {
    const { container } = render(
      <ItineraryMap itinerary={makeItineraryWithLocations()} className="h-[400px]" />,
    );
    const wrapper = container.firstElementChild;
    expect(wrapper?.className).toContain("h-[400px]");
  });
});
