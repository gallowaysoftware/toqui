import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth";
import { getConfig } from "@/lib/config";

interface TrialStatus {
  isTrialActive: boolean;
  isTrialExpired: boolean;
  daysRemaining: number | null;
  isLastDay: boolean;
  isLoading: boolean;
}

interface CheckoutStatusResponse {
  unlocked: boolean;
  trial_active?: boolean;
  trial_end?: string;
}

const NO_TRIAL: TrialStatus = {
  isTrialActive: false,
  isTrialExpired: false,
  daysRemaining: null,
  isLastDay: false,
  isLoading: false,
};

export function useTrialStatus(tripId: string): TrialStatus {
  const { accessToken } = useAuth();

  const { data, isLoading } = useQuery({
    queryKey: ["trialStatus", tripId],
    queryFn: async (): Promise<CheckoutStatusResponse> => {
      const res = await fetch(
        `${getConfig().apiUrl}/api/checkout/status?trip_id=${encodeURIComponent(tripId)}`,
        { headers: { Authorization: `Bearer ${accessToken}` } },
      );
      if (!res.ok) return { unlocked: false };
      return res.json();
    },
    enabled: !!accessToken && !!tripId,
    staleTime: 60_000,
    retry: false,
  });

  if (isLoading || !data) {
    return { ...NO_TRIAL, isLoading };
  }

  if (!data.trial_end) {
    if (!data.trial_active) return NO_TRIAL;
    return { ...NO_TRIAL, isTrialActive: true };
  }

  const msRemaining = new Date(data.trial_end).getTime() - Date.now();

  if (msRemaining <= 0) {
    return { ...NO_TRIAL, isTrialExpired: true, daysRemaining: 0 };
  }

  const days = Math.floor(msRemaining / (1000 * 60 * 60 * 24));
  return {
    isTrialActive: true,
    isTrialExpired: false,
    daysRemaining: days,
    isLastDay: days < 1,
    isLoading: false,
  };
}
