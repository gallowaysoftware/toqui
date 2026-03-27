import { describe, it, expect } from "vitest";
import { buildICSContent } from "../calendar-export";

// Minimal type stubs matching the protobuf shapes used by the export code.
// We cast to `any` to avoid pulling in the full protobuf runtime.
function makeTrip(overrides: Record<string, unknown> = {}): any {
  return {
    id: "trip-1",
    userId: "user-1",
    title: "Test Trip",
    description: "",
    status: 1,
    startDate: "2025-07-01",
    endDate: "2025-07-03",
    destinationCountry: "JP",
    themes: [],
    ...overrides,
  };
}

function makeItinerary(
  days: Array<{
    dayNumber: number;
    items: Array<Record<string, unknown>>;
    date?: string;
    summary?: string;
  }>
): any {
  return {
    tripId: "trip-1",
    days: days.map((d) => ({
      id: `day-${d.dayNumber}`,
      dayNumber: d.dayNumber,
      date: d.date ?? "",
      summary: d.summary ?? "",
      items: d.items.map((item, i) => ({
        id: `item-${i}`,
        orderInDay: i,
        type: "",
        title: "",
        description: "",
        ...item,
      })),
    })),
  };
}

function makeTimestamp(isoString: string): { seconds: bigint } {
  return { seconds: BigInt(Math.floor(new Date(isoString).getTime() / 1000)) };
}

// ---------------------------------------------------------------------------
// Structural validity
// ---------------------------------------------------------------------------
describe("buildICSContent - structure", () => {
  it("produces a valid VCALENDAR wrapper with required properties", () => {
    const ics = buildICSContent(makeTrip(), makeItinerary([]));
    expect(ics).toContain("BEGIN:VCALENDAR");
    expect(ics).toContain("END:VCALENDAR");
    expect(ics).toContain("VERSION:2.0");
    expect(ics).toContain("PRODID:-//Toqui//Toqui Travel//EN");
    expect(ics).toContain("CALSCALE:GREGORIAN");
    expect(ics).toContain("METHOD:PUBLISH");
  });

  it("uses CRLF line endings throughout (RFC 5545 requirement)", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "Walk" }] },
      ])
    );
    // Every line boundary should be \r\n. No bare \n should appear outside folded content.
    const lines = ics.split("\r\n");
    // After splitting on \r\n, no element should contain a bare \n
    for (const line of lines) {
      expect(line).not.toContain("\n");
    }
  });

  it("ends with a trailing CRLF", () => {
    const ics = buildICSContent(makeTrip(), makeItinerary([]));
    expect(ics.endsWith("\r\n")).toBe(true);
  });

  it("wraps each event in BEGIN:VEVENT / END:VEVENT", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "Shrine visit" }, { title: "Ramen lunch" }],
        },
      ])
    );
    const begins = (ics.match(/BEGIN:VEVENT/g) || []).length;
    const ends = (ics.match(/END:VEVENT/g) || []).length;
    expect(begins).toBe(2);
    expect(ends).toBe(2);
  });
});

// ---------------------------------------------------------------------------
// Empty / edge case itineraries
// ---------------------------------------------------------------------------
describe("buildICSContent - empty itinerary", () => {
  it("produces valid VCALENDAR with zero events for empty days array", () => {
    const ics = buildICSContent(makeTrip(), makeItinerary([]));
    expect(ics).toContain("BEGIN:VCALENDAR");
    expect(ics).toContain("END:VCALENDAR");
    expect(ics).not.toContain("BEGIN:VEVENT");
  });

  it("produces valid VCALENDAR when a day has zero items", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([{ dayNumber: 1, items: [] }])
    );
    expect(ics).not.toContain("BEGIN:VEVENT");
  });
});

