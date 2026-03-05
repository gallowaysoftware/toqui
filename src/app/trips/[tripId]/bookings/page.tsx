"use client";

import { useParams } from "next/navigation";
import { BookingUpload } from "@/components/booking/BookingUpload";

export default function BookingsPage() {
  const { tripId } = useParams<{ tripId: string }>();

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200 px-4 py-4">
        <div className="max-w-4xl mx-auto">
          <h1 className="text-xl font-semibold">Bookings</h1>
        </div>
      </header>
      <main className="max-w-4xl mx-auto p-4">
        <BookingUpload tripId={tripId} />
      </main>
    </div>
  );
}
