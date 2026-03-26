import { Platform } from "react-native";
import type { Trip, Itinerary, ItineraryItem, ItineraryDay } from "@gen/toqui/v1/trip_pb";

function formatICSDate(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, "0");
  const d = String(date.getDate()).padStart(2, "0");
  return `${y}${m}${d}`;
}

function formatICSDateTime(date: Date): string {
  return (
    formatICSDate(date) + "T" +
    String(date.getUTCHours()).padStart(2, "0") +
    String(date.getUTCMinutes()).padStart(2, "0") +
    String(date.getUTCSeconds()).padStart(2, "0") + "Z"
  );
}

function escapeICSText(text: string): string {
  return text.replace(/\\/g, "\\\\").replace(/;/g, "\\;").replace(/,/g, "\\,").replace(/\n/g, "\\n");
}

function foldLine(line: string): string {
  const MAX = 75;
  if (line.length <= MAX) return line;
  const parts: string[] = [line.substring(0, MAX)];
  let remaining = line.substring(MAX);
  while (remaining.length > 0) {
    parts.push(" " + remaining.substring(0, MAX - 1));
    remaining = remaining.substring(MAX - 1);
  }
  return parts.join("\r\n");
}

function simpleHash(str: string): string {
  let hash = 0;
  for (let i = 0; i < str.length; i++) hash = ((hash << 5) - hash + str.charCodeAt(i)) | 0;
  return Math.abs(hash).toString(36);
}

function getDayDate(startDate: string | undefined, dayNumber: number): Date | null {
  if (!startDate) return null;
  try {
    const base = new Date(startDate + "T00:00:00");
    if (isNaN(base.getTime())) return null;
    const date = new Date(base);
    date.setDate(date.getDate() + (dayNumber - 1));
    return date;
  } catch { return null; }
}

function buildVEvent(item: ItineraryItem, day: ItineraryDay, dayDate: Date | null, tripTitle: string, itemIndex: number): string {
  const uid = `${simpleHash(`${tripTitle}-day${day.dayNumber}-item${itemIndex}`)}@toqui.travel`;
  const now = formatICSDateTime(new Date());
  const summary = escapeICSText(item.title);
  const description = item.description
    ? escapeICSText(`Day ${day.dayNumber}: ${item.description}`)
    : escapeICSText(`Day ${day.dayNumber} of ${tripTitle}`);
  const typeTag = item.type ? ` [${item.type}]` : "";

  const lines: string[] = ["BEGIN:VEVENT", foldLine(`UID:${uid}`), foldLine(`DTSTAMP:${now}`)];

  if (item.startTime && item.endTime) {
    const start = new Date(Number(item.startTime.seconds) * 1000);
    const end = new Date(Number(item.endTime.seconds) * 1000);
    lines.push(foldLine(`DTSTART:${formatICSDateTime(start)}`));
    lines.push(foldLine(`DTEND:${formatICSDateTime(end)}`));
  } else if (dayDate) {
    const dateStr = formatICSDate(dayDate);
    const nextDay = new Date(dayDate);
    nextDay.setDate(nextDay.getDate() + 1);
    lines.push(foldLine(`DTSTART;VALUE=DATE:${dateStr}`));
    lines.push(foldLine(`DTEND;VALUE=DATE:${formatICSDate(nextDay)}`));
  } else {
    const placeholder = new Date(2099, 0, 1);
    placeholder.setDate(placeholder.getDate() + (day.dayNumber - 1));
    const nextDay = new Date(placeholder);
    nextDay.setDate(nextDay.getDate() + 1);
    lines.push(foldLine(`DTSTART;VALUE=DATE:${formatICSDate(placeholder)}`));
    lines.push(foldLine(`DTEND;VALUE=DATE:${formatICSDate(nextDay)}`));
  }

  lines.push(foldLine(`SUMMARY:${summary}${escapeICSText(typeTag)}`));
  lines.push(foldLine(`DESCRIPTION:${description}`));
  lines.push("END:VEVENT");
  return lines.join("\r\n");
}

export function buildICSContent(trip: Trip, itinerary: Itinerary): string {
  const events: string[] = [];
  for (const day of itinerary.days) {
    const dayDate = getDayDate(trip.startDate, day.dayNumber);
    for (let i = 0; i < day.items.length; i++) {
      events.push(buildVEvent(day.items[i]!, day, dayDate, trip.title, i));
    }
  }
  const calName = escapeICSText(`${trip.title} - Toqui Itinerary`);
  return [
    "BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Toqui//Toqui Travel//EN",
    "CALSCALE:GREGORIAN", "METHOD:PUBLISH", foldLine(`X-WR-CALNAME:${calName}`),
    ...events, "END:VCALENDAR",
  ].join("\r\n") + "\r\n";
}

export async function exportItineraryICal(trip: Trip, itinerary: Itinerary): Promise<void> {
  const icsContent = buildICSContent(trip, itinerary);
  const filename = `${trip.title.replace(/[^a-zA-Z0-9]/g, "_")}_itinerary.ics`;

  if (Platform.OS === "web") {
    const blob = new Blob([icsContent], { type: "text/calendar;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    link.style.display = "none";
    document.body.appendChild(link);
    link.click();
    setTimeout(() => { document.body.removeChild(link); URL.revokeObjectURL(url); }, 100);
    return;
  }

  // Native: write to temp file and share
  const FileSystem = await import("expo-file-system");
  const Sharing = await import("expo-sharing");
  const fileUri = FileSystem.cacheDirectory + filename;
  await FileSystem.writeAsStringAsync(fileUri, icsContent);
  await Sharing.shareAsync(fileUri, { mimeType: "text/calendar", UTI: "com.apple.ical.ics" });
}
