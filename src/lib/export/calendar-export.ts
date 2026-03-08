import type { Trip, Itinerary, ItineraryItem, ItineraryDay } from "@/gen/toqui/v1/trip_pb";

/**
 * Format a Date as an ICS date string: YYYYMMDD
 */
function formatICSDate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}${month}${day}`;
}

/**
 * Format a Date as an ICS datetime string: YYYYMMDDTHHmmssZ
 */
function formatICSDateTime(date: Date): string {
  return (
    formatICSDate(date) +
    "T" +
    String(date.getUTCHours()).padStart(2, "0") +
    String(date.getUTCMinutes()).padStart(2, "0") +
    String(date.getUTCSeconds()).padStart(2, "0") +
    "Z"
  );
}

/**
 * Escape text for ICS format (RFC 5545).
 * Special characters that must be escaped: backslash, semicolon, comma, newlines.
 */
function escapeICSText(text: string): string {
  return text
    .replace(/\\/g, "\\\\")
    .replace(/;/g, "\\;")
    .replace(/,/g, "\\,")
    .replace(/\n/g, "\\n");
}

/**
 * Fold long lines per RFC 5545 (max 75 octets per line).
 * Continuation lines start with a space.
 */
function foldLine(line: string): string {
  const MAX = 75;
  if (line.length <= MAX) return line;

  const parts: string[] = [];
  parts.push(line.substring(0, MAX));
  let remaining = line.substring(MAX);

  while (remaining.length > 0) {
    // Continuation lines have a leading space, so max content is 74
    parts.push(" " + remaining.substring(0, MAX - 1));
    remaining = remaining.substring(MAX - 1);
  }

  return parts.join("\r\n");
}

/**
 * Generate a unique UID for an ICS event.
 */
function generateUID(tripTitle: string, dayNumber: number, itemIndex: number): string {
  const hash = simpleHash(`${tripTitle}-day${dayNumber}-item${itemIndex}`);
  return `${hash}@toqui.travel`;
}

/**
 * Simple string hash for UID generation.
 */
function simpleHash(str: string): string {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const chr = str.charCodeAt(i);
    hash = ((hash << 5) - hash + chr) | 0;
  }
  return Math.abs(hash).toString(36);
}

/**
 * Compute the actual date for a day number given the trip start date.
 * If no start date, returns null.
 */
function getDayDate(startDate: string | undefined, dayNumber: number): Date | null {
  if (!startDate) return null;
  try {
    const base = new Date(startDate + "T00:00:00");
    if (isNaN(base.getTime())) return null;
    const date = new Date(base);
    date.setDate(date.getDate() + (dayNumber - 1));
    return date;
  } catch {
    return null;
  }
}

/**
 * Build an ICS VEVENT for a single itinerary item.
 */
function buildVEvent(
  item: ItineraryItem,
  day: ItineraryDay,
  dayDate: Date | null,
  tripTitle: string,
  itemIndex: number,
): string {
  const uid = generateUID(tripTitle, day.dayNumber, itemIndex);
  const now = formatICSDateTime(new Date());

  const summary = escapeICSText(item.title);
  const description = item.description
    ? escapeICSText(`Day ${day.dayNumber}: ${item.description}`)
    : escapeICSText(`Day ${day.dayNumber} of ${tripTitle}`);

  const typeTag = item.type ? ` [${item.type}]` : "";

  const lines: string[] = ["BEGIN:VEVENT", foldLine(`UID:${uid}`), foldLine(`DTSTAMP:${now}`)];

  if (item.startTime && item.endTime) {
    // Use explicit times from the itinerary item
    const start = new Date(Number(item.startTime.seconds) * 1000);
    const end = new Date(Number(item.endTime.seconds) * 1000);
    lines.push(foldLine(`DTSTART:${formatICSDateTime(start)}`));
    lines.push(foldLine(`DTEND:${formatICSDateTime(end)}`));
  } else if (dayDate) {
    // All-day event based on trip start date + day number
    const dateStr = formatICSDate(dayDate);
    const nextDay = new Date(dayDate);
    nextDay.setDate(nextDay.getDate() + 1);
    lines.push(foldLine(`DTSTART;VALUE=DATE:${dateStr}`));
    lines.push(foldLine(`DTEND;VALUE=DATE:${formatICSDate(nextDay)}`));
  } else {
    // No date info — use relative day label. Use a far-future placeholder date.
    // This is a fallback; items will appear in calendar but not on correct dates.
    const placeholder = new Date(2099, 0, 1);
    placeholder.setDate(placeholder.getDate() + (day.dayNumber - 1));
    const dateStr = formatICSDate(placeholder);
    const nextDay = new Date(placeholder);
    nextDay.setDate(nextDay.getDate() + 1);
    lines.push(foldLine(`DTSTART;VALUE=DATE:${dateStr}`));
    lines.push(foldLine(`DTEND;VALUE=DATE:${formatICSDate(nextDay)}`));
  }

  lines.push(foldLine(`SUMMARY:${summary}${escapeICSText(typeTag)}`));
  lines.push(foldLine(`DESCRIPTION:${description}`));
  lines.push("END:VEVENT");

  return lines.join("\r\n");
}

/**
 * Build a complete ICS calendar string from trip and itinerary data.
 */
export function buildICSContent(trip: Trip, itinerary: Itinerary): string {
  const events: string[] = [];

  for (const day of itinerary.days) {
    const dayDate = getDayDate(trip.startDate, day.dayNumber);

    for (let i = 0; i < day.items.length; i++) {
      events.push(buildVEvent(day.items[i], day, dayDate, trip.title, i));
    }
  }

  const calName = escapeICSText(`${trip.title} - Toqui Itinerary`);

  const lines = [
    "BEGIN:VCALENDAR",
    "VERSION:2.0",
    "PRODID:-//Toqui//Toqui Travel//EN",
    "CALSCALE:GREGORIAN",
    "METHOD:PUBLISH",
    foldLine(`X-WR-CALNAME:${calName}`),
    ...events,
    "END:VCALENDAR",
  ];

  return lines.join("\r\n") + "\r\n";
}

/**
 * Export itinerary as an .ics file download.
 * Each itinerary item becomes a calendar event.
 * Day number is mapped to actual date if trip has a start_date.
 */
export function exportItineraryICal(trip: Trip, itinerary: Itinerary): void {
  const icsContent = buildICSContent(trip, itinerary);

  const blob = new Blob([icsContent], { type: "text/calendar;charset=utf-8" });
  const url = URL.createObjectURL(blob);

  const link = document.createElement("a");
  link.href = url;
  link.download = `${trip.title.replace(/[^a-zA-Z0-9]/g, "_")}_itinerary.ics`;
  link.style.display = "none";
  document.body.appendChild(link);
  link.click();

  // Clean up
  setTimeout(() => {
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }, 100);
}

// For testing
export { formatICSDate, formatICSDateTime, escapeICSText, getDayDate, foldLine };
