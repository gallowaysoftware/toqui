"use client";

import { useParams } from "next/navigation";
import { BookingUpload } from "@/components/booking/BookingUpload";

export default function BookingsPage() {
  const { tripId } = useParams<{ tripId: string }>();

  return (
    <div className="min-h-screen bg-[var(--color-surface-secondary)]">
      <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)] px-4 py-4">
        <div className="max-w-4xl mx-auto">
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">Bookings</h1>
        </div>
      </header>
      <main className="max-w-4xl mx-auto p-4">
        <BookingUpload tripId={tripId} />
      </main>
    </div>
  );
}
