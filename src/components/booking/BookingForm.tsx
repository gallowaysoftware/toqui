"use client";

import { useState } from "react";
import { X } from "lucide-react";
import { BookingType } from "@/gen/toqui/v1/booking_pb";
import { useIngestBooking } from "@/lib/hooks/useBookings";
import { bookingTypeLabels } from "@/lib/booking-utils";

/** Booking types available for manual creation (excludes UNSPECIFIED). */
const selectableTypes = [
  BookingType.FLIGHT,
  BookingType.HOTEL,
  BookingType.CAR_RENTAL,
  BookingType.TRAIN,
  BookingType.ACTIVITY,
  BookingType.RESTAURANT,
  BookingType.TOUR,
  BookingType.OTHER,
] as const;

interface BookingFormProps {
  tripId: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export function BookingForm({ tripId, onClose, onSuccess }: BookingFormProps) {
  const [bookingType, setBookingType] = useState<BookingType>(
    BookingType.FLIGHT,
  );
  const [title, setTitle] = useState("");
  const [provider, setProvider] = useState("");
  const [confirmationCode, setConfirmationCode] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [address, setAddress] = useState("");
  const [notes, setNotes] = useState("");
  const [formError, setFormError] = useState("");

  const ingestBooking = useIngestBooking();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError("");

    if (!title.trim()) {
      setFormError("Title is required.");
      return;
    }

    // Build a structured text representation for the AI to parse
    const lines = [
      `Booking Type: ${bookingTypeLabels[bookingType]}`,
      `Title: ${title}`,
      provider && `Provider: ${provider}`,
      confirmationCode && `Confirmation Code: ${confirmationCode}`,
      startDate && `Start Date: ${startDate}`,
      endDate && `End Date: ${endDate}`,
      address && `Address: ${address}`,
      notes && `Notes: ${notes}`,
    ]
      .filter(Boolean)
      .join("\n");

    try {
      await ingestBooking.mutateAsync({
        tripId,
        type: bookingType,
        rawText: lines,
      });
      onSuccess?.();
      onClose();
    } catch (err) {
      setFormError(
        err instanceof Error ? err.message : "Failed to create booking.",
      );
    }
  };

  return (
    <div className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-6">
      <div className="flex items-center justify-between mb-4">
        <h2 className="font-semibold text-lg text-[var(--color-text-primary)]">Add Booking Manually</h2>
        <button
          type="button"
          onClick={onClose}
          className="text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)] transition-colors"
          aria-label="Close form"
        >
          <X size={20} />
        </button>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        {/* Booking Type */}
        <div>
          <label
            htmlFor="booking-type"
            className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
          >
            Type
          </label>
          <select
            id="booking-type"
            value={bookingType}
            onChange={(e) => setBookingType(Number(e.target.value) as BookingType)}
            className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
          >
            {selectableTypes.map((t) => (
              <option key={t} value={t}>
                {bookingTypeLabels[t]}
              </option>
            ))}
          </select>
        </div>

        {/* Title */}
        <div>
          <label
            htmlFor="booking-title"
            className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
          >
            Title <span className="text-[var(--color-error)]">*</span>
          </label>
          <input
            id="booking-title"
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="e.g. Flight to Paris, Hotel Marriott..."
            className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
          />
        </div>

        {/* Provider */}
        <div>
          <label
            htmlFor="booking-provider"
            className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
          >
            Provider
          </label>
          <input
            id="booking-provider"
            type="text"
            value={provider}
            onChange={(e) => setProvider(e.target.value)}
            placeholder="e.g. Air France, Hilton, Enterprise..."
            className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
          />
        </div>

        {/* Confirmation Code */}
        <div>
          <label
            htmlFor="booking-confirmation"
            className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
          >
            Confirmation Code
          </label>
          <input
            id="booking-confirmation"
            type="text"
            value={confirmationCode}
            onChange={(e) => setConfirmationCode(e.target.value)}
            placeholder="e.g. ABC123"
            className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
          />
        </div>

        {/* Date Row */}
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label
              htmlFor="booking-start"
              className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
            >
              Start Date
            </label>
            <input
              id="booking-start"
              type="date"
              value={startDate}
              onChange={(e) => setStartDate(e.target.value)}
              className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
            />
          </div>
          <div>
            <label
              htmlFor="booking-end"
              className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
            >
              End Date
            </label>
            <input
              id="booking-end"
              type="date"
              value={endDate}
              onChange={(e) => setEndDate(e.target.value)}
              className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
            />
          </div>
        </div>

        {/* Address */}
        <div>
          <label
            htmlFor="booking-address"
            className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
          >
            Address / Location
          </label>
          <input
            id="booking-address"
            type="text"
            value={address}
            onChange={(e) => setAddress(e.target.value)}
            placeholder="e.g. 123 Main St, Paris, France"
            className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
          />
        </div>

        {/* Notes */}
        <div>
          <label
            htmlFor="booking-notes"
            className="block text-sm font-medium text-[var(--color-text-secondary)] mb-1"
          >
            Additional Notes
          </label>
          <textarea
            id="booking-notes"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            placeholder="Any other details..."
            rows={3}
            className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent resize-none"
          />
        </div>

        {/* Error */}
        {formError && (
          <p className="text-sm text-[var(--color-error)]" role="alert">
            {formError}
          </p>
        )}

        {/* Actions */}
        <div className="flex items-center justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={ingestBooking.isPending}
            className="bg-[var(--color-accent)] text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-[var(--color-accent-hover)] transition-colors disabled:opacity-50"
          >
            {ingestBooking.isPending ? "Creating..." : "Create Booking"}
          </button>
        </div>
      </form>
    </div>
  );
}
