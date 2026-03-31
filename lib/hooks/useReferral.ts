import { useState, useEffect, useCallback } from "react";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

interface ReferralData {
  code: string;
  link: string;
  successfulReferrals: number;
  rewardsEarned: number;
}

export function useReferral() {
  const { accessToken } = useAuth();
  const [data, setData] = useState<ReferralData | null>(null);
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
        const json = (await res.json()) as ReferralData;
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
    successfulReferrals: data?.successfulReferrals ?? 0,
    rewardsEarned: data?.rewardsEarned ?? 0,
    isLoading,
    error,
    redeemCode,
  };
}
