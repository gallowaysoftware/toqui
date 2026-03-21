"use client";

import {
  Clock,
  MapPin,
  Utensils,
  Bus,
  Hotel,
  Camera,
  ShoppingBag,
  Moon,
  Compass,
  CalendarDays,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { Itinerary, ItineraryDay, ItineraryItem } from "@/gen/toqui/v1/trip_pb";
import { getDayColor } from "@/components/map/colors";

const itemTypeIcons: Record<string, LucideIcon> = {
  activity: Compass,
  meal: Utensils,
  transport: Bus,
  accommodation: Hotel,
  sightseeing: Camera,
  shopping: ShoppingBag,
  nightlife: Moon,
};

const itemTypeLabels: Record<string, string> = {
  activity: "Activity",
  meal: "Meal",
  transport: "Transport",
  accommodation: "Accommodation",
  sightseeing: "Sightseeing",
  shopping: "Shopping",
  nightlife: "Nightlife",
};

function formatTime(ts: { seconds: bigint } | undefined): string {
  if (!ts) return "";
  const d = new Date(Number(ts.seconds) * 1000);
  return d.toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit" });
}

interface ItineraryTimelineProps {
  itinerary: Itinerary | undefined;
  isLoading: boolean;
}

export function ItineraryTimeline({ itinerary, isLoading }: ItineraryTimelineProps) {
  if (isLoading) {
    return (
      <div className="text-center py-8" role="status" aria-busy="true">
        <div className="inline-block animate-spin h-5 w-5 border-2 border-[var(--color-text-tertiary)] border-t-transparent rounded-full mb-2" />
        <p className="text-sm text-[var(--color-text-tertiary)]">Loading itinerary...</p>
      </div>
    );
  }

  if (!itinerary || itinerary.days.length === 0) {
    return (
      <div className="text-center py-8">
        <CalendarDays
          size={32}
          className="mx-auto text-[var(--color-text-tertiary)] mb-2"
          aria-hidden="true"
        />
        <p className="text-sm text-[var(--color-text-tertiary)]">
          No itinerary yet. Chat with Toqui to start building your plan.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {itinerary.days.map((day) => (
        <DaySection key={day.id || day.dayNumber} day={day} />
      ))}
    </div>
  );
}

function DaySection({ day }: { day: ItineraryDay }) {
  const color = getDayColor(day.dayNumber);

  return (
    <div>
      <div className="flex items-center gap-3 mb-3">
        <div
          className="w-8 h-8 rounded-full flex items-center justify-center text-white text-sm font-bold flex-shrink-0"
          style={{ backgroundColor: color }}
        >
          {day.dayNumber}
        </div>
        <div>
          <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">
            Day {day.dayNumber}
            {day.date && (
              <span className="text-[var(--color-text-tertiary)] font-normal ml-2">
                {new Date(day.date + "T00:00:00").toLocaleDateString("en-US", {
                  weekday: "short",
                  month: "short",
                  day: "numeric",
                })}
              </span>
            )}
          </h3>
          {day.summary && (
            <p className="text-xs text-[var(--color-text-secondary)]">{day.summary}</p>
          )}
        </div>
      </div>

      <div className="ml-4 border-l-2 border-[var(--color-border)] pl-6 space-y-3">
        {day.items.map((item, idx) => (
          <ItemCard key={item.id || idx} item={item} dayColor={color} />
        ))}
      </div>
    </div>
  );
}

function ItemCard({ item, dayColor }: { item: ItineraryItem; dayColor: string }) {
  const Icon = itemTypeIcons[item.type] ?? Compass;
  const typeLabel = itemTypeLabels[item.type] ?? item.type;
  const startTime = formatTime(item.startTime);
  const endTime = formatTime(item.endTime);
  const timeStr = startTime && endTime ? `${startTime} – ${endTime}` : startTime || endTime;

  return (
    <div className="bg-[var(--color-surface)] rounded-lg border border-[var(--color-border)] p-3">
      <div className="flex items-start gap-3">
        <div
          className="w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0 opacity-80"
          style={{ backgroundColor: dayColor + "20", color: dayColor }}
        >
          <Icon size={16} aria-hidden="true" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-0.5">
            <span className="text-sm font-medium text-[var(--color-text-primary)] truncate">
              {item.title}
            </span>
            <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-[var(--color-surface-tertiary)] text-[var(--color-text-tertiary)] flex-shrink-0">
              {typeLabel}
            </span>
          </div>
          {item.description && (
            <p className="text-xs text-[var(--color-text-secondary)] line-clamp-2">
              {item.description}
            </p>
          )}
          <div className="flex items-center gap-3 mt-1 text-[10px] text-[var(--color-text-tertiary)]">
            {timeStr && (
              <span className="flex items-center gap-1">
                <Clock size={10} aria-hidden="true" />
                {timeStr}
              </span>
            )}
            {item.location && (
              <span className="flex items-center gap-1">
                <MapPin size={10} aria-hidden="true" />
                {item.metadata["address"] ?? `${item.location.latitude.toFixed(4)}, ${item.location.longitude.toFixed(4)}`}
              </span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
