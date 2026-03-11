"use client";

import { useParams } from "next/navigation";
import Link from "next/link";
import {
  MessageSquare,
  Briefcase,
  Play,
  CheckCircle,
  Map,
  Printer,
  CalendarDays,
  Download,
  ArrowLeft,
  Settings,
} from "lucide-react";
import { useTrip, useUpdateTrip } from "@/lib/hooks/useTrips";
import { useItinerary } from "@/lib/hooks/useItinerary";
import { TripStatus } from "@/gen/toqui/v1/trip_pb";
import DynamicItineraryMap from "@/components/map/DynamicItineraryMap";
import { exportItineraryPDF } from "@/lib/export/pdf-export";
import { exportItineraryICal } from "@/lib/export/calendar-export";

const statusLabels: Record<number, string> = {
  [TripStatus.PLANNING]: "Planning",
  [TripStatus.ACTIVE]: "Traveling",
  [TripStatus.COMPLETED]: "Completed",
};

const statusColors: Record<number, string> = {
  [TripStatus.PLANNING]:
    "bg-[var(--color-status-planning-bg)] text-[var(--color-status-planning-text)]",
  [TripStatus.ACTIVE]: "bg-[var(--color-status-active-bg)] text-[var(--color-status-active-text)]",
  [TripStatus.COMPLETED]:
    "bg-[var(--color-status-completed-bg)] text-[var(--color-status-completed-text)]",
};

