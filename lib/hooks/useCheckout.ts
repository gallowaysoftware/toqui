import { useState, useCallback } from "react";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

interface CheckoutInitResponse {
  url: string;
}

interface CheckoutStatusResponse {
  unlocked: boolean;
  priceCents: number;
  currency: string;
}

export function useCheckout(tripId: string) {
  const { accessToken } = useAuth();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // NOTE: price_variant is sent for analytics/logging only.
  // The backend MUST determine the actual charge amount server-side
  // (e.g., by evaluating PostHog flags server-side or from its own config).
  // Never trust this client-supplied value to set the payment amount.
  const initCheckout = useCallback(async (priceVariant?: string): Promise<CheckoutInitResponse> => {
    setIsLoading(true);
    setError(null);
    try {
      const body: Record<string, string> = { trip_id: tripId };
      if (priceVariant) body.price_variant = priceVariant;
      const res = await authFetch(
        `${getConfig().apiUrl}/api/checkout`,
        accessToken,
        {
          method: "POST",
          body: JSON.stringify(body),
        },
      );
      if (!res.ok) {
        throw new Error(`Checkout init failed: ${res.status}`);
      }
      return (await res.json()) as CheckoutInitResponse;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Checkout failed";
      setError(message);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [tripId, accessToken]);

  const checkStatus = useCallback(async (): Promise<CheckoutStatusResponse> => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/checkout/status?trip_id=${encodeURIComponent(tripId)}`,
        accessToken,
      );
      if (!res.ok) {
        throw new Error(`Status check failed: ${res.status}`);
      }
      return (await res.json()) as CheckoutStatusResponse;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Status check failed";
      setError(message);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, [tripId, accessToken]);

  return { initCheckout, checkStatus, isLoading, error };
}
