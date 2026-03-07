import { Info } from "lucide-react";

export function AffiliateDisclosure() {
  return (
    <div className="flex items-start gap-1.5 mt-1.5 ml-1 px-1" role="note" aria-label="Affiliate disclosure">
      <Info size={12} className="text-[var(--color-text-tertiary)] mt-0.5 flex-shrink-0" aria-hidden="true" />
      <p className="text-[11px] text-[var(--color-text-tertiary)] leading-relaxed">
        Toqui may earn a commission from bookings made through these links at no extra cost to you.
      </p>
    </div>
  );
}
