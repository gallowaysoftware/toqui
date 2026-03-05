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

const statusLabels: Record<number, string> = {
  [TripStatus.PLANNING]: "planning",
  [TripStatus.ACTIVE]: "traveling",
  [TripStatus.COMPLETED]: "completed",
};

const statusColors: Record<string, string> = {
  planning: "bg-blue-100 text-blue-700",
  traveling: "bg-green-100 text-green-700",
  completed: "bg-gray-100 text-gray-500",
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

  const handleTripCreated = useCallback((trip: CreatedTrip) => {
    queryClient.invalidateQueries({ queryKey: ["trips"] });
    // Navigate to the new trip's chat after a short delay so the user sees the AI response
    setTimeout(() => {
      router.push(`/trips/${trip.id}/chat`);
    }, 2000);
  }, [queryClient, router]);

  const handleTripSelected = useCallback((trip: SelectedTrip) => {
    // Navigate to the selected trip's chat after a short delay so the user sees the AI response
    setTimeout(() => {
      router.push(`/trips/${trip.id}/chat`);
    }, 2000);
  }, [router]);

  if (authLoading || !user) {
    return (
      <div className="h-screen flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      </div>
    );
  }

  return (
    <div className="h-screen flex">
      {/* Trip sidebar */}
      <aside className="w-64 bg-gray-50 border-r border-gray-200 flex flex-col flex-shrink-0">
        <div className="p-4 border-b border-gray-200">
          <h2 className="font-semibold text-sm text-gray-500 uppercase tracking-wide">Your Trips</h2>
        </div>
        <nav className="flex-1 overflow-y-auto p-2 space-y-1">
          {trips.length === 0 ? (
            <p className="text-xs text-gray-400 p-2">No trips yet. Start chatting!</p>
          ) : (
            trips.map((trip: Trip) => (
              <TripSidebarItem key={trip.id} trip={trip} />
            ))
          )}
        </nav>
      </aside>

      {/* Main chat area */}
      <main className="flex-1 flex flex-col min-w-0">
        <header className="bg-white border-b border-gray-200 px-4 py-3 flex-shrink-0 flex items-center gap-3">
          <MessageSquare size={20} className="text-blue-600" />
          <h1 className="text-lg font-semibold">Toqui</h1>
        </header>
        <ChatContainer mode="selection" onTripCreated={handleTripCreated} onTripSelected={handleTripSelected} />
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
      className="block rounded-lg p-3 hover:bg-gray-100 transition-colors"
    >
      <div className="flex items-center justify-between mb-0.5">
        <span className="text-sm font-medium text-gray-900 truncate">{trip.title}</span>
        <span className={`text-[10px] px-1.5 py-0.5 rounded-full font-medium flex-shrink-0 ${colors}`}>
          {label}
        </span>
      </div>
      {trip.description && (
        <p className="text-xs text-gray-400 truncate">{trip.description}</p>
      )}
    </Link>
  );
}
