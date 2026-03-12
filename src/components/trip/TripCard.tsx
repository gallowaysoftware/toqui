import Link from "next/link";
import { Calendar } from "lucide-react";
import type { Trip } from "@/gen/toqui/v1/trip_pb";
import { TripStatus } from "@/gen/toqui/v1/trip_pb";

const statusLabel: Record<number, string> = {
  [TripStatus.PLANNING]: "planning",
  [TripStatus.ACTIVE]: "active",
  [TripStatus.COMPLETED]: "completed",
};

const statusColors: Record<string, string> = {
  planning: "bg-[var(--color-status-planning-bg)] text-[var(--color-status-planning-text)]",
  active: "bg-[var(--color-status-active-bg)] text-[var(--color-status-active-text)]",
  completed: "bg-[var(--color-status-completed-bg)] text-[var(--color-status-completed-text)]",
};

function formatDate(dateStr: string): string {
  try {
    const date = new Date(dateStr + "T00:00:00");
    return date.toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
      year: "numeric",
    });
  } catch {
    return dateStr;
  }
}

export function TripCard({ trip }: { trip: Trip }) {
  const label = statusLabel[trip.status] || "planning";

  return (
    <Link
      href={`/trips/${trip.id}`}
      className="bg-[var(--color-surface)] rounded-xl p-5 border border-[var(--color-border)] hover:shadow-md dark:hover:shadow-black/25 hover:border-[var(--color-border-strong)] transition-all block focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
    >
      <div className="flex items-start justify-between mb-2">
        <h3 className="font-semibold text-[var(--color-text-primary)]">{trip.title}</h3>
        <span className={`text-xs px-2 py-1 rounded-full font-medium ${statusColors[label]}`}>
          {label}
        </span>
      </div>

      {trip.description && (
        <p className="text-sm text-[var(--color-text-secondary)] mb-3 line-clamp-2">
          {trip.description}
        </p>
      )}

      {trip.startDate && (
        <div className="flex items-center gap-1.5 text-xs text-[var(--color-text-tertiary)]">
          <Calendar size={12} aria-hidden="true" />
          <span>
            {formatDate(trip.startDate)}
            {trip.endDate && ` - ${formatDate(trip.endDate)}`}
          </span>
        </div>
      )}
    </Link>
  );
}
