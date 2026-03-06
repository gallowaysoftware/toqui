"use client";

import { useParams } from "next/navigation";
import { ChatContainer } from "@/components/chat/ChatContainer";
import { useTrip } from "@/lib/hooks/useTrips";
import { TripStatus } from "@/gen/toqui/v1/trip_pb";
import { ThemeToggleButton } from "@/components/theme/ThemeToggle";

export default function ChatPage() {
  const { tripId } = useParams<{ tripId: string }>();
  const { trip, isLoading } = useTrip(tripId);

  const mode = trip?.status === TripStatus.ACTIVE ? "companion" : "planning";

  if (isLoading) {
    return (
      <div className="h-screen flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--color-accent)]" />
      </div>
    );
  }

  return (
    <div className="h-screen flex flex-col">
      <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)] px-4 py-3 flex-shrink-0 flex items-center justify-between">
        <h1 className="text-lg font-semibold text-[var(--color-text-primary)]">
          {mode === "companion" ? "Travel Companion" : "Trip Planning"}
        </h1>
        <div className="flex items-center gap-2">
          <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${
            mode === "companion"
              ? "bg-[var(--color-status-active-bg)] text-[var(--color-status-active-text)]"
              : "bg-[var(--color-status-planning-bg)] text-[var(--color-status-planning-text)]"
          }`}>
            {mode === "companion" ? "Traveling" : "Planning"}
          </span>
          <ThemeToggleButton />
        </div>
      </header>
      <ChatContainer tripId={tripId} mode={mode} />
    </div>
  );
}
