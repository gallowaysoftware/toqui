"use client";

import { Briefcase } from "lucide-react";
import type { Booking } from "@/gen/toqui/v1/booking_pb";
import { BookingCard } from "./BookingCard";

interface BookingListProps {
  bookings: Booking[];
  isLoading: boolean;
  onBookingClick?: (booking: Booking) => void;
}

function BookingListSkeleton() {
  return (
    <div className="space-y-3" data-testid="booking-list-skeleton">
      {[1, 2, 3].map((i) => (
        <div
          key={i}
          className="bg-[var(--color-surface)] rounded-xl p-4 border border-[var(--color-border)] animate-pulse"
        >
          <div className="flex items-start gap-3">
            <div className="w-10 h-10 rounded-lg bg-[var(--color-surface-tertiary)]" />
            <div className="flex-1">
              <div className="h-4 bg-[var(--color-surface-tertiary)] rounded w-1/3 mb-2" />
              <div className="h-3 bg-[var(--color-surface-tertiary)] rounded w-1/2 mb-3" />
              <div className="h-3 bg-[var(--color-surface-tertiary)] rounded w-1/4" />
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}

function BookingListEmpty() {
  return (
    <div
      className="bg-[var(--color-surface)] rounded-xl p-8 border border-[var(--color-border)] text-center"
      data-testid="booking-list-empty"
    >
      <Briefcase className="mx-auto text-[var(--color-text-tertiary)] mb-3" size={40} />
      <h3 className="font-semibold text-[var(--color-text-secondary)] mb-1">No bookings yet</h3>
      <p className="text-sm text-[var(--color-text-secondary)]">
        Add your first booking by pasting a confirmation email or creating one
        manually.
      </p>
    </div>
  );
}

export function BookingList({
  bookings,
  isLoading,
  onBookingClick,
}: BookingListProps) {
  if (isLoading) {
    return <BookingListSkeleton />;
  }

  if (bookings.length === 0) {
    return <BookingListEmpty />;
  }

  return (
    <div className="space-y-3" data-testid="booking-list">
      {bookings.map((booking) => (
        <BookingCard
          key={booking.id}
          booking={booking}
          onClick={onBookingClick}
        />
      ))}
    </div>
  );
}