export default function TripDetailPage() {
  const { tripId } = useParams<{ tripId: string }>();
  const { trip, isLoading } = useTrip(tripId);
  const { itinerary, isLoading: itineraryLoading } = useItinerary(tripId);
  const updateTrip = useUpdateTrip();

  const handleStartTrip = () => {
    updateTrip.mutate({ id: tripId, status: TripStatus.ACTIVE });
  };

  const handleCompleteTrip = () => {
    updateTrip.mutate({ id: tripId, status: TripStatus.COMPLETED });
  };

  const handleExportPDF = () => {
    if (trip && itinerary) {
      exportItineraryPDF(trip, itinerary);
    }
  };

  const handleExportCalendar = () => {
    if (trip && itinerary) {
      exportItineraryICal(trip, itinerary);
    }
  };

  const hasItinerary = itinerary && itinerary.days.length > 0;

  if (isLoading) {
    return (
      <div
        className="min-h-screen bg-[var(--color-surface-secondary)] flex items-center justify-center"
        aria-busy="true"
        role="status"
      >
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--color-accent)]" />
        <span className="sr-only">Loading trip details...</span>
      </div>
    );
  }

  const status = trip?.status ?? TripStatus.PLANNING;
  const isActive = status === TripStatus.ACTIVE;

  return (
    <div className="min-h-screen bg-[var(--color-surface-secondary)]">
      <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)] px-4 py-4">
        <div className="max-w-4xl mx-auto">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3 min-w-0">
              <Link
                href="/trips"
                className="p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-tertiary)] transition-colors flex-shrink-0 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
                aria-label="Back to trips"
              >
                <ArrowLeft size={20} aria-hidden="true" />
              </Link>
              <div className="min-w-0">
                <h1 className="text-xl font-semibold text-[var(--color-text-primary)] truncate">
                  {trip?.title ?? "Trip Details"}
                </h1>
                {trip?.description && (
                  <p className="text-sm text-[var(--color-text-secondary)] mt-1 truncate">
                    {trip.description}
                  </p>
                )}
              </div>
            </div>
            <div className="flex items-center gap-3 flex-shrink-0">
              <Link
                href={`/trips/${tripId}/settings`}
                className="p-1.5 rounded-lg text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-tertiary)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
                aria-label="Trip settings"
              >
                <Settings size={18} aria-hidden="true" />
              </Link>
              <span
                className={`px-3 py-1 rounded-full text-xs font-medium ${statusColors[status] ?? "bg-[var(--color-status-completed-bg)] text-[var(--color-status-completed-text)]"}`}
              >
                {statusLabels[status] ?? "Unknown"}
              </span>
              {status === TripStatus.PLANNING && (
                <button
                  onClick={handleStartTrip}
                  disabled={updateTrip.isPending}
                  className="flex items-center gap-1.5 bg-[var(--color-success)] text-white px-4 py-2 rounded-lg hover:opacity-90 transition-colors text-sm font-medium disabled:opacity-50"
                >
                  <Play size={14} aria-hidden="true" />
                  Start Trip
                </button>
              )}
              {status === TripStatus.ACTIVE && (
                <button
                  onClick={handleCompleteTrip}
                  disabled={updateTrip.isPending}
                  className="flex items-center gap-1.5 bg-[var(--color-text-secondary)] text-white px-4 py-2 rounded-lg hover:opacity-90 transition-colors text-sm font-medium disabled:opacity-50"
                >
                  <CheckCircle size={14} aria-hidden="true" />
                  Complete Trip
                </button>
              )}
            </div>
          </div>
        </div>
      </header>

      <main id="main-content" className="max-w-4xl mx-auto p-4 space-y-6">
        <div className="grid gap-4 md:grid-cols-2">
          <Link
            href={`/trips/${tripId}/chat`}
            className="bg-[var(--color-surface)] rounded-xl p-6 hover:shadow-md dark:hover:shadow-black/25 hover:border-[var(--color-border-strong)] transition-all border border-[var(--color-border)]"
          >
            <MessageSquare
              className="text-[var(--color-accent)] mb-3"
              size={24}
              aria-hidden="true"
            />
            <h2 className="font-semibold mb-1 text-[var(--color-text-primary)]">
              {isActive ? "Travel Companion" : "Plan with AI"}
            </h2>
            <p className="text-sm text-[var(--color-text-secondary)]">
              {isActive ? "Get real-time help while traveling" : "Chat to build your itinerary"}
            </p>
          </Link>

          <Link
            href={`/trips/${tripId}/bookings`}
            className="bg-[var(--color-surface)] rounded-xl p-6 hover:shadow-md dark:hover:shadow-black/25 hover:border-[var(--color-border-strong)] transition-all border border-[var(--color-border)]"
          >
            <Briefcase className="text-[var(--color-success)] mb-3" size={24} aria-hidden="true" />
            <h2 className="font-semibold mb-1 text-[var(--color-text-primary)]">Bookings</h2>
            <p className="text-sm text-[var(--color-text-secondary)]">Manage your reservations</p>
          </Link>
        </div>

        {/* Itinerary Map */}
        <section>
          <div className="flex items-center gap-2 mb-3">
            <Map className="text-[var(--color-accent)]" size={20} aria-hidden="true" />
            <h2 className="font-semibold text-[var(--color-text-primary)]">Itinerary Map</h2>
          </div>
          <div className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] overflow-hidden">
            <DynamicItineraryMap
              itinerary={itinerary}
              isLoading={itineraryLoading}
              className="h-[300px] md:h-[400px]"
            />
          </div>
        </section>

        {/* Export */}
        {hasItinerary && (
          <section>
            <div className="flex items-center gap-2 mb-3">
              <Download className="text-[var(--color-accent)]" size={20} aria-hidden="true" />
              <h2 className="font-semibold text-[var(--color-text-primary)]">Export Itinerary</h2>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
              <button
                onClick={handleExportPDF}
                className="flex items-center gap-3 bg-[var(--color-surface)] rounded-xl p-4 border border-[var(--color-border)] hover:shadow-md dark:hover:shadow-black/25 hover:border-[var(--color-border-strong)] transition-all text-left"
              >
                <div className="flex-shrink-0 w-10 h-10 rounded-full bg-[var(--color-accent-soft)] flex items-center justify-center">
                  <Printer size={18} className="text-[var(--color-accent)]" aria-hidden="true" />
                </div>
                <div>
                  <h3 className="font-medium text-sm text-[var(--color-text-primary)]">
                    Export PDF
                  </h3>
                  <p className="text-xs text-[var(--color-text-tertiary)]">
                    Print-friendly itinerary
                  </p>
                </div>
              </button>
              <button
                onClick={handleExportCalendar}
                className="flex items-center gap-3 bg-[var(--color-surface)] rounded-xl p-4 border border-[var(--color-border)] hover:shadow-md dark:hover:shadow-black/25 hover:border-[var(--color-border-strong)] transition-all text-left"
              >
                <div className="flex-shrink-0 w-10 h-10 rounded-full bg-[var(--color-accent-soft)] flex items-center justify-center">
                  <CalendarDays
                    size={18}
                    className="text-[var(--color-accent)]"
                    aria-hidden="true"
                  />
                </div>
                <div>
                  <h3 className="font-medium text-sm text-[var(--color-text-primary)]">
                    Export Calendar
                  </h3>
                  <p className="text-xs text-[var(--color-text-tertiary)]">Download .ics file</p>
                </div>
              </button>
            </div>
          </section>
        )}
      </main>
    </div>
  );
}
