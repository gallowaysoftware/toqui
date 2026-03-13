"use client";

import { useMutation, useQuery } from "@tanstack/react-query";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

export interface WaitlistJoinResponse {
  position: number;
  invite_code: string;
}

export interface WaitlistStatusResponse {
  position: number;
  accepted: boolean;
}

export function useJoinWaitlist() {
  return useMutation<WaitlistJoinResponse, Error, { email: string }>({
    mutationFn: async ({ email }) => {
      const res = await fetch(`${API_URL}/waitlist`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      });

      if (!res.ok) {
        const body = await res.text().catch(() => "");
        throw new Error(body || `Failed to join waitlist (${res.status})`);
      }

      return res.json();
    },
  });
}

export function useWaitlistStatus(email: string | null) {
  return useQuery<WaitlistStatusResponse>({
    queryKey: ["waitlist-status", email],
    queryFn: async () => {
      const res = await fetch(`${API_URL}/waitlist/status?email=${encodeURIComponent(email!)}`);

      if (!res.ok) {
        throw new Error(`Failed to check waitlist status (${res.status})`);
      }

      return res.json();
    },
    enabled: !!email,
    refetchInterval: (query) =>
      query.state.data?.accepted ? false : 30_000, // Stop polling once accepted
  });
}
