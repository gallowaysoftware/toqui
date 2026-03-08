"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  Calendar,
  MapPin,
  Utensils,
  Landmark,
  ShoppingBag,
  Compass,
  Bed,
  Plane,
  Clock,
} from "lucide-react";
import type {
  SharedTripResponse,
  SharedItineraryDay,
  SharedItineraryItem,
} from "@/lib/shared-trip-types";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

function formatDate(dateStr?: string): string {
  if (!dateStr) return "";
  try {
    const date = new Date(dateStr + "T00:00:00");
    return date.toLocaleDateString("en-US", {
      weekday: "short",
      month: "short",
      day: "numeric",
      year: "numeric",
    });
  } catch {
    return dateStr;
  }
}

function ItemTypeIcon({ type }: { type?: string }) {
  const iconProps = { size: 16, className: "text-[var(--color-accent)]", "aria-hidden": true as const };
  const key = type?.toLowerCase() ?? "";
  switch (key) {
    case "activity": return <Compass {...iconProps} />;
    case "food": case "restaurant": case "dining": return <Utensils {...iconProps} />;
    case "sightseeing": case "attraction": case "museum": return <Landmark {...iconProps} />;
    case "shopping": return <ShoppingBag {...iconProps} />;
    case "accommodation": case "hotel": return <Bed {...iconProps} />;
    case "transport": case "flight": case "transit": return <Plane {...iconProps} />;
    default: return <Clock {...iconProps} />;
  }
}

function ItineraryItemCard({ item }: { item: SharedItineraryItem }) {
  const typeLabel = item.type
    ? item.type.charAt(0).toUpperCase() + item.type.slice(1)
    : null;

  return (
    <div className="flex gap-3 py-3 px-4 bg-[var(--color-surface)] rounded-lg border border-[var(--color-border)]">
      <div className="flex-shrink-0 mt-0.5">
        <div className="w-8 h-8 rounded-full bg-[var(--color-accent-soft)] flex items-center justify-center">
          <ItemTypeIcon type={item.type} />
        </div>
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <h4 className="font-medium text-[var(--color-text-primary)] text-sm">
            {item.title}
          </h4>
          {typeLabel && (
            <span className="text-xs text-[var(--color-text-tertiary)] bg-[var(--color-surface-tertiary)] px-2 py-0.5 rounded-full">
              {typeLabel}
            </span>
          )}
        </div>
        {item.description && (
          <p className="text-sm text-[var(--color-text-secondary)] mt-1 leading-relaxed">
            {item.description}
          </p>
        )}
      </div>
    </div>
  );
}

function DaySection({ day }: { day: SharedItineraryDay }) {
  return (
    <section className="space-y-3">
      <h3 className="text-base font-semibold text-[var(--color-text-primary)] flex items-center gap-2">
        <span className="inline-flex items-center justify-center w-7 h-7 rounded-full bg-[var(--color-accent)] text-white text-xs font-bold">
          {day.day_number}
        </span>
        Day {day.day_number}
      </h3>
      <div className="space-y-2 pl-2">
        {day.items.map((item, idx) => (
          <ItineraryItemCard key={idx} item={item} />
        ))}
        {day.items.length === 0 && (
          <p className="text-sm text-[var(--color-text-tertiary)] italic pl-4">
            No activities planned for this day
          </p>
        )}
      </div>
    </section>
  );
}

function LoadingState() {
  return (
    <div
      className="min-h-screen bg-[var(--color-surface-secondary)] flex items-center justify-center"
      aria-busy="true"
      role="status"
    >
      <div className="text-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--color-accent)] mx-auto mb-3" />
        <p className="text-sm text-[var(--color-text-secondary)]">Loading trip...</p>
        <span className="sr-only">Loading shared trip details...</span>
      </div>
    </div>
  );
}

function NotFoundState() {
  return (
    <div className="min-h-screen bg-[var(--color-surface-secondary)] flex items-center justify-center">
      <div className="text-center px-6 max-w-md">
        <div className="w-16 h-16 rounded-full bg-[var(--color-surface-tertiary)] flex items-center justify-center mx-auto mb-4">
          <MapPin size={28} className="text-[var(--color-text-tertiary)]" aria-hidden="true" />
        </div>
        <h1 className="text-xl font-semibold text-[var(--color-text-primary)] mb-2">
          Trip not found
        </h1>
        <p className="text-sm text-[var(--color-text-secondary)] mb-6">
          This shared trip link may have expired or been removed by the owner.
        </p>
        <Link
          href="/waitlist"
          className="inline-flex items-center gap-2 bg-[var(--color-accent)] text-white px-5 py-2.5 rounded-lg hover:bg-[var(--color-accent-hover)] transition-colors text-sm font-medium"
        >
          Plan your own trip with Toqui
        </Link>
      </div>
    </div>
  );
}

