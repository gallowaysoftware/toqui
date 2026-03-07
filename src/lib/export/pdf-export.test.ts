import { describe, it, expect, vi, beforeEach } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  TripSchema,
  ItinerarySchema,
  ItineraryDaySchema,
  ItineraryItemSchema,
  TripStatus,
} from "@/gen/toqui/v1/trip_pb";
import { exportItineraryPDF, buildPrintHTML, getTypeLabel } from "./pdf-export";

function makeTrip(overrides: Partial<{ title: string; description: string; startDate: string; endDate: string }> = {}) {
  return create(TripSchema, {
    id: "trip-1",
    title: overrides.title ?? "Tokyo Adventure",
    description: overrides.description ?? "Two weeks exploring Japan",
    status: TripStatus.PLANNING,
    startDate: overrides.startDate ?? "2026-04-01",
    endDate: overrides.endDate ?? "2026-04-14",
  });
}

function makeItinerary(
  days: Array<{
    dayNumber: number;
    summary?: string;
    items: Array<{ title: string; type?: string; description?: string }>;
  }>,
) {
  return create(ItinerarySchema, {
    tripId: "trip-1",
    days: days.map((d) =>
      create(ItineraryDaySchema, {
        dayNumber: d.dayNumber,
        summary: d.summary ?? "",
        items: d.items.map((item, idx) =>
          create(ItineraryItemSchema, {
            id: `item-${d.dayNumber}-${idx}`,
            orderInDay: idx + 1,
            title: item.title,
            type: item.type ?? "",
            description: item.description ?? "",
          }),
        ),
      }),
    ),
  });
}

describe("pdf-export", () => {
  describe("getTypeLabel", () => {
    it("returns known type labels", () => {
      expect(getTypeLabel("activity")).toBe("Activity");
      expect(getTypeLabel("food")).toBe("Food");
      expect(getTypeLabel("sightseeing")).toBe("Sightseeing");
    });

    it("capitalizes unknown types", () => {
      expect(getTypeLabel("custom")).toBe("Custom");
    });

    it("returns empty string for undefined", () => {
      expect(getTypeLabel(undefined)).toBe("");
    });

    it("is case-insensitive", () => {
      expect(getTypeLabel("FOOD")).toBe("Food");
      expect(getTypeLabel("Activity")).toBe("Activity");
    });
  });

  describe("buildPrintHTML", () => {
    it("includes trip title", () => {
      const trip = makeTrip({ title: "Paris Getaway" });
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Arrive" }] },
      ]);

      const html = buildPrintHTML(trip, itinerary);
      expect(html).toContain("Paris Getaway");
    });

    it("includes trip description", () => {
      const trip = makeTrip({ description: "A wonderful trip" });
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Arrive" }] },
      ]);

      const html = buildPrintHTML(trip, itinerary);
      expect(html).toContain("A wonderful trip");
    });

    it("includes date range", () => {
      const trip = makeTrip({
        startDate: "2026-04-01",
        endDate: "2026-04-14",
      });
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Arrive" }] },
      ]);

      const html = buildPrintHTML(trip, itinerary);
      expect(html).toContain("2026-04-01 to 2026-04-14");
    });

    it("includes day headers", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Morning Walk" }] },
        { dayNumber: 2, items: [{ title: "Museum Visit" }] },
      ]);

      const html = buildPrintHTML(trip, itinerary);
      expect(html).toContain("Day 1");
      expect(html).toContain("Day 2");
    });

    it("includes item titles and descriptions", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([
        {
          dayNumber: 1,
          items: [
            { title: "Visit Temple", type: "sightseeing", description: "Beautiful ancient temple" },
          ],
        },
      ]);

      const html = buildPrintHTML(trip, itinerary);
      expect(html).toContain("Visit Temple");
      expect(html).toContain("Beautiful ancient temple");
      expect(html).toContain("Sightseeing");
    });

    it("includes Toqui branding", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Test" }] },
      ]);

      const html = buildPrintHTML(trip, itinerary);
      expect(html).toContain("TOQUI");
    });

    it("includes print media query", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Test" }] },
      ]);

      const html = buildPrintHTML(trip, itinerary);
      expect(html).toContain("@media print");
    });

    it("escapes HTML entities in content", () => {
      const trip = makeTrip({ title: "Trip <script>alert(1)</script>" });
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Item with <b>bold</b>" }] },
      ]);

      const html = buildPrintHTML(trip, itinerary);
      expect(html).not.toContain("<script>");
      expect(html).toContain("&lt;script&gt;");
    });

    it("includes day summary when present", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([
        {
          dayNumber: 1,
          summary: "Exploring the city center",
          items: [{ title: "Walk around" }],
        },
      ]);

      const html = buildPrintHTML(trip, itinerary);
      expect(html).toContain("Exploring the city center");
    });
  });

  describe("exportItineraryPDF", () => {
    beforeEach(() => {
      vi.restoreAllMocks();
    });

    it("is callable as a function", () => {
      expect(typeof exportItineraryPDF).toBe("function");
    });

    it("opens a new window for printing", () => {
      const mockWrite = vi.fn();
      const mockClose = vi.fn();
      const mockFocus = vi.fn();
      const mockPrint = vi.fn();

      const mockWindow = {
        document: {
          open: vi.fn(),
          write: mockWrite,
          close: mockClose,
        },
        focus: mockFocus,
        print: mockPrint,
        onload: null as (() => void) | null,
      };

      vi.spyOn(window, "open").mockReturnValue(mockWindow as unknown as Window);

      const trip = makeTrip();
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Test" }] },
      ]);

      exportItineraryPDF(trip, itinerary);

      expect(window.open).toHaveBeenCalledWith("", "_blank");
      expect(mockWrite).toHaveBeenCalled();
      expect(mockClose).toHaveBeenCalled();

      // The written HTML should contain the trip title
      const writtenHTML = mockWrite.mock.calls[0][0];
      expect(writtenHTML).toContain("Tokyo Adventure");
    });

    it("falls back to iframe when popup is blocked", () => {
      vi.spyOn(window, "open").mockReturnValue(null);

      const mockIframe = {
        style: {} as CSSStyleDeclaration,
        contentDocument: {
          open: vi.fn(),
          write: vi.fn(),
          close: vi.fn(),
        },
        contentWindow: {
          focus: vi.fn(),
          print: vi.fn(),
        },
      };

      vi.spyOn(document, "createElement").mockReturnValue(mockIframe as unknown as HTMLIFrameElement);
      vi.spyOn(document.body, "appendChild").mockImplementation(() => mockIframe as unknown as HTMLIFrameElement);
      vi.spyOn(document.body, "removeChild").mockImplementation(() => mockIframe as unknown as HTMLIFrameElement);

      const trip = makeTrip();
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Test" }] },
      ]);

      exportItineraryPDF(trip, itinerary);

      expect(document.body.appendChild).toHaveBeenCalled();
      expect(mockIframe.contentDocument!.write).toHaveBeenCalled();
    });
  });
});
