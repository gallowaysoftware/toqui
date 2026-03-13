import {
  Plane,
  Hotel,
  Car,
  TrainFront,
  Ticket,
  UtensilsCrossed,
  Map,
  Package,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { BookingType, BookingSource } from "@/gen/toqui/v1/booking_pb";
import type { Booking } from "@/gen/toqui/v1/booking_pb";
import type { Timestamp } from "@bufbuild/protobuf/wkt";

/**
 * Maps a BookingType enum to a human-readable label.
 */
export const bookingTypeLabels: Record<BookingType, string> = {
  [BookingType.UNSPECIFIED]: "Other",
  [BookingType.FLIGHT]: "Flight",
  [BookingType.HOTEL]: "Hotel",
  [BookingType.CAR_RENTAL]: "Car Rental",
  [BookingType.TRAIN]: "Train",
  [BookingType.ACTIVITY]: "Activity",
  [BookingType.RESTAURANT]: "Restaurant",
  [BookingType.OTHER]: "Other",
  [BookingType.TOUR]: "Tour",
};

/**
 * Maps a BookingType enum to a Lucide icon component.
 * Shared between BookingCard and BookingDetail to avoid duplication.
 */
export const bookingTypeIcons: Record<BookingType, LucideIcon> = {
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

/**
 * Maps a BookingType enum to a background color class for cards.
 */
export const bookingTypeColors: Record<BookingType, string> = {
  [BookingType.UNSPECIFIED]: "bg-[var(--color-booking-other-bg)] text-[var(--color-booking-other-text)]",
  [BookingType.FLIGHT]: "bg-[var(--color-booking-flight-bg)] text-[var(--color-booking-flight-text)]",
  [BookingType.HOTEL]: "bg-[var(--color-booking-hotel-bg)] text-[var(--color-booking-hotel-text)]",
  [BookingType.CAR_RENTAL]: "bg-[var(--color-booking-car-bg)] text-[var(--color-booking-car-text)]",
  [BookingType.TRAIN]: "bg-[var(--color-booking-train-bg)] text-[var(--color-booking-train-text)]",
  [BookingType.ACTIVITY]: "bg-[var(--color-booking-activity-bg)] text-[var(--color-booking-activity-text)]",
  [BookingType.RESTAURANT]: "bg-[var(--color-booking-restaurant-bg)] text-[var(--color-booking-restaurant-text)]",
  [BookingType.OTHER]: "bg-[var(--color-booking-other-bg)] text-[var(--color-booking-other-text)]",
  [BookingType.TOUR]: "bg-[var(--color-booking-tour-bg)] text-[var(--color-booking-tour-text)]",
};

export const bookingSourceLabels: Record<BookingSource, string> = {
  [BookingSource.UNSPECIFIED]: "Unknown",
  [BookingSource.EMAIL]: "Email",
  [BookingSource.MANUAL]: "Manual",
  [BookingSource.PASTE]: "Paste",
};

/**
 * Formats a protobuf Timestamp to a readable date string.
 */
export function formatTimestamp(ts: Timestamp | undefined): string {
  if (!ts) return "";
  const date = new Date(Number(ts.seconds) * 1000 + ts.nanos / 1_000_000);
  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

/**
 * Formats a protobuf Timestamp to a readable date+time string.
 */
export function formatTimestampWithTime(ts: Timestamp | undefined): string {
  if (!ts) return "";
  const date = new Date(Number(ts.seconds) * 1000 + ts.nanos / 1_000_000);
  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

/**
 * Formats a date range from two timestamps.
 */
export function formatDateRange(start: Timestamp | undefined, end: Timestamp | undefined): string {
  const startStr = formatTimestamp(start);
  const endStr = formatTimestamp(end);
  if (!startStr && !endStr) return "";
  if (!endStr) return startStr;
  if (!startStr) return endStr;
  if (startStr === endStr) return startStr;
  return `${startStr} - ${endStr}`;
}

/**
 * Returns a subtitle for a booking based on its type-specific details.
 */
export function getBookingSubtitle(booking: Booking): string {
  switch (booking.bookingDetails.case) {
    case "flightDetails": {
      const d = booking.bookingDetails.value;
      const route =
        d.departureAirport && d.arrivalAirport ? `${d.departureAirport} → ${d.arrivalAirport}` : "";
      const flight = d.flightNumber
        ? `${d.airline || ""} ${d.flightNumber}`.trim()
        : d.airline || "";
      return [route, flight].filter(Boolean).join(" · ");
    }
    case "hotelDetails": {
      const d = booking.bookingDetails.value;
      return [d.hotelName, d.roomType].filter(Boolean).join(" · ");
    }
    case "carRentalDetails": {
      const d = booking.bookingDetails.value;
      return [d.company, d.carType].filter(Boolean).join(" · ");
    }
    case "trainDetails": {
      const d = booking.bookingDetails.value;
      const route =
        d.departureStation && d.arrivalStation ? `${d.departureStation} → ${d.arrivalStation}` : "";
      return [route, d.operator].filter(Boolean).join(" · ");
    }
    case "tourDetails": {
      const d = booking.bookingDetails.value;
      return [d.tourName, d.tourOperator].filter(Boolean).join(" · ");
    }
    case "activityDetails": {
      const d = booking.bookingDetails.value;
      return [d.activityName, d.operator].filter(Boolean).join(" · ");
    }
    case "restaurantDetails": {
      const d = booking.bookingDetails.value;
      return [d.restaurantName, d.cuisine].filter(Boolean).join(" · ");
    }
    default:
      return booking.provider || "";
  }
}
