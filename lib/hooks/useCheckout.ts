import { useState, useCallback } from "react";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

interface CheckoutInitResponse {
  checkoutToken: string;
  secretToken: string;
  priceCents: number;
  currency: string;
}

interface CheckoutValidateResponse {
  unlocked: boolean;
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

  const initCheckout = useCallback(async (): Promise<CheckoutInitResponse> => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/checkout`,
        accessToken,
        {
          method: "POST",
          body: JSON.stringify({ trip_id: tripId }),
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

  const validatePayment = useCallback(
    async (response: string, hash: string): Promise<CheckoutValidateResponse> => {
      setIsLoading(true);
      setError(null);
      try {
        const res = await authFetch(
          `${getConfig().apiUrl}/api/checkout/validate`,
          accessToken,
          {
            method: "POST",
            body: JSON.stringify({ trip_id: tripId, response, hash }),
          },
        );
        if (!res.ok) {
          throw new Error(`Payment validation failed: ${res.status}`);
        }
        return (await res.json()) as CheckoutValidateResponse;
      } catch (err) {
        const message = err instanceof Error ? err.message : "Validation failed";
        setError(message);
        throw err;
      } finally {
        setIsLoading(false);
      }
    },
    [tripId, accessToken],
  );

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

  return { initCheckout, validatePayment, checkStatus, isLoading, error };
}
