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
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { Booking } from "@/gen/toqui/v1/booking_pb";
import { BookingType } from "@/gen/toqui/v1/booking_pb";
import {
  bookingTypeLabels,
  bookingTypeColors,
  formatDateRange,
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

interface BookingCardProps {
  booking: Booking;
  onClick?: (booking: Booking) => void;
}

export function BookingCard({ booking, onClick }: BookingCardProps) {
  const Icon = iconMap[booking.type] || Package;
  const typeLabel = bookingTypeLabels[booking.type] || "Other";
  const typeColor = bookingTypeColors[booking.type] || "bg-[var(--color-surface-tertiary)] text-[var(--color-text-secondary)]";
  const dateRange = formatDateRange(booking.startTime, booking.endTime);
  const subtitle = getBookingSubtitle(booking);

  return (
    <button
      type="button"
      onClick={() => onClick?.(booking)}
      className="w-full text-left bg-[var(--color-surface)] rounded-xl p-4 border border-[var(--color-border)] hover:shadow-md transition-shadow"
      data-testid="booking-card"
    >
      <div className="flex items-start gap-3">
        {/* Type icon badge */}
        <div
          className={`flex-shrink-0 w-10 h-10 rounded-lg flex items-center justify-center ${typeColor}`}
          data-testid="booking-type-icon"
        >
          <Icon size={20} />
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0">
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <h3 className="font-semibold text-[var(--color-text-primary)] truncate">
                {booking.title || typeLabel}
              </h3>
              {subtitle && (
                <p className="text-sm text-[var(--color-text-secondary)] truncate mt-0.5">
                  {subtitle}
                </p>
              )}
            </div>
            <span
              className={`flex-shrink-0 text-xs px-2 py-1 rounded-full font-medium ${typeColor}`}
            >
              {typeLabel}
            </span>
          </div>

          {/* Meta row */}
          <div className="flex items-center gap-3 mt-2 text-xs text-[var(--color-text-tertiary)]">
            {dateRange && (
              <span className="flex items-center gap-1">
                <Calendar size={12} />
                {dateRange}
              </span>
            )}
            {booking.confirmationCode && (
              <span className="flex items-center gap-1">
                <Hash size={12} />
                {booking.confirmationCode}
              </span>
            )}
          </div>
        </div>
      </div>
    </button>
  );
}
