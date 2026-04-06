import { useState, useEffect, useCallback } from "react";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

/** Raw JSON from GET /api/referral (Go uses snake_case keys). */
interface ReferralResponse {
  code: string;
  link: string;
  successful_referrals: number;
  rewards_earned: number;
  rewards_remaining: number;
  max_rewards: number;
}

export function useReferral() {
  const { accessToken } = useAuth();
  const [data, setData] = useState<ReferralResponse | null>(null);
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
          `${getConfig().apiUrl}/api/referral`,
          accessToken,
        );
        if (!res.ok) {
          throw new Error(`Failed to fetch referral data: ${res.status}`);
        }
        const json = (await res.json()) as ReferralResponse;
        if (!cancelled) {
          setData(json);
          setError(null);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load referral data");
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

  const redeemCode = useCallback(
    async (code: string) => {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/referral/redeem`,
        accessToken,
        {
          method: "POST",
          body: JSON.stringify({ code }),
        },
      );
      if (!res.ok) {
        throw new Error(`Failed to redeem code: ${res.status}`);
      }
    },
    [accessToken],
  );

  return {
    code: data?.code ?? null,
    link: data?.link ?? null,
    successfulReferrals: data?.successful_referrals ?? 0,
    rewardsEarned: data?.rewards_earned ?? 0,
    rewardsRemaining: data?.rewards_remaining ?? 0,
    maxRewards: data?.max_rewards ?? 10,
    isLoading,
    error,
    redeemCode,
  };
}
