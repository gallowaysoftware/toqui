import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

export type UsageTier = "free" | "pro" | "explorer" | "voyager";

export interface UsageData {
  used: number;
  limit: number;
  resetsAt: Date | null;
  isLoading: boolean;
  /** User's current tier (from API, defaults to "free" if not provided). */
  tier: UsageTier;
  /** Messages remaining before hitting the daily cap. */
  remainingMessages: number;
  /** True when the user has consumed >80% of their daily limit. */
  isNearLimit: boolean;
  /** True when the user has reached or exceeded their daily limit. */
  isAtLimit: boolean;
}

interface UsageResponse {
  used: number;
  limit: number;
  resets_at: string;
  tier?: string;
}

function parseTier(raw?: string): UsageTier {
  if (raw === "pro" || raw === "explorer" || raw === "voyager") return raw;
  return "free";
}

const DEFAULT: UsageData = {
  used: 0,
  limit: 0,
  resetsAt: null,
  isLoading: false,
  tier: "free",
  remainingMessages: 0,
  isNearLimit: false,
  isAtLimit: false,
};

export function useUsage(): UsageData {
  const { accessToken } = useAuth();

  const { data, isLoading } = useQuery({
    queryKey: ["usage", accessToken],
    queryFn: async (): Promise<UsageResponse> => {
      const res = await authFetch(
        `${getConfig().apiUrl}/api/usage`,
        accessToken,
      );
      if (!res.ok) throw new Error("Failed to fetch usage");
      return res.json();
    },
    enabled: !!accessToken,
    staleTime: 30_000,
    retry: false,
  });

  if (isLoading) {
    return { ...DEFAULT, isLoading: true };
  }

  if (!data) return DEFAULT;

  const remaining = Math.max(0, data.limit - data.used);
  const tier = parseTier(data.tier);

  return {
    used: data.used,
    limit: data.limit,
    resetsAt: data.resets_at ? new Date(data.resets_at) : null,
    isLoading: false,
    tier,
    remainingMessages: remaining,
    isNearLimit: data.limit > 0 && data.used / data.limit > 0.8,
    isAtLimit: data.limit > 0 && data.used >= data.limit,
  };
}

/** Returns a human-readable string like "in 3h 22m" or "in 45m". */
export function formatTimeUntilReset(resetsAt: Date | null): string {
  if (!resetsAt) return "";
  const msRemaining = resetsAt.getTime() - Date.now();
  if (msRemaining <= 0) return "soon";
  const totalMinutes = Math.ceil(msRemaining / 60_000);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  if (hours < 1) return `in ${minutes}m`;
  if (minutes === 0) return `in ${hours}h`;
  return `in ${hours}h ${minutes}m`;
}
