import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock react-native Platform before importing the module under test.
vi.mock("react-native", () => ({
  Platform: { OS: "web" },
}));

// Mock the colors module since it's a UI concern outside our test scope.
vi.mock("@/components/map/colors", () => ({
  getDayColor: (dayNumber: number) => `#color${dayNumber}`,
}));

import { exportItineraryPDF } from "../pdf-export";

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------
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

/** Capture the HTML written by exportItineraryPDF via a fake window.open. */
async function captureHTML(trip: any, itinerary: any): Promise<string> {
  let capturedHTML = "";
  const fakeDoc = {
    write: (html: string) => { capturedHTML = html; },
    close: vi.fn(),
  };
  const fakeWin = { document: fakeDoc, print: vi.fn() };
  vi.stubGlobal("window", { open: () => fakeWin });

  await exportItineraryPDF(trip, itinerary);
  return capturedHTML;
}

beforeEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// escapeHtml - XSS prevention (tested through the HTML output)
// ---------------------------------------------------------------------------
describe("PDF export - escapeHtml XSS prevention", () => {
  it("escapes <script> tags in trip title", async () => {
    const html = await captureHTML(
      makeTrip({ title: '<script>alert(1)</script>' }),
      makeItinerary([])
    );
    expect(html).not.toContain("<script>");
    expect(html).not.toContain("</script>");
    expect(html).toContain("&lt;script&gt;alert(1)&lt;/script&gt;");
  });

  it("escapes <img onerror> XSS in item title", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: '<img src=x onerror="alert(1)">' }],
        },
      ])
    );
    expect(html).not.toContain("<img");
    expect(html).toContain("&lt;img");
  });

  it("escapes double quotes to prevent attribute breakout", async () => {
    const html = await captureHTML(
      makeTrip({ title: 'Trip " onmouseover="alert(1)' }),
      makeItinerary([])
    );
    expect(html).not.toContain('" onmouseover=');
    expect(html).toContain("&quot;");
  });

  it("escapes single quotes to prevent attribute breakout", async () => {
    const html = await captureHTML(
      makeTrip({ description: "Trip ' onmouseover='alert(1)" }),
      makeItinerary([])
    );
    expect(html).not.toContain("' onmouseover='");
    expect(html).toContain("&#x27;");
  });

  it("escapes ampersands to prevent entity injection", async () => {
    const html = await captureHTML(
      makeTrip({ title: "AT&T Conference" }),
      makeItinerary([])
    );
    expect(html).toContain("AT&amp;T Conference");
  });

  it("escapes HTML in item descriptions", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [
            {
              title: "Safe",
              description: '<div onclick="steal()">Click me</div>',
            },
          ],
        },
      ])
    );
    expect(html).not.toContain('<div onclick=');
    expect(html).toContain("&lt;div onclick=");
  });

  it("escapes HTML in day summary", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          summary: "<b>Bold</b>",
          items: [],
        },
      ])
    );
    expect(html).not.toContain("<b>Bold</b>");
    expect(html).toContain("&lt;b&gt;Bold&lt;/b&gt;");
  });

  it("escapes HTML in day date field", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          date: '2025-07-01"><script>alert(1)</script>',
          items: [],
        },
      ])
    );
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
  });

  it("escapes HTML in item type", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "X", type: '<script>alert("xss")</script>' }],
        },
      ])
    );
    expect(html).not.toContain("<script>");
  });

  it("escapes HTML in destination country", async () => {
    const html = await captureHTML(
      makeTrip({ destinationCountry: '<img src=x onerror=alert(1)>' }),
      makeItinerary([])
    );
    expect(html).not.toContain("<img");
  });

  it("handles multiple levels of escaping correctly (no double-escaping on first pass)", async () => {
    const html = await captureHTML(
      makeTrip({ title: "A&B<C>D" }),
      makeItinerary([])
    );
    // Should escape & first, then < and > -- no &amp;lt; double-escape
    expect(html).toContain("A&amp;B&lt;C&gt;D");
  });
});

