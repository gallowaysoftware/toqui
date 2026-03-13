"use client";

import { useState } from "react";
import { Upload, FileText, CheckCircle, AlertCircle } from "lucide-react";
import { BookingType } from "@/gen/toqui/v1/booking_pb";
import { useIngestBooking } from "@/lib/hooks/useBookings";
import { bookingTypeLabels } from "@/lib/booking-utils";

const typeOptions = [
  BookingType.UNSPECIFIED,
  BookingType.FLIGHT,
  BookingType.HOTEL,
  BookingType.CAR_RENTAL,
  BookingType.TRAIN,
  BookingType.ACTIVITY,
  BookingType.RESTAURANT,
  BookingType.TOUR,
  BookingType.OTHER,
] as const;

interface BookingUploadProps {
  tripId: string;
  onSuccess?: () => void;
}

export function BookingUpload({ tripId, onSuccess }: BookingUploadProps) {
  const [text, setText] = useState("");
  const [bookingType, setBookingType] = useState<BookingType>(BookingType.UNSPECIFIED);
  const [successMessage, setSuccessMessage] = useState("");
  const [errorMessage, setErrorMessage] = useState("");

  const ingestBooking = useIngestBooking();

  const handleSubmit = async () => {
    if (!text.trim()) return;
    setSuccessMessage("");
    setErrorMessage("");

    try {
      const result = await ingestBooking.mutateAsync({
        tripId,
        type: bookingType,
        rawText: text,
      });
      setText("");
      setBookingType(BookingType.UNSPECIFIED);
      setSuccessMessage(
        result?.title
          ? `Booking "${result.title}" created successfully.`
          : "Booking created successfully.",
      );
      onSuccess?.();
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "Failed to process booking text.");
    }
  };

  return (
    <div className="space-y-6">
      <div className="bg-[var(--color-surface)] rounded-xl p-6 border border-[var(--color-border)]">
        <h2 className="font-semibold mb-4 flex items-center gap-2 text-[var(--color-text-primary)]">
          <FileText size={18} aria-hidden="true" />
          Paste Booking Confirmation
        </h2>

        {/* Optional type hint */}
        <div className="mb-3">
          <label
            htmlFor="upload-type"
            className="block text-sm text-[var(--color-text-secondary)] mb-1"
          >
            Booking type (optional hint for AI)
          </label>
          <select
            id="upload-type"
            value={bookingType}
            onChange={(e) => setBookingType(Number(e.target.value) as BookingType)}
            className="w-full sm:w-auto rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-3 py-2 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
          >
            {typeOptions.map((t) => (
              <option key={t} value={t}>
                {t === BookingType.UNSPECIFIED ? "Auto-detect" : bookingTypeLabels[t]}
              </option>
            ))}
          </select>
        </div>

        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="Paste your booking confirmation email or text here. Our AI will extract the details automatically."
          aria-label="Booking confirmation text"
          rows={8}
          className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-4 py-3 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent resize-none"
        />

        {/* Status messages */}
        {successMessage && (
          <div
            className="mt-3 flex items-center gap-2 text-sm text-[var(--color-success)] bg-[var(--color-success-bg)] rounded-lg px-3 py-2"
            role="status"
            aria-live="polite"
          >
            <CheckCircle size={16} aria-hidden="true" />
            {successMessage}
          </div>
        )}
        {errorMessage && (
          <div
            className="mt-3 flex items-center gap-2 text-sm text-[var(--color-error)] bg-[var(--color-error-bg)] rounded-lg px-3 py-2"
            role="alert"
          >
            <AlertCircle size={16} aria-hidden="true" />
            {errorMessage}
          </div>
        )}

        <button
          onClick={handleSubmit}
          disabled={!text.trim() || ingestBooking.isPending}
          className="mt-3 bg-[var(--color-accent)] text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-[var(--color-accent-hover)] transition-colors disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2"
        >
          {ingestBooking.isPending ? "Processing..." : "Extract Booking"}
        </button>
      </div>

      <div className="bg-[var(--color-surface)] rounded-xl p-6 border border-[var(--color-border)]">
        <h2 className="font-semibold mb-4 flex items-center gap-2 text-[var(--color-text-primary)]">
          <Upload size={18} aria-hidden="true" />
          Forward Booking Emails
        </h2>
        <p className="text-sm text-[var(--color-text-secondary)]">
          Forward your booking confirmation emails to{" "}
          <span className="font-mono bg-[var(--color-surface-tertiary)] px-2 py-1 rounded text-[var(--color-accent)]">
            trips@toqui.travel
          </span>{" "}
          and they will be automatically added to your trip.
        </p>
      </div>
    </div>
  );
}
