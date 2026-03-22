"use client";

import { useQuery } from "@tanstack/react-query";
import { Crown, Receipt } from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

interface PaymentRecord {
  id: string;
  trip_title: string;
  amount_cents: number;
  currency: string;
  created_at: string;
}

interface BillingData {
  payments: PaymentRecord[];
  total_count: number;
}

export function BillingSection() {
  const { data } = useQuery<BillingData>({
    queryKey: ["billing"],
    queryFn: async () => {
      const res = await fetch(`${API_URL}/api/usage`, {
        credentials: "include",
      });
      if (!res.ok) return { payments: [], total_count: 0 };
      return res.json();
    },
    // Billing data doesn't change often
    staleTime: 60_000,
  });

  const payments = data?.payments ?? [];

  return (
    <div className="bg-[var(--color-surface)] rounded-xl border border-[var(--color-border)] p-6">
      <h2 className="text-sm font-medium text-[var(--color-text-secondary)] mb-4 flex items-center gap-2">
        <Crown size={16} />
        Billing
      </h2>

      {payments.length === 0 ? (
        <p className="text-sm text-[var(--color-text-tertiary)]">
          No purchases yet. Trip Pro unlocks are $12 CAD per trip.
        </p>
      ) : (
        <div className="space-y-3">
          {payments.map((p) => (
            <div
              key={p.id}
              className="flex items-center justify-between py-2 border-b border-[var(--color-border)] last:border-0"
            >
              <div className="flex items-center gap-2">
                <Receipt size={14} className="text-[var(--color-text-tertiary)]" />
                <div>
                  <p className="text-sm text-[var(--color-text-primary)]">
                    Trip Pro — {p.trip_title || "Untitled Trip"}
                  </p>
                  <p className="text-xs text-[var(--color-text-tertiary)]">
                    {new Date(p.created_at).toLocaleDateString("en-US", {
                      month: "short",
                      day: "numeric",
                      year: "numeric",
                    })}
                  </p>
                </div>
              </div>
              <span className="text-sm font-medium text-[var(--color-text-primary)]">
                ${(p.amount_cents / 100).toFixed(2)} {p.currency}
              </span>
            </div>
          ))}
        </div>
      )}

      <p className="text-xs text-[var(--color-text-tertiary)] mt-4">
        Payments processed by Helcim. For refund requests, contact{" "}
        <a
          href="mailto:support@toqui.travel"
          className="text-[var(--color-accent)] hover:underline"
        >
          support@toqui.travel
        </a>
        .
      </p>
    </div>
  );
}
