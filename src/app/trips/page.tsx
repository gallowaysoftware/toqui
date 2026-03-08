"use client";

import { useEffect, useCallback } from "react";
import { useAuth } from "@/components/providers/AuthProvider";
import { useTrips } from "@/lib/hooks/useTrips";
import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { ChatContainer } from "@/components/chat/ChatContainer";
import { TripStatus } from "@/gen/toqui/v1/trip_pb";
import type { Trip } from "@/gen/toqui/v1/trip_pb";
import type { CreatedTrip, SelectedTrip } from "@/lib/hooks/useChat";
import Link from "next/link";
import { MessageSquare } from "lucide-react";
import { ThemeToggleButton } from "@/components/theme/ThemeToggle";

const statusLabels: Record<number, string> = {
  [TripStatus.PLANNING]: "planning",
  [TripStatus.ACTIVE]: "traveling",
  [TripStatus.COMPLETED]: "completed",
};

const statusColors: Record<string, string> = {
  planning: "bg-[var(--color-status-planning-bg)] text-[var(--color-status-planning-text)]",
  traveling: "bg-[var(--color-status-active-bg)] text-[var(--color-status-active-text)]",
  completed: "bg-[var(--color-status-completed-bg)] text-[var(--color-status-completed-text)]",
};

export default function TripsPage() {
  const { user, isLoading: authLoading } = useAuth();
  const { trips } = useTrips();
  const router = useRouter();
  const queryClient = useQueryClient();

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/");
    }
  }, [authLoading, user, router]);

  const handleTripCreated = useCallback(
    (trip: CreatedTrip) => {
      void queryClient.invalidateQueries({ queryKey: ["trips"] });
      // Navigate to the new trip's chat after a short delay so the user sees the AI response
      setTimeout(() => {
        router.push(`/trips/${trip.id}/chat`);
      }, 2000);
    },
    [queryClient, router],
  );

  const handleTripSelected = useCallback(
    (trip: SelectedTrip) => {
      // Navigate to the selected trip's chat after a short delay so the user sees the AI response
      setTimeout(() => {
        router.push(`/trips/${trip.id}/chat`);
      }, 2000);
    },
    [router],
  );

  if (authLoading || !user) {
    return (
      <div className="h-screen flex items-center justify-center" aria-busy="true" role="status">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--color-accent)]" />
        <span className="sr-only">Loading...</span>
      </div>
    );
  }

  return (
    <div className="h-screen flex">
      {/* Trip sidebar */}
      <aside className="w-64 bg-[var(--color-surface-secondary)] border-r border-[var(--color-border)] flex flex-col flex-shrink-0">
        <div className="p-4 border-b border-[var(--color-border)]">
          <h2 className="font-semibold text-sm text-[var(--color-text-secondary)] uppercase tracking-wide">
            Your Trips
          </h2>
        </div>
        <nav className="flex-1 overflow-y-auto p-2" aria-label="Trip list">
          {trips.length === 0 ? (
            <p className="text-xs text-[var(--color-text-tertiary)] p-2">
              No trips yet. Start chatting!
            </p>
          ) : (
            trips.map((trip: Trip) => <TripSidebarItem key={trip.id} trip={trip} />)
          )}
        </nav>
      </aside>

      {/* Main chat area */}
      <main id="main-content" className="flex-1 flex flex-col min-w-0">
        <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)] px-4 py-3 flex-shrink-0 flex items-center gap-3">
          <MessageSquare size={20} className="text-[var(--color-accent)]" aria-hidden="true" />
          <h1 className="text-lg font-semibold text-[var(--color-text-primary)] flex-1">Toqui</h1>
          <ThemeToggleButton />
        </header>
        <ChatContainer
          mode="selection"
          onTripCreated={handleTripCreated}
          onTripSelected={handleTripSelected}
        />
      </main>
    </div>
  );
}

function TripSidebarItem({ trip }: { trip: Trip }) {
  const label = statusLabels[trip.status] || "planning";
  const colors = statusColors[label] || statusColors.planning;

  return (
    <Link
      href={`/trips/${trip.id}`}
      className="block rounded-lg p-3 hover:bg-[var(--color-surface-tertiary)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
    >
      <div className="flex items-center justify-between mb-0.5">
        <span className="text-sm font-medium text-[var(--color-text-primary)] truncate">
          {trip.title}
        </span>
        <span
          className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium flex-shrink-0 ${colors}`}
        >
          {label}
        </span>
      </div>
      {trip.description && (
        <p className="text-xs text-[var(--color-text-tertiary)] truncate">{trip.description}</p>
      )}
    </Link>
  );
}