// ---------------------------------------------------------------------------
// HTML structure
// ---------------------------------------------------------------------------
describe("PDF export - HTML structure", () => {
  it("produces a valid HTML document", async () => {
    const html = await captureHTML(makeTrip(), makeItinerary([]));
    expect(html).toContain("<!DOCTYPE html>");
    expect(html).toContain("<html>");
    expect(html).toContain("</html>");
    expect(html).toContain('<meta charset="utf-8"/>');
  });

  it("includes trip title in h1", async () => {
    const html = await captureHTML(
      makeTrip({ title: "Japan 2025" }),
      makeItinerary([])
    );
    expect(html).toContain("<h1>Japan 2025</h1>");
  });

  it("displays start and end dates", async () => {
    const html = await captureHTML(
      makeTrip({ startDate: "2025-07-01", endDate: "2025-07-10" }),
      makeItinerary([])
    );
    expect(html).toContain("2025-07-01");
    expect(html).toContain("2025-07-10");
  });

  it("includes description paragraph when provided", async () => {
    const html = await captureHTML(
      makeTrip({ description: "A wonderful trip to Japan" }),
      makeItinerary([])
    );
    expect(html).toContain("A wonderful trip to Japan");
  });

  it("shows no-items message for empty itinerary", async () => {
    const html = await captureHTML(makeTrip(), makeItinerary([]));
    expect(html).toContain("No itinerary items yet.");
  });

  it("includes Toqui branding footer", async () => {
    const html = await captureHTML(makeTrip(), makeItinerary([]));
    expect(html).toContain("Generated by Toqui");
    expect(html).toContain("toqui.travel");
  });
});

// ---------------------------------------------------------------------------
// Itinerary content rendering
// ---------------------------------------------------------------------------
describe("PDF export - itinerary rendering", () => {
  it("renders day numbers", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "Walk" }] },
        { dayNumber: 2, items: [{ title: "Hike" }] },
      ])
    );
    expect(html).toContain("Day 1");
    expect(html).toContain("Day 2");
  });

  it("renders item titles", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "Visit Fushimi Inari" }],
        },
      ])
    );
    expect(html).toContain("Visit Fushimi Inari");
  });

  it("renders item descriptions", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [{ title: "Temple", description: "Morning visit recommended" }],
        },
      ])
    );
    expect(html).toContain("Morning visit recommended");
  });

  it("renders item type when present", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "Sushi", type: "food" }] },
      ])
    );
    expect(html).toContain("food");
  });

  it("renders day date when present", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, date: "July 1, 2025", items: [] },
      ])
    );
    expect(html).toContain("July 1, 2025");
  });

  it("renders day summary when present", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, summary: "Explore downtown Tokyo", items: [] },
      ])
    );
    expect(html).toContain("Explore downtown Tokyo");
  });

  it("sorts days by dayNumber", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        { dayNumber: 3, items: [{ title: "Third" }] },
        { dayNumber: 1, items: [{ title: "First" }] },
        { dayNumber: 2, items: [{ title: "Second" }] },
      ])
    );
    const firstIdx = html.indexOf("First");
    const secondIdx = html.indexOf("Second");
    const thirdIdx = html.indexOf("Third");
    expect(firstIdx).toBeLessThan(secondIdx);
    expect(secondIdx).toBeLessThan(thirdIdx);
  });

  it("sorts items within a day by orderInDay", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [
            { title: "Lunch", orderInDay: 2 },
            { title: "Breakfast", orderInDay: 1 },
            { title: "Dinner", orderInDay: 3 },
          ],
        },
      ])
    );
    const breakfastIdx = html.indexOf("Breakfast");
    const lunchIdx = html.indexOf("Lunch");
    const dinnerIdx = html.indexOf("Dinner");
    expect(breakfastIdx).toBeLessThan(lunchIdx);
    expect(lunchIdx).toBeLessThan(dinnerIdx);
  });

  it("shows 'No items' placeholder for a day with empty items array", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([{ dayNumber: 1, items: [] }])
    );
    expect(html).toContain("No items");
  });
});

// ---------------------------------------------------------------------------
// Time rendering
// ---------------------------------------------------------------------------
describe("PDF export - time display", () => {
  it("renders time string for items with startTime", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        {
          dayNumber: 1,
          items: [
            {
              title: "Dinner",
              startTime: makeTimestamp("2025-07-01T18:00:00Z"),
            },
          ],
        },
      ])
    );
    // toLocaleTimeString output varies by locale, but should contain some time representation
    // The key thing is that a time span element is present (not empty)
    expect(html).toMatch(/<span[^>]*color:#888[^>]*>[^<]+<\/span>/);
  });

  it("does not render time element when startTime is absent", async () => {
    const html = await captureHTML(
      makeTrip(),
      makeItinerary([
        { dayNumber: 1, items: [{ title: "Free time" }] },
      ])
    );
    // The time span should not appear at all (empty string for time means no span)
    // Check that there's no span with the time styling next to the title
    const titleSection = html.substring(
      html.indexOf("Free time"),
      html.indexOf("Free time") + 200
    );
    expect(titleSection).not.toMatch(/<span[^>]*color:#888/);
  });
});

// ---------------------------------------------------------------------------
// window.open returns null (popup blocked)
// ---------------------------------------------------------------------------
describe("PDF export - popup blocked", () => {
  it("does not throw when window.open returns null", async () => {
    vi.stubGlobal("window", { open: () => null });
    // Should not throw
    await expect(
      exportItineraryPDF(makeTrip(), makeItinerary([]))
    ).resolves.toBeUndefined();
  });
});
