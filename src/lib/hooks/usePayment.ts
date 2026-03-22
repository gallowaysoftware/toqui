"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

export interface CheckoutResult {
  checkout_token: string;
  secret_token: string;
  price_cents: number;
  currency: string;
}

export interface UnlockStatus {
  unlocked: boolean;
  price_cents: number;
  currency: string;
}

export function useTripUnlockStatus(tripId: string | undefined) {
  return useQuery<UnlockStatus>({
    queryKey: ["trip-unlock", tripId],
    queryFn: async () => {
      const res = await fetch(
        `${API_URL}/api/checkout/status?trip_id=${encodeURIComponent(tripId!)}`,
        { credentials: "include" },
      );
      if (!res.ok) throw new Error(`Failed to check unlock status (${res.status})`);
      return res.json();
    },
    enabled: !!tripId,
  });
}

export function useCreateCheckout() {
  return useMutation<CheckoutResult, Error, { tripId: string }>({
    mutationFn: async ({ tripId }) => {
      const res = await fetch(`${API_URL}/api/checkout`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({ trip_id: tripId }),
      });
      if (!res.ok) {
        const body = await res.text().catch(() => "");
        throw new Error(body || `Checkout failed (${res.status})`);
      }
      return res.json();
    },
  });
}

export function useValidatePayment() {
  const queryClient = useQueryClient();

  return useMutation<{ status: string }, Error, {
    checkoutToken: string;
    responseData: unknown;
    responseHash: string;
    tripId: string;
  }>({
    mutationFn: async ({ checkoutToken, responseData, responseHash }) => {
      const res = await fetch(`${API_URL}/api/checkout/validate`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({
          checkout_token: checkoutToken,
          response_data: responseData,
          response_hash: responseHash,
        }),
      });
      if (!res.ok) {
        const body = await res.text().catch(() => "");
        throw new Error(body || `Payment validation failed (${res.status})`);
      }
      return res.json();
    },
    onSuccess: (_, variables) => {
      void queryClient.invalidateQueries({ queryKey: ["trip-unlock", variables.tripId] });
    },
  });
}
