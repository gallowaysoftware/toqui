"use client";

import { useState } from "react";
import { Crown } from "lucide-react";
import { useTripUnlockStatus } from "@/lib/hooks/usePayment";
import { TripProModal } from "./TripProModal";

interface TripProBannerProps {
  tripId: string;
}

export function TripProBanner({ tripId }: TripProBannerProps) {
  const [showModal, setShowModal] = useState(false);
  const { data: status } = useTripUnlockStatus(tripId);

  // Don't show banner if trip is already unlocked or status unknown
  if (!status || status.unlocked) return null;

  return (
    <>
      <button
        onClick={() => setShowModal(true)}
        className="flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium bg-[var(--color-accent-soft)] text-[var(--color-accent)] hover:bg-[var(--color-accent)] hover:text-white transition-colors"
      >
        <Crown size={14} />
        Upgrade
      </button>

      <TripProModal
        tripId={tripId}
        isOpen={showModal}
        onClose={() => setShowModal(false)}
        onSuccess={() => setShowModal(false)}
      />
    </>
  );
}
