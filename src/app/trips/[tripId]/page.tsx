"use client";

import { useParams } from "next/navigation";
import Link from "next/link";
import { MessageSquare, Briefcase, Play, CheckCircle } from "lucide-react";
import { useTrip, useUpdateTrip } from "@/lib/hooks/useTrips";
import { TripStatus } from "@/gen/toqui/v1/trip_pb";

const statusLabels: Record<number, string> = {
  [TripStatus.PLANNING]: "Planning",
  [TripStatus.ACTIVE]: "Traveling",
  [TripStatus.COMPLETED]: "Completed",
};

const statusColors: Record<number, string> = {
  [TripStatus.PLANNING]: "bg-[var(--color-status-planning-bg)] text-[var(--color-status-planning-text)]",
  [TripStatus.ACTIVE]: "bg-[var(--color-status-active-bg)] text-[var(--color-status-active-text)]",
  [TripStatus.COMPLETED]: "bg-[var(--color-status-completed-bg)] text-[var(--color-status-completed-text)]",
};

export default function TripDetailPage() {
  const { tripId } = useParams<{ tripId: string }>();
  const { trip, isLoading } = useTrip(tripId);
  const updateTrip = useUpdateTrip();

  const handleStartTrip = () => {
    updateTrip.mutate({ id: tripId, status: TripStatus.ACTIVE });
  };

  const handleCompleteTrip = () => {
    updateTrip.mutate({ id: tripId, status: TripStatus.COMPLETED });
  };

  if (isLoading) {
    return (
      <div className="min-h-screen bg-[var(--color-surface-secondary)] flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--color-accent)]" />
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
            <div>
              <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">{trip?.title || "Trip Details"}</h1>
              {trip?.description && (
                <p className="text-sm text-[var(--color-text-secondary)] mt-1">{trip.description}</p>
              )}
            </div>
            <div className="flex items-center gap-3">
              <span className={`px-3 py-1 rounded-full text-xs font-medium ${statusColors[status] || "bg-[var(--color-status-completed-bg)] text-[var(--color-status-completed-text)]"}`}>
                {statusLabels[status] || "Unknown"}
              </span>
              {status === TripStatus.PLANNING && (
                <button
                  onClick={handleStartTrip}
                  disabled={updateTrip.isPending}
                  className="flex items-center gap-1.5 bg-[var(--color-success)] text-white px-4 py-2 rounded-lg hover:opacity-90 transition-colors text-sm font-medium disabled:opacity-50"
                >
                  <Play size={14} />
                  Start Trip
                </button>
              )}
              {status === TripStatus.ACTIVE && (
                <button
                  onClick={handleCompleteTrip}
                  disabled={updateTrip.isPending}
                  className="flex items-center gap-1.5 bg-[var(--color-text-secondary)] text-white px-4 py-2 rounded-lg hover:opacity-90 transition-colors text-sm font-medium disabled:opacity-50"
                >
                  <CheckCircle size={14} />
                  Complete Trip
                </button>
              )}
            </div>
          </div>
        </div>
      </header>

      <main className="max-w-4xl mx-auto p-4">
        <div className="grid gap-4 md:grid-cols-2">
          <Link
            href={`/trips/${tripId}/chat`}
            className="bg-[var(--color-surface)] rounded-xl p-6 hover:shadow-md dark:hover:shadow-black/25 hover:border-[var(--color-border-strong)] transition-all border border-[var(--color-border)]"
          >
            <MessageSquare className="text-[var(--color-accent)] mb-3" size={24} />
            <h2 className="font-semibold mb-1 text-[var(--color-text-primary)]">
              {isActive ? "Travel Companion" : "Plan with AI"}
            </h2>
            <p className="text-sm text-[var(--color-text-secondary)]">
              {isActive
                ? "Get real-time help while traveling"
                : "Chat to build your itinerary"}
            </p>
          </Link>

          <Link
            href={`/trips/${tripId}/bookings`}
            className="bg-[var(--color-surface)] rounded-xl p-6 hover:shadow-md dark:hover:shadow-black/25 hover:border-[var(--color-border-strong)] transition-all border border-[var(--color-border)]"
          >
            <Briefcase className="text-[var(--color-success)] mb-3" size={24} />
            <h2 className="font-semibold mb-1 text-[var(--color-text-primary)]">Bookings</h2>
            <p className="text-sm text-[var(--color-text-secondary)]">Manage your reservations</p>
          </Link>
        </div>
      </main>
    </div>
  );
}
