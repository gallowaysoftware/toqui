"use client";

import { memo } from "react";
import { ExternalLink, Plane, Hotel, Ticket } from "lucide-react";
import { AffiliateDisclosure } from "./AffiliateDisclosure";

export interface Recommendation {
  partner: string;
  category: string;
  title: string;
  description: string;
  url: string;
  price?: string;
  disclosure?: string;
}

interface RecommendationCardProps {
  recommendation: Recommendation;
}

const partnerConfig: Record<string, { label: string; icon: typeof Plane }> = {
  skyscanner: { label: "Skyscanner", icon: Plane },
  "booking.com": { label: "Booking.com", icon: Hotel },
  bookingcom: { label: "Booking.com", icon: Hotel },
  getyourguide: { label: "GetYourGuide", icon: Ticket },
  viator: { label: "Viator", icon: Ticket },
};

const categoryConfig: Record<string, { label: string; icon: typeof Plane }> = {
  flight: { label: "Flight", icon: Plane },
  hotel: { label: "Hotel", icon: Hotel },
  activity: { label: "Activity", icon: Ticket },
};

function getPartnerInfo(partner: string, category: string) {
  const key = partner.toLowerCase().replace(/[\s.]+/g, "");
  const partnerInfo = partnerConfig[key];
  const categoryInfo = categoryConfig[category.toLowerCase()];

  return {
    label: partnerInfo?.label ?? partner,
    Icon: partnerInfo?.icon ?? categoryInfo?.icon ?? ExternalLink,
  };
}

export const RecommendationCard = memo(function RecommendationCard({ recommendation }: RecommendationCardProps) {
  const { partner, category, title, description, url, price } = recommendation;
  const { label: partnerLabel, Icon } = getPartnerInfo(partner, category);

  return (
    <div className="flex justify-start">
      <div className="max-w-[85%] w-full">
        <div
          className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] overflow-hidden shadow-sm"
          role="article"
          aria-label={`${partnerLabel} recommendation: ${title}`}
        >
          {/* Header */}
          <div className="flex items-center gap-2 px-4 py-2.5 bg-[var(--color-surface-tertiary)] border-b border-[var(--color-border)]">
            <Icon
              size={16}
              className="text-[var(--color-accent)] flex-shrink-0"
              aria-hidden="true"
            />
            <span className="text-xs font-semibold text-[var(--color-text-secondary)] uppercase tracking-wide">
              {partnerLabel}
            </span>
            <span className="text-xs text-[var(--color-text-tertiary)]">
              {categoryConfig[category.toLowerCase()]?.label ?? category}
            </span>
          </div>

          {/* Body */}
          <div className="px-4 py-3 space-y-2">
            <p className="text-sm font-semibold text-[var(--color-text-primary)] leading-snug" role="heading" aria-level={3}>
              {title}
            </p>
            <p className="text-xs text-[var(--color-text-secondary)] leading-relaxed">
              {description}
            </p>
            {price && <p className="text-sm font-semibold text-[var(--color-success)]">{price}</p>}
          </div>

          {/* CTA */}
          <div className="px-4 pb-3">
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer sponsored"
              className="inline-flex items-center gap-1.5 px-4 py-2 rounded-lg bg-[var(--color-accent)] text-white text-sm font-medium hover:bg-[var(--color-accent-hover)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2"
            >
              View on {partnerLabel}
              <ExternalLink size={14} aria-hidden="true" />
            </a>
          </div>
        </div>

        <AffiliateDisclosure />
      </div>
    </div>
  );
});
