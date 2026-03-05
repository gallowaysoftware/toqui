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
  [TripStatus.PLANNING]: "bg-blue-100 text-blue-700",
  [TripStatus.ACTIVE]: "bg-green-100 text-green-700",
  [TripStatus.COMPLETED]: "bg-gray-100 text-gray-500",
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
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      </div>
    );
  }

  const status = trip?.status ?? TripStatus.PLANNING;
  const isActive = status === TripStatus.ACTIVE;

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200 px-4 py-4">
        <div className="max-w-4xl mx-auto">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-xl font-semibold">{trip?.title || "Trip Details"}</h1>
              {trip?.description && (
                <p className="text-sm text-gray-500 mt-1">{trip.description}</p>
              )}
            </div>
            <div className="flex items-center gap-3">
              <span className={`px-3 py-1 rounded-full text-xs font-medium ${statusColors[status] || "bg-gray-100 text-gray-500"}`}>
                {statusLabels[status] || "Unknown"}
              </span>
              {status === TripStatus.PLANNING && (
                <button
                  onClick={handleStartTrip}
                  disabled={updateTrip.isPending}
                  className="flex items-center gap-1.5 bg-green-600 text-white px-4 py-2 rounded-lg hover:bg-green-700 transition-colors text-sm font-medium disabled:opacity-50"
                >
                  <Play size={14} />
                  Start Trip
                </button>
              )}
              {status === TripStatus.ACTIVE && (
                <button
                  onClick={handleCompleteTrip}
                  disabled={updateTrip.isPending}
                  className="flex items-center gap-1.5 bg-gray-600 text-white px-4 py-2 rounded-lg hover:bg-gray-700 transition-colors text-sm font-medium disabled:opacity-50"
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
            className="bg-white rounded-xl p-6 hover:shadow-md transition-shadow border border-gray-200"
          >
            <MessageSquare className="text-blue-600 mb-3" size={24} />
            <h2 className="font-semibold mb-1">
              {isActive ? "Travel Companion" : "Plan with AI"}
            </h2>
            <p className="text-sm text-gray-500">
              {isActive
                ? "Get real-time help while traveling"
                : "Chat to build your itinerary"}
            </p>
          </Link>

          <Link
            href={`/trips/${tripId}/bookings`}
            className="bg-white rounded-xl p-6 hover:shadow-md transition-shadow border border-gray-200"
          >
            <Briefcase className="text-green-600 mb-3" size={24} />
            <h2 className="font-semibold mb-1">Bookings</h2>
            <p className="text-sm text-gray-500">Manage your reservations</p>
          </Link>
        </div>
      </main>
    </div>
  );
}
