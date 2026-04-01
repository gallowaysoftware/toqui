import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

export interface UsageData {
  used: number;
  limit: number;
  resetsAt: Date | null;
  isLoading: boolean;
}

interface UsageResponse {
  used: number;
  limit: number;
  resets_at: string;
}

const DEFAULT: UsageData = {
  used: 0,
  limit: 0,
  resetsAt: null,
  isLoading: false,
};

export function useUsage(): UsageData {
  const { accessToken } = useAuth();

  const { data, isLoading } = useQuery({
    queryKey: ["usage"],
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

  return {
    used: data.used,
    limit: data.limit,
    resetsAt: data.resets_at ? new Date(data.resets_at) : null,
    isLoading: false,
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
