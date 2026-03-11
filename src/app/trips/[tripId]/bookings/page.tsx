"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Plus, FileText, PenLine } from "lucide-react";
import { useBookings, useDeleteBooking } from "@/lib/hooks/useBookings";
import { BookingList } from "@/components/booking/BookingList";
import { BookingDetail } from "@/components/booking/BookingDetail";
import { BookingUpload } from "@/components/booking/BookingUpload";
import { BookingForm } from "@/components/booking/BookingForm";
import type { Booking } from "@/gen/toqui/v1/booking_pb";

type ViewMode = "list" | "detail" | "upload" | "manual";

export default function BookingsPage() {
  const { tripId } = useParams<{ tripId: string }>();
  const { bookings, isLoading } = useBookings(tripId);
  const deleteBooking = useDeleteBooking();

  const [viewMode, setViewMode] = useState<ViewMode>("list");
  const [selectedBooking, setSelectedBooking] = useState<Booking | null>(null);

  const handleBookingClick = (booking: Booking) => {
    setSelectedBooking(booking);
    setViewMode("detail");
  };

  const handleBackToList = () => {
    setSelectedBooking(null);
    setViewMode("list");
  };

  const handleDelete = async (booking: Booking) => {
    const confirmed = window.confirm(
      `Are you sure you want to delete "${booking.title || "this booking"}"? This action cannot be undone.`,
    );
    if (!confirmed) return;

    try {
      await deleteBooking.mutateAsync({ id: booking.id, tripId });
      handleBackToList();
    } catch (err) {
      console.error("Failed to delete booking:", err);
    }
  };

  return (
    <div className="min-h-screen bg-[var(--color-surface-secondary)]">
      <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)] px-4 py-4">
        <div className="max-w-4xl mx-auto">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Link
                href={`/trips/${tripId}`}
                className="text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] rounded"
                aria-label="Back to trip"
              >
                <ArrowLeft size={20} aria-hidden="true" />
              </Link>
              <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">Bookings</h1>
              {!isLoading && bookings.length > 0 && (
                <span className="text-sm text-[var(--color-text-tertiary)]">
                  ({bookings.length})
                </span>
              )}
            </div>

            {/* Action buttons - only show in list mode */}
            {viewMode === "list" && (
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setViewMode("upload")}
                  className="flex items-center gap-1.5 text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] px-3 py-2 rounded-lg hover:bg-[var(--color-surface-tertiary)] transition-colors"
                >
                  <FileText size={16} aria-hidden="true" />
                  <span className="hidden sm:inline">Paste</span>
                </button>
                <button
                  type="button"
                  onClick={() => setViewMode("manual")}
                  className="flex items-center gap-1.5 bg-[var(--color-accent)] text-white px-3 py-2 rounded-lg text-sm font-medium hover:bg-[var(--color-accent-hover)] transition-colors"
                >
                  <Plus size={16} aria-hidden="true" />
                  <span className="hidden sm:inline">Add Booking</span>
                </button>
              </div>
            )}
          </div>
        </div>
      </header>

      <main id="main-content" className="max-w-4xl mx-auto p-4">
        {viewMode === "list" && (
          <BookingList
            bookings={bookings}
            isLoading={isLoading}
            onBookingClick={handleBookingClick}
          />
        )}

        {viewMode === "detail" && selectedBooking && (
          <BookingDetail
            booking={selectedBooking}
            onBack={handleBackToList}
            onDelete={handleDelete}
            isDeleting={deleteBooking.isPending}
          />
        )}

        {viewMode === "upload" && (
          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={handleBackToList}
                className="text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)] transition-colors"
                aria-label="Back to bookings"
              >
                <ArrowLeft size={20} />
              </button>
              <h2 className="font-semibold text-lg text-[var(--color-text-primary)]">
                Upload Booking
              </h2>
            </div>
            <BookingUpload tripId={tripId} onSuccess={handleBackToList} />
          </div>
        )}

        {viewMode === "manual" && (
          <div className="space-y-4">
            <div className="flex items-center gap-3 mb-2">
              <button
                type="button"
                onClick={handleBackToList}
                className="text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)] transition-colors"
                aria-label="Back to bookings"
              >
                <ArrowLeft size={20} />
              </button>
              <h2 className="font-semibold text-lg flex items-center gap-2 text-[var(--color-text-primary)]">
                <PenLine size={18} aria-hidden="true" />
                Manual Entry
              </h2>
            </div>
            <BookingForm tripId={tripId} onClose={handleBackToList} onSuccess={handleBackToList} />
          </div>
        )}
      </main>
    </div>
  );
}
