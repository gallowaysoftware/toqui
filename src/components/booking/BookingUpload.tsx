"use client";

import { useState } from "react";
import { Upload, FileText } from "lucide-react";

interface BookingUploadProps {
  tripId: string;
}

export function BookingUpload({ tripId }: BookingUploadProps) {
  const [text, setText] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async () => {
    if (!text.trim()) return;
    setIsSubmitting(true);
    // TODO: Call IngestBooking RPC with tripId
    console.log("Ingesting booking for trip:", tripId);
    setIsSubmitting(false);
    setText("");
  };

  return (
    <div className="space-y-6">
      <div className="bg-[var(--color-surface)] rounded-xl p-6 border border-[var(--color-border)]">
        <h2 className="font-semibold mb-4 flex items-center gap-2 text-[var(--color-text-primary)]">
          <FileText size={18} />
          Paste Booking Confirmation
        </h2>
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="Paste your booking confirmation email or text here. Our AI will extract the details automatically."
          rows={8}
          className="w-full rounded-lg border border-[var(--color-input-border)] bg-[var(--color-input-bg)] px-4 py-3 text-sm text-[var(--color-text-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent resize-none"
        />
        <button
          onClick={handleSubmit}
          disabled={!text.trim() || isSubmitting}
          className="mt-3 bg-[var(--color-accent)] text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-[var(--color-accent-hover)] transition-colors disabled:opacity-50"
        >
          {isSubmitting ? "Processing..." : "Extract Booking"}
        </button>
      </div>

      <div className="bg-[var(--color-surface)] rounded-xl p-6 border border-[var(--color-border)]">
        <h2 className="font-semibold mb-4 flex items-center gap-2 text-[var(--color-text-primary)]">
          <Upload size={18} />
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
