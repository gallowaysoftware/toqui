"use client";

import {
  Plane,
  Hotel,
  Car,
  TrainFront,
  Ticket,
  UtensilsCrossed,
  Map,
  Package,
  Calendar,
  Hash,
  MapPin,
  ArrowLeft,
  Trash2,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { Booking } from "@/gen/toqui/v1/booking_pb";
import { BookingType } from "@/gen/toqui/v1/booking_pb";
import {
  bookingTypeLabels,
  bookingTypeColors,
  bookingSourceLabels,
  formatTimestampWithTime,
  formatTimestamp,
  getBookingSubtitle,
} from "@/lib/booking-utils";

const iconMap: Record<BookingType, LucideIcon> = {
  [BookingType.UNSPECIFIED]: Package,
  [BookingType.FLIGHT]: Plane,
  [BookingType.HOTEL]: Hotel,
  [BookingType.CAR_RENTAL]: Car,
  [BookingType.TRAIN]: TrainFront,
  [BookingType.ACTIVITY]: Ticket,
  [BookingType.RESTAURANT]: UtensilsCrossed,
  [BookingType.OTHER]: Package,
  [BookingType.TOUR]: Map,
};

interface BookingDetailProps {
  booking: Booking;
  onBack: () => void;
  onDelete?: (booking: Booking) => void;
  isDeleting?: boolean;
}

function DetailRow({
  icon: Icon,
  label,
  value,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
}) {
  if (!value) return null;
  return (
    <div className="flex items-start gap-3 py-2">
      <Icon size={16} className="text-[var(--color-text-tertiary)] mt-0.5 flex-shrink-0" />
      <div>
        <p className="text-xs text-[var(--color-text-tertiary)] uppercase tracking-wide">{label}</p>
        <p className="text-sm text-[var(--color-text-primary)]">{value}</p>
      </div>
    </div>
  );
}

function FlightDetailSection({ booking }: { booking: Booking }) {
  if (booking.bookingDetails.case !== "flightDetails") return null;
  const d = booking.bookingDetails.value;
  return (
    <div className="space-y-1">
      <h3 className="text-sm font-medium text-[var(--color-text-secondary)] mb-2">
        Flight Details
      </h3>
      {d.airline && <DetailRow icon={Plane} label="Airline" value={d.airline} />}
      {d.flightNumber && <DetailRow icon={Hash} label="Flight Number" value={d.flightNumber} />}
      {d.departureAirport && (
        <DetailRow
          icon={MapPin}
          label="Departure"
          value={`${d.departureAirport}${d.departureTerminal ? ` (Terminal ${d.departureTerminal})` : ""}`}
        />
      )}
      {d.arrivalAirport && (
        <DetailRow
          icon={MapPin}
          label="Arrival"
          value={`${d.arrivalAirport}${d.arrivalTerminal ? ` (Terminal ${d.arrivalTerminal})` : ""}`}
        />
      )}
      {d.seat && <DetailRow icon={Ticket} label="Seat" value={d.seat} />}
      {d.cabinClass && <DetailRow icon={Ticket} label="Class" value={d.cabinClass} />}
      {d.passengers.length > 0 && (
        <DetailRow icon={Users} label="Passengers" value={d.passengers.join(", ")} />
      )}
    </div>
  );
}

function HotelDetailSection({ booking }: { booking: Booking }) {
  if (booking.bookingDetails.case !== "hotelDetails") return null;
  const d = booking.bookingDetails.value;
  return (
    <div className="space-y-1">
      <h3 className="text-sm font-medium text-[var(--color-text-secondary)] mb-2">Hotel Details</h3>
      {d.hotelName && <DetailRow icon={Hotel} label="Hotel" value={d.hotelName} />}
      {d.roomType && <DetailRow icon={Ticket} label="Room Type" value={d.roomType} />}
      {d.checkInDate && <DetailRow icon={Calendar} label="Check-in" value={d.checkInDate} />}
      {d.checkOutDate && <DetailRow icon={Calendar} label="Check-out" value={d.checkOutDate} />}
      {d.numGuests > 0 && <DetailRow icon={Users} label="Guests" value={String(d.numGuests)} />}
      {d.address && <DetailRow icon={MapPin} label="Address" value={d.address} />}
    </div>
  );
}

function CarRentalDetailSection({ booking }: { booking: Booking }) {
  if (booking.bookingDetails.case !== "carRentalDetails") return null;
  const d = booking.bookingDetails.value;
  return (
    <div className="space-y-1">
      <h3 className="text-sm font-medium text-[var(--color-text-secondary)] mb-2">
        Car Rental Details
      </h3>
      {d.company && <DetailRow icon={Car} label="Company" value={d.company} />}
      {d.carType && <DetailRow icon={Car} label="Vehicle" value={d.carType} />}
      {d.pickupLocation && <DetailRow icon={MapPin} label="Pick-up" value={d.pickupLocation} />}
      {d.dropoffLocation && <DetailRow icon={MapPin} label="Drop-off" value={d.dropoffLocation} />}
      {d.pickupTime && <DetailRow icon={Calendar} label="Pick-up Time" value={d.pickupTime} />}
      {d.dropoffTime && <DetailRow icon={Calendar} label="Drop-off Time" value={d.dropoffTime} />}
    </div>
  );
}

function TrainDetailSection({ booking }: { booking: Booking }) {
  if (booking.bookingDetails.case !== "trainDetails") return null;
  const d = booking.bookingDetails.value;
  return (
    <div className="space-y-1">
      <h3 className="text-sm font-medium text-[var(--color-text-secondary)] mb-2">Train Details</h3>
      {d.operator && <DetailRow icon={TrainFront} label="Operator" value={d.operator} />}
      {d.trainNumber && <DetailRow icon={Hash} label="Train Number" value={d.trainNumber} />}
      {d.departureStation && (
        <DetailRow icon={MapPin} label="Departure" value={d.departureStation} />
      )}
      {d.arrivalStation && <DetailRow icon={MapPin} label="Arrival" value={d.arrivalStation} />}
      {d.seat && <DetailRow icon={Ticket} label="Seat" value={d.seat} />}
      {d.carNumber && <DetailRow icon={TrainFront} label="Car" value={d.carNumber} />}
      {d.class && <DetailRow icon={Ticket} label="Class" value={d.class} />}
    </div>
  );
}

function TourDetailSection({ booking }: { booking: Booking }) {
  if (booking.bookingDetails.case !== "tourDetails") return null;
  const d = booking.bookingDetails.value;
  return (
    <div className="space-y-1">
      <h3 className="text-sm font-medium text-[var(--color-text-secondary)] mb-2">Tour Details</h3>
      {d.tourName && <DetailRow icon={Map} label="Tour" value={d.tourName} />}
      {d.tourOperator && <DetailRow icon={Map} label="Operator" value={d.tourOperator} />}
      {d.meetingPoint && <DetailRow icon={MapPin} label="Meeting Point" value={d.meetingPoint} />}
      {d.numParticipants > 0 && (
        <DetailRow icon={Users} label="Participants" value={String(d.numParticipants)} />
      )}
      {d.stops.length > 0 && (
        <div className="pl-7 mt-2">
          <p className="text-xs text-[var(--color-text-tertiary)] uppercase tracking-wide mb-1">
            Stops
          </p>
          <ul className="space-y-1">
            {d.stops.map((stop, i) => (
              <li key={i} className="text-sm text-[var(--color-text-secondary)]">
                {stop.name}
                {stop.location ? ` - ${stop.location}` : ""}
                {stop.duration ? ` (${stop.duration})` : ""}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

function ActivityDetailSection({ booking }: { booking: Booking }) {
  if (booking.bookingDetails.case !== "activityDetails") return null;
  const d = booking.bookingDetails.value;
  return (
    <div className="space-y-1">
      <h3 className="text-sm font-medium text-[var(--color-text-secondary)] mb-2">
        Activity Details
      </h3>
      {d.activityName && <DetailRow icon={Ticket} label="Activity" value={d.activityName} />}
      {d.operator && <DetailRow icon={Ticket} label="Operator" value={d.operator} />}
      {d.location && <DetailRow icon={MapPin} label="Location" value={d.location} />}
      {d.numGuests > 0 && <DetailRow icon={Users} label="Guests" value={String(d.numGuests)} />}
      {d.notes && <DetailRow icon={Ticket} label="Notes" value={d.notes} />}
    </div>
  );
}

function RestaurantDetailSection({ booking }: { booking: Booking }) {
  if (booking.bookingDetails.case !== "restaurantDetails") return null;
  const d = booking.bookingDetails.value;
  return (
    <div className="space-y-1">
      <h3 className="text-sm font-medium text-[var(--color-text-secondary)] mb-2">
        Restaurant Details
      </h3>
      {d.restaurantName && (
        <DetailRow icon={UtensilsCrossed} label="Restaurant" value={d.restaurantName} />
      )}
      {d.cuisine && <DetailRow icon={UtensilsCrossed} label="Cuisine" value={d.cuisine} />}
      {d.partySize > 0 && <DetailRow icon={Users} label="Party Size" value={String(d.partySize)} />}
      {d.notes && <DetailRow icon={Ticket} label="Notes" value={d.notes} />}
    </div>
  );
}

export function BookingDetail({ booking, onBack, onDelete, isDeleting }: BookingDetailProps) {
  const Icon = iconMap[booking.type] || Package;
  const typeLabel = bookingTypeLabels[booking.type] || "Other";
  const typeColor =
    bookingTypeColors[booking.type] ||
    "bg-[var(--color-surface-tertiary)] text-[var(--color-text-secondary)]";
  const subtitle = getBookingSubtitle(booking);

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={onBack}
          className="text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] rounded"
          aria-label="Back to bookings"
        >
          <ArrowLeft size={20} aria-hidden="true" />
        </button>
        <h2 className="font-semibold text-lg text-[var(--color-text-primary)] flex-1">
          Booking Details
        </h2>
        {onDelete && (
          <button
            type="button"
            onClick={() => onDelete(booking)}
            disabled={isDeleting}
            className="text-[var(--color-error)] hover:opacity-80 transition-colors disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] rounded"
            aria-label="Delete booking"
          >
            <Trash2 size={18} aria-hidden="true" />
          </button>
        )}
      </div>

      {/* Main card */}
      <div className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-6">
        {/* Title row */}
        <div className="flex items-start gap-3 mb-4">
          <div
            className={`flex-shrink-0 w-12 h-12 rounded-lg flex items-center justify-center ${typeColor}`}
          >
            <Icon size={24} aria-hidden="true" />
          </div>
          <div className="flex-1 min-w-0">
            <h3 className="text-lg font-semibold text-[var(--color-text-primary)]">
              {booking.title || typeLabel}
            </h3>
            {subtitle && (
              <p className="text-sm text-[var(--color-text-secondary)] mt-0.5">{subtitle}</p>
            )}
            <span
              className={`inline-block mt-1 text-xs px-2 py-0.5 rounded-full font-medium ${typeColor}`}
            >
              {typeLabel}
            </span>
          </div>
        </div>

        {/* Common fields */}
        <div className="border-t border-[var(--color-border)] pt-4 space-y-1">
          <DetailRow icon={Hash} label="Confirmation Code" value={booking.confirmationCode} />
          <DetailRow
            icon={Calendar}
            label="Start"
            value={formatTimestampWithTime(booking.startTime)}
          />
          <DetailRow icon={Calendar} label="End" value={formatTimestampWithTime(booking.endTime)} />
          <DetailRow icon={MapPin} label="Address" value={booking.address} />
          {booking.provider && (
            <DetailRow icon={Package} label="Provider" value={booking.provider} />
          )}
          {booking.numGuests > 0 && (
            <DetailRow icon={Users} label="Guests" value={String(booking.numGuests)} />
          )}
          {booking.departureLocation && (
            <DetailRow icon={MapPin} label="Departure Location" value={booking.departureLocation} />
          )}
          {booking.arrivalLocation && (
            <DetailRow icon={MapPin} label="Arrival Location" value={booking.arrivalLocation} />
          )}
        </div>

        {/* Type-specific details */}
        {booking.bookingDetails.case && (
          <div className="border-t border-[var(--color-border)] pt-4 mt-4">
            <FlightDetailSection booking={booking} />
            <HotelDetailSection booking={booking} />
            <CarRentalDetailSection booking={booking} />
            <TrainDetailSection booking={booking} />
            <TourDetailSection booking={booking} />
            <ActivityDetailSection booking={booking} />
            <RestaurantDetailSection booking={booking} />
          </div>
        )}

        {/* Source info */}
        <div className="border-t border-[var(--color-border)] pt-3 mt-4 text-xs text-[var(--color-text-tertiary)]">
          <span>Source: {bookingSourceLabels[booking.source] || "Unknown"}</span>
          {booking.createdAt && (
            <span className="ml-3">Added {formatTimestamp(booking.createdAt)}</span>
          )}
        </div>
      </div>
    </div>
  );
}
