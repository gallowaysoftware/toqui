import { useState, useEffect, useCallback } from "react";
import { Linking } from "react-native";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

export type SubscriptionTier = "free" | "pro" | "explorer" | "voyager";
export type SubscriptionStatus = "active" | "past_due" | "canceled" | "inactive";
export type BillingPeriod = "monthly" | "annual";

export interface Subscription {
  tier: SubscriptionTier;
  status: SubscriptionStatus;
  billingPeriod: BillingPeriod | null;
  currentPeriodEnd: Date | null;
  cancelAtPeriodEnd: boolean;
}

interface SubscriptionResponse {
  tier: SubscriptionTier;
  status: SubscriptionStatus;
  billing_period: BillingPeriod | null;
  current_period_end: string | null;
  cancel_at_period_end: boolean;
}

interface CheckoutResponse {
  url: string;
}

interface PortalResponse {
  url: string;
}

export function useSubscription() {
  const { accessToken } = useAuth();
  const [subscription, setSubscription] = useState<Subscription | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!accessToken) {
      setIsLoading(false);
      return;
    }

    let cancelled = false;

    (async () => {
      try {
        const res = await authFetch(
          `${getConfig().apiUrl}/api/subscription`,
          accessToken,
        );
        if (!res.ok) {
          throw new Error(`Failed to fetch subscription: ${res.status}`);
        }
        const json = (await res.json()) as SubscriptionResponse;
        if (!cancelled) {
          setSubscription({
            tier: json.tier,
            status: json.status,
            billingPeriod: json.billing_period ?? null,
            currentPeriodEnd: json.current_period_end
              ? new Date(json.current_period_end)
              : null,
            cancelAtPeriodEnd: json.cancel_at_period_end,
          });
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(
            err instanceof Error ? err.message : "Failed to load subscription",
          );
        }
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [accessToken]);

  const subscribe = useCallback(
    async (tier: "explorer" | "voyager", annual: boolean) => {
      const billingPeriod: BillingPeriod = annual ? "annual" : "monthly";
      setError(null);
      try {
        const res = await authFetch(
          `${getConfig().apiUrl}/api/subscription/checkout`,
          accessToken,
          {
            method: "POST",
            body: JSON.stringify({ tier, billing_period: billingPeriod }),
          },
        );
        if (!res.ok) {
          throw new Error(`Subscription checkout failed: ${res.status}`);
        }
        const json = (await res.json()) as CheckoutResponse;
        await Linking.openURL(json.url);
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Subscription checkout failed";
        setError(message);
        throw err;
      }
    },
    [accessToken],
  );

  const cancel = useCallback(async () => {
    setError(null);
    try {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/subscription/cancel`,
        accessToken,
        { method: "POST" },
      );
      if (!res.ok) {
        throw new Error(`Cancellation failed: ${res.status}`);
      }
      // Update local state to reflect cancellation
      setSubscription((prev) =>
        prev ? { ...prev, cancelAtPeriodEnd: true } : prev,
      );
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Cancellation failed";
      setError(message);
      throw err;
    }
  }, [accessToken]);

  const manageSubscription = useCallback(async () => {
    setError(null);
    try {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/subscription/portal`,
        accessToken,
        { method: "POST" },
      );
      if (!res.ok) {
        throw new Error(`Failed to open billing portal: ${res.status}`);
      }
      const json = (await res.json()) as PortalResponse;
      await Linking.openURL(json.url);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to open billing portal";
      setError(message);
      throw err;
    }
  }, [accessToken]);

  return {
    subscription,
    isLoading,
    error,
    subscribe,
    cancel,
    manageSubscription,
  };
}
