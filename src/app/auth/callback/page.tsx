"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/providers/AuthProvider";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8090";

export default function AuthCallbackPage() {
  const router = useRouter();
  const { setTokens } = useAuth();

  useEffect(() => {
    // Exchange the temporary HttpOnly cookie for tokens.
    // The backend set this cookie during the OAuth redirect.
    (async () => {
      try {
        const res = await fetch(`${API_URL}/auth/exchange`, {
          method: "POST",
          credentials: "include",
        });

        if (!res.ok) {
          // Check if the error is a waitlist/capacity response
          const errorData = await res.json().catch(() => null);
          if (
            res.status === 503 ||
            errorData?.error === "waitlist" ||
            errorData?.error === "capacity" ||
            errorData?.code === "WAITLIST" ||
            errorData?.code === "CAPACITY"
          ) {
            const emailParam = errorData?.email
              ? `?email=${encodeURIComponent(errorData.email)}`
              : "";
            router.push(`/waitlist${emailParam}`);
            return;
          }
          router.push("/");
          return;
        }

        const data = await res.json();
        if (data.access_token && data.refresh_token && data.user_id && data.email) {
          setTokens(data.access_token, data.refresh_token, {
            id: data.user_id,
            email: data.email,
            name: data.name || "",
            avatarUrl: data.avatar_url || "",
          });
          router.push("/trips");
        } else {
          router.push("/");
        }
      } catch {
        router.push("/");
      }
    })();
  }, [setTokens, router]);

  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="text-center">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--color-accent)] mx-auto mb-4" />
        <p className="text-[var(--color-text-secondary)]">Signing you in...</p>
      </div>
    </div>
  );
}
