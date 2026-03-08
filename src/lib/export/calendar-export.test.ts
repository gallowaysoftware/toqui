import { describe, it, expect, vi, beforeEach } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  TripSchema,
  ItinerarySchema,
  ItineraryDaySchema,
  ItineraryItemSchema,
  TripStatus,
} from "@/gen/toqui/v1/trip_pb";
import {
  buildICSContent,
  exportItineraryICal,
  formatICSDate,
  formatICSDateTime,
  escapeICSText,
  getDayDate,
  foldLine,
} from "./calendar-export";

function makeTrip(overrides: Partial<{ title: string; startDate: string; endDate: string }> = {}) {
  return create(TripSchema, {
    id: "trip-1",
    title: overrides.title ?? "Tokyo Adventure",
    status: TripStatus.PLANNING,
    startDate: overrides.startDate ?? "2026-04-01",
    endDate: overrides.endDate ?? "2026-04-14",
  });
}

function makeItinerary(
  days: Array<{
    dayNumber: number;
    items: Array<{ title: string; type?: string; description?: string }>;
  }>,
) {
  return create(ItinerarySchema, {
    tripId: "trip-1",
    days: days.map((d) =>
      create(ItineraryDaySchema, {
        dayNumber: d.dayNumber,
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

describe("calendar-export", () => {
  describe("formatICSDate", () => {
    it("formats date correctly", () => {
      const date = new Date(2026, 3, 1); // April 1, 2026
      expect(formatICSDate(date)).toBe("20260401");
    });

    it("pads single-digit month and day", () => {
      const date = new Date(2026, 0, 5); // Jan 5
      expect(formatICSDate(date)).toBe("20260105");
    });
  });

  describe("formatICSDateTime", () => {
    it("formats datetime in UTC", () => {
      const date = new Date("2026-04-01T12:30:00Z");
      expect(formatICSDateTime(date)).toBe("20260401T123000Z");
    });
  });

  describe("escapeICSText", () => {
    it("escapes backslashes", () => {
      expect(escapeICSText("path\\to\\file")).toBe("path\\\\to\\\\file");
    });

    it("escapes semicolons", () => {
      expect(escapeICSText("hello;world")).toBe("hello\\;world");
    });

    it("escapes commas", () => {
      expect(escapeICSText("hello,world")).toBe("hello\\,world");
    });

    it("escapes newlines", () => {
      expect(escapeICSText("line1\nline2")).toBe("line1\\nline2");
    });
  });

  describe("foldLine", () => {
    it("returns short lines unchanged", () => {
      const line = "SUMMARY:Short summary";
      expect(foldLine(line)).toBe(line);
    });

    it("folds long lines at 75 characters", () => {
      const line = "DESCRIPTION:" + "a".repeat(100);
      const folded = foldLine(line);
      const lines = folded.split("\r\n");
      expect(lines.length).toBeGreaterThan(1);
      expect(lines[0].length).toBe(75);
      // Continuation lines start with space
      for (let i = 1; i < lines.length; i++) {
        expect(lines[i][0]).toBe(" ");
      }
    });
  });

  describe("getDayDate", () => {
    it("returns correct date for day 1", () => {
      const date = getDayDate("2026-04-01", 1);
      expect(date).not.toBeNull();
      expect(date!.getDate()).toBe(1);
      expect(date!.getMonth()).toBe(3); // April = 3
    });

    it("returns correct date for day 3", () => {
      const date = getDayDate("2026-04-01", 3);
      expect(date).not.toBeNull();
      expect(date!.getDate()).toBe(3);
    });

    it("returns null when no start date", () => {
      expect(getDayDate(undefined, 1)).toBeNull();
      expect(getDayDate("", 1)).toBeNull();
    });

    it("returns null for invalid date", () => {
      expect(getDayDate("not-a-date", 1)).toBeNull();
    });
  });

  describe("buildICSContent", () => {
    it("generates valid ICS format", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "Visit Temple", type: "sightseeing" }],
        },
      ]);

      const ics = buildICSContent(trip, itinerary);

      expect(ics).toContain("BEGIN:VCALENDAR");
      expect(ics).toContain("END:VCALENDAR");
      expect(ics).toContain("VERSION:2.0");
      expect(ics).toContain("PRODID:-//Toqui//Toqui Travel//EN");
    });

    it("creates VEVENT for each itinerary item", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([
        {
          dayNumber: 1,
          items: [
            { title: "Visit Temple", type: "sightseeing" },
            { title: "Lunch at Ramen Shop", type: "food" },
          ],
        },
        {
          dayNumber: 2,
          items: [{ title: "Shopping in Shibuya", type: "shopping" }],
        },
      ]);

      const ics = buildICSContent(trip, itinerary);

      const eventCount = (ics.match(/BEGIN:VEVENT/g) ?? []).length;
      expect(eventCount).toBe(3);
    });

    it("includes item title in SUMMARY", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "Visit Temple", type: "sightseeing" }],
        },
      ]);

      const ics = buildICSContent(trip, itinerary);
      expect(ics).toContain("SUMMARY:Visit Temple");
    });

    it("uses all-day events when trip has start_date", () => {
      const trip = makeTrip({ startDate: "2026-04-01" });
      const itinerary = makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "Day 1 Activity" }],
        },
      ]);

      const ics = buildICSContent(trip, itinerary);
      expect(ics).toContain("DTSTART;VALUE=DATE:20260401");
    });

    it("maps day number to correct date from start_date", () => {
      const trip = makeTrip({ startDate: "2026-04-01" });
      const itinerary = makeItinerary([
        {
          dayNumber: 3,
          items: [{ title: "Day 3 Activity" }],
        },
      ]);

      const ics = buildICSContent(trip, itinerary);
      // Day 3 = April 1 + 2 = April 3
      expect(ics).toContain("DTSTART;VALUE=DATE:20260403");
    });

    it("includes calendar name", () => {
      const trip = makeTrip({ title: "Tokyo Trip" });
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Arrive" }] },
      ]);

      const ics = buildICSContent(trip, itinerary);
      expect(ics).toContain("X-WR-CALNAME:");
      expect(ics).toContain("Tokyo Trip");
    });

    it("includes description with day number", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([
        {
          dayNumber: 2,
          items: [{ title: "Temple Visit", description: "Beautiful temple" }],
        },
      ]);

      const ics = buildICSContent(trip, itinerary);
      expect(ics).toContain("DESCRIPTION:Day 2: Beautiful temple");
    });

    it("handles empty itinerary", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([]);

      const ics = buildICSContent(trip, itinerary);

      expect(ics).toContain("BEGIN:VCALENDAR");
      expect(ics).toContain("END:VCALENDAR");
      expect(ics).not.toContain("BEGIN:VEVENT");
    });

    it("ends with CRLF", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Test" }] },
      ]);

      const ics = buildICSContent(trip, itinerary);
      expect(ics.endsWith("\r\n")).toBe(true);
    });
  });

  describe("exportItineraryICal", () => {
    beforeEach(() => {
      // URL.createObjectURL/revokeObjectURL don't exist in jsdom, so define them
      if (!URL.createObjectURL) {
        URL.createObjectURL = vi.fn();
      }
      if (!URL.revokeObjectURL) {
        URL.revokeObjectURL = vi.fn();
      }
      vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:mock-url");
      vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
      vi.spyOn(document.body, "appendChild").mockImplementation(() => document.createElement("a"));
      vi.spyOn(document.body, "removeChild").mockImplementation(() => document.createElement("a"));
    });

    it("is callable and creates a download", () => {
      const trip = makeTrip();
      const itinerary = makeItinerary([
        { dayNumber: 1, items: [{ title: "Test Activity" }] },
      ]);

      const clickSpy = vi.fn();
      vi.spyOn(document, "createElement").mockReturnValue({
        href: "",
        download: "",
        style: { display: "" },
        click: clickSpy,
      } as unknown as HTMLAnchorElement);

      exportItineraryICal(trip, itinerary);

      expect(clickSpy).toHaveBeenCalled();
    });
  });
});