// ---------------------------------------------------------------------------
// Date handling
// ---------------------------------------------------------------------------
describe("buildICSContent - date handling", () => {
  it("generates all-day events (VALUE=DATE) when items have no startTime/endTime", () => {
    const ics = buildICSContent(
      makeTrip({ startDate: "2025-07-01" }),
      makeItinerary([{ dayNumber: 1, items: [{ title: "Free day" }] }])
    );
    expect(ics).toContain("DTSTART;VALUE=DATE:20250701");
    expect(ics).toContain("DTEND;VALUE=DATE:20250702");
  });

  it("computes day 2 date correctly from trip start date", () => {
    const ics = buildICSContent(
      makeTrip({ startDate: "2025-07-01" }),
      makeItinerary([{ dayNumber: 2, items: [{ title: "Day two" }] }])
    );
    expect(ics).toContain("DTSTART;VALUE=DATE:20250702");
    expect(ics).toContain("DTEND;VALUE=DATE:20250703");
  });

  it("uses placeholder year 2099 when trip has no start date", () => {
    const ics = buildICSContent(
      makeTrip({ startDate: "" }),
      makeItinerary([{ dayNumber: 1, items: [{ title: "Unscheduled" }] }])
    );
    expect(ics).toContain("DTSTART;VALUE=DATE:20990101");
    expect(ics).toContain("DTEND;VALUE=DATE:20990102");
  });

  it("uses placeholder for day 3 when trip has no start date", () => {
    const ics = buildICSContent(
      makeTrip({ startDate: "" }),
      makeItinerary([{ dayNumber: 3, items: [{ title: "Day 3" }] }])
    );
    // 2099-01-01 + 2 days = 2099-01-03
    expect(ics).toContain("DTSTART;VALUE=DATE:20990103");
    expect(ics).toContain("DTEND;VALUE=DATE:20990104");
  });

  it("generates timed events with UTC timestamps when startTime/endTime are present", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [
            {
              title: "Dinner",
              startTime: makeTimestamp("2025-07-01T18:00:00Z"),
              endTime: makeTimestamp("2025-07-01T20:00:00Z"),
            },
          ],
        },
      ])
    );
    expect(ics).toContain("DTSTART:20250701T180000Z");
    expect(ics).toContain("DTEND:20250701T200000Z");
    // Timed events should NOT use VALUE=DATE
    expect(ics).not.toContain("VALUE=DATE");
  });

  it("handles startTime with seconds precision", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [
            {
              title: "Meeting",
              startTime: makeTimestamp("2025-07-01T09:30:45Z"),
              endTime: makeTimestamp("2025-07-01T10:15:30Z"),
            },
          ],
        },
      ])
    );
    expect(ics).toContain("DTSTART:20250701T093045Z");
    expect(ics).toContain("DTEND:20250701T101530Z");
  });

  it("handles month/year boundaries (Dec 31 to Jan 1)", () => {
    const ics = buildICSContent(
      makeTrip({ startDate: "2025-12-31" }),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "NYE" }] },
        { dayNumber: 2, items: [{ title: "New Year" }] },
      ])
    );
    expect(ics).toContain("DTSTART;VALUE=DATE:20251231");
    expect(ics).toContain("DTSTART;VALUE=DATE:20260101");
  });
});

// ---------------------------------------------------------------------------
// UID generation (determinism and uniqueness)
// ---------------------------------------------------------------------------
describe("buildICSContent - UID generation", () => {
  it("produces deterministic UIDs for the same trip/day/item", () => {
    const trip = makeTrip();
    const itin = makeItinerary([
      { dayNumber: 1, items: [{ title: "Visit" }] },
    ]);
    const ics1 = buildICSContent(trip, itin);
    const ics2 = buildICSContent(trip, itin);
    const extractUID = (s: string) => {
      const match = s.match(/UID:([^\r\n]+)/);
      return match?.[1];
    };
    expect(extractUID(ics1)).toBe(extractUID(ics2));
  });

  it("produces different UIDs for different items in the same day", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "Item A" }, { title: "Item B" }],
        },
      ])
    );
    const uids = [...ics.matchAll(/UID:([^\r\n]+)/g)].map((m) => m[1]);
    expect(uids.length).toBe(2);
    expect(uids[0]).not.toBe(uids[1]);
  });

  it("produces UIDs with @toqui.travel domain", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([{ dayNumber: 1, items: [{ title: "X" }] }])
    );
    const uids = [...ics.matchAll(/UID:([^\r\n]+)/g)].map((m) => m[1]);
    for (const uid of uids) {
      expect(uid).toMatch(/@toqui\.travel$/);
    }
  });

  it("produces different UIDs for items in different days", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "Same title" }] },
        { dayNumber: 2, items: [{ title: "Same title" }] },
      ])
    );
    const uids = [...ics.matchAll(/UID:([^\r\n]+)/g)].map((m) => m[1]);
    expect(uids.length).toBe(2);
    expect(uids[0]).not.toBe(uids[1]);
  });
});

// ---------------------------------------------------------------------------
// ICS text escaping (RFC 5545 compliance)
// ---------------------------------------------------------------------------
describe("buildICSContent - text escaping", () => {
  it("escapes backslashes in event summary", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "path\\to\\place" }] },
      ])
    );
    expect(ics).toContain("SUMMARY:path\\\\to\\\\place");
  });

  it("escapes semicolons in event summary", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "Option A; Option B" }] },
      ])
    );
    expect(ics).toContain("SUMMARY:Option A\\; Option B");
  });

  it("escapes commas in event summary", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "Tokyo, Japan" }] },
      ])
    );
    expect(ics).toContain("SUMMARY:Tokyo\\, Japan");
  });

  it("escapes newlines in event description", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "Visit", description: "Line1\nLine2" }],
        },
      ])
    );
    // The description should contain escaped newline \\n, not literal \n
    expect(ics).toContain("Line1\\nLine2");
  });

  it("escapes multiple special characters in a single string", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "A\\B;C,D" }],
        },
      ])
    );
    expect(ics).toContain("SUMMARY:A\\\\B\\;C\\,D");
  });

  it("includes escaped type tag in summary", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "Dinner", type: "food" }] },
      ])
    );
    // type tag is appended as " [food]" with brackets escaped for commas/semis if needed
    expect(ics).toContain("SUMMARY:Dinner [food]");
  });
});