function ErrorState() {
  return (
    <div className="min-h-screen bg-[var(--color-surface-secondary)] flex items-center justify-center">
      <div className="text-center px-6 max-w-md">
        <h1 className="text-xl font-semibold text-[var(--color-text-primary)] mb-2">
          Something went wrong
        </h1>
        <p className="text-sm text-[var(--color-text-secondary)] mb-6">
          We could not load this shared trip. Please try again later.
        </p>
        <Link
          href="/waitlist"
          className="inline-flex items-center gap-2 bg-[var(--color-accent)] text-white px-5 py-2.5 rounded-lg hover:bg-[var(--color-accent-hover)] transition-colors text-sm font-medium"
        >
          Plan your own trip with Toqui
        </Link>
      </div>
    </div>
  );
}

export default function SharedTripPage() {
  const { token } = useParams<{ token: string }>();
  const [data, setData] = useState<SharedTripResponse | null>(null);
  const [status, setStatus] = useState<"loading" | "success" | "not_found" | "error">("loading");

  useEffect(() => {
    if (!token) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- guard clause, no async
      setStatus("not_found");
      return;
    }

    let cancelled = false;

    async function fetchSharedTrip() {
      try {
        const res = await fetch(`${API_URL}/shared/${encodeURIComponent(token)}`);
        if (cancelled) return;

        if (res.status === 404) {
          setStatus("not_found");
          return;
        }

        if (!res.ok) {
          setStatus("error");
          return;
        }

        const json: SharedTripResponse = await res.json();
        if (cancelled) return;

        setData(json);
        setStatus("success");
      } catch {
        if (!cancelled) {
          setStatus("error");
        }
      }
    }

    void fetchSharedTrip();
    return () => {
      cancelled = true;
    };
  }, [token]);

  if (status === "loading") return <LoadingState />;
  if (status === "not_found") return <NotFoundState />;
  if (status === "error") return <ErrorState />;
  if (!data) return <NotFoundState />;

  const { trip, itinerary } = data;

  return (
    <div className="min-h-screen bg-[var(--color-surface-secondary)]">
      {/* Header with Toqui branding */}
      <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)]">
        <div className="max-w-3xl mx-auto px-4 py-3 flex items-center justify-between">
          <Link href="/waitlist" className="flex items-center gap-2">
            <span className="text-lg font-bold text-[var(--color-accent)]">Toqui</span>
          </Link>
          <span className="text-xs text-[var(--color-text-tertiary)]">Shared Trip</span>
        </div>
      </header>

      <main id="main-content" className="max-w-3xl mx-auto p-4 space-y-6">
        {/* Trip info card */}
        <div className="bg-[var(--color-surface)] rounded-xl p-6 border border-[var(--color-border)]">
          <h1 className="text-2xl font-bold text-[var(--color-text-primary)] mb-2">
            {trip.title}
          </h1>
          {trip.description && (
            <p className="text-[var(--color-text-secondary)] mb-4 leading-relaxed">
              {trip.description}
            </p>
          )}

          <div className="flex flex-wrap gap-3 text-sm">
            {trip.destination_country && (
              <div className="flex items-center gap-1.5 text-[var(--color-text-secondary)]">
                <MapPin size={14} className="text-[var(--color-accent)]" aria-hidden="true" />
                <span>{trip.destination_country}</span>
              </div>
            )}
            {trip.start_date && (
              <div className="flex items-center gap-1.5 text-[var(--color-text-secondary)]">
                <Calendar size={14} className="text-[var(--color-accent)]" aria-hidden="true" />
                <span>
                  {formatDate(trip.start_date)}
                  {trip.end_date && ` - ${formatDate(trip.end_date)}`}
                </span>
              </div>
            )}
          </div>
        </div>

        {/* Itinerary */}
        {itinerary && itinerary.length > 0 ? (
          <div className="space-y-6">
            <h2 className="text-lg font-semibold text-[var(--color-text-primary)]">Itinerary</h2>
            {itinerary.map((day) => (
              <DaySection key={day.day_number} day={day} />
            ))}
          </div>
        ) : (
          <div className="bg-[var(--color-surface)] rounded-xl p-8 border border-[var(--color-border)] text-center">
            <p className="text-[var(--color-text-secondary)]">
              No itinerary has been added to this trip yet.
            </p>
          </div>
        )}

        {/* CTA */}
        <div className="bg-[var(--color-accent-soft)] rounded-xl p-6 border border-[var(--color-border)] text-center">
          <h2 className="text-lg font-semibold text-[var(--color-text-primary)] mb-2">
            Plan your own trip with Toqui
          </h2>
          <p className="text-sm text-[var(--color-text-secondary)] mb-4">
            AI-powered travel planning that builds your perfect itinerary through conversation.
          </p>
          <Link
            href="/waitlist"
            className="inline-flex items-center gap-2 bg-[var(--color-accent)] text-white px-6 py-2.5 rounded-lg hover:bg-[var(--color-accent-hover)] transition-colors text-sm font-medium"
          >
            Get Started
          </Link>
        </div>
      </main>
    </div>
  );
}
