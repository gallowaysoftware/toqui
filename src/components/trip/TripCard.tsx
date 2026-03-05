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
  planning: "bg-yellow-100 text-yellow-800",
  active: "bg-green-100 text-green-800",
  completed: "bg-gray-100 text-gray-800",
};

export function TripCard({ trip }: { trip: Trip }) {
  const label = statusLabel[trip.status] || "planning";

  return (
    <Link
      href={`/trips/${trip.id}`}
      className="bg-white rounded-xl p-5 border border-gray-200 hover:shadow-md transition-shadow block"
    >
      <div className="flex items-start justify-between mb-2">
        <h3 className="font-semibold text-gray-900">{trip.title}</h3>
        <span
          className={`text-xs px-2 py-1 rounded-full font-medium ${statusColors[label]}`}
        >
          {label}
        </span>
      </div>

      {trip.description && (
        <p className="text-sm text-gray-500 mb-3 line-clamp-2">{trip.description}</p>
      )}

      {trip.startDate && (
        <div className="flex items-center gap-1.5 text-xs text-gray-400">
          <Calendar size={12} />
          <span>
            {trip.startDate}
            {trip.endDate && ` - ${trip.endDate}`}
          </span>
        </div>
      )}
    </Link>
  );
}