// ---------------------------------------------------------------------------
// Line folding (RFC 5545: max 75 octets per line)
// ---------------------------------------------------------------------------
describe("buildICSContent - line folding", () => {
  it("folds lines longer than 75 characters", () => {
    const longTitle = "A".repeat(100);
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([{ dayNumber: 1, items: [{ title: longTitle }] }])
    );
    // After folding, continuation lines start with CRLF + space
    const rawLines = ics.split("\r\n");
    // Find the SUMMARY line(s)
    const summaryIdx = rawLines.findIndex((l) => l.startsWith("SUMMARY:"));
    expect(summaryIdx).toBeGreaterThan(-1);
    // The SUMMARY line itself should be <= 75 chars
    expect(rawLines[summaryIdx]!.length).toBeLessThanOrEqual(75);
    // The next line should be a continuation (starts with space)
    expect(rawLines[summaryIdx + 1]!.startsWith(" ")).toBe(true);
  });

  it("does not fold lines that are exactly 75 characters", () => {
    // "SUMMARY:" = 8 chars, so title of 67 chars = exactly 75
    const title = "B".repeat(67);
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([{ dayNumber: 1, items: [{ title }] }])
    );
    const rawLines = ics.split("\r\n");
    const summaryIdx = rawLines.findIndex((l) => l.startsWith("SUMMARY:"));
    expect(rawLines[summaryIdx]!.length).toBe(75);
    // Next line should NOT be a continuation
    expect(rawLines[summaryIdx + 1]!.startsWith(" ")).toBe(false);
  });

  it("no unfolded line exceeds 75 octets", () => {
    const longTitle = "C".repeat(200);
    const longDesc = "D".repeat(200);
    const ics = buildICSContent(
      makeTrip({ title: "E".repeat(100) }),
      makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: longTitle, description: longDesc }],
        },
      ])
    );
    const rawLines = ics.split("\r\n");
    for (const line of rawLines) {
      expect(line.length).toBeLessThanOrEqual(75);
    }
  });
});

// ---------------------------------------------------------------------------
// Calendar name and description content
// ---------------------------------------------------------------------------
describe("buildICSContent - content fields", () => {
  it("includes trip title in X-WR-CALNAME", () => {
    const ics = buildICSContent(
      makeTrip({ title: "Japan Adventure" }),
      makeItinerary([])
    );
    expect(ics).toContain("X-WR-CALNAME:Japan Adventure - Toqui Itinerary");
  });

  it("includes day number in event description", () => {
    const ics = buildICSContent(
      makeTrip({ title: "My Trip" }),
      makeItinerary([
        { dayNumber: 3, items: [{ title: "Hike" }] },
      ])
    );
    expect(ics).toContain("Day 3 of My Trip");
  });

  it("uses item description when provided", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "Temple", description: "Beautiful ancient temple" }],
        },
      ])
    );
    expect(ics).toContain("Day 1: Beautiful ancient temple");
  });

  it("falls back to trip title description when item has no description", () => {
    const ics = buildICSContent(
      makeTrip({ title: "Tokyo Trip" }),
      makeItinerary([
        { dayNumber: 2, items: [{ title: "Walk" }] },
      ])
    );
    expect(ics).toContain("DESCRIPTION:Day 2 of Tokyo Trip");
  });
});

// ---------------------------------------------------------------------------
// DTSTAMP is present (required by RFC 5545)
// ---------------------------------------------------------------------------
describe("buildICSContent - DTSTAMP", () => {
  it("includes DTSTAMP in every VEVENT", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "A" }, { title: "B" }],
        },
      ])
    );
    const stamps = (ics.match(/DTSTAMP:/g) || []).length;
    expect(stamps).toBe(2);
  });

  it("DTSTAMP is in UTC format (ends with Z)", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([{ dayNumber: 1, items: [{ title: "X" }] }])
    );
    const match = ics.match(/DTSTAMP:(\S+)/);
    expect(match).not.toBeNull();
    expect(match![1]).toMatch(/^\d{8}T\d{6}Z$/);
  });
});

// ---------------------------------------------------------------------------
// Unicode / special character handling
// ---------------------------------------------------------------------------
describe("buildICSContent - unicode and special chars", () => {
  it("preserves unicode characters in summary", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "Visit Shibuya 渋谷" }] },
      ])
    );
    expect(ics).toContain("渋谷");
  });

  it("preserves emoji in title", () => {
    const ics = buildICSContent(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "Beach day 🏖️" }] },
      ])
    );
    expect(ics).toContain("🏖️");
  });
});
