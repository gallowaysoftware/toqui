"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/providers/AuthProvider";

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

export default function AuthCallbackPage() {
  const router = useRouter();
  const { setSession } = useAuth();

  useEffect(() => {
    // Exchange the temporary HttpOnly cookie for auth session.
    // The backend sets HttpOnly auth cookies (toqui_access, toqui_refresh)
    // on the response — we only need the user info and expiry from the body.
    void (async () => {
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
        const expiresAt = data.expires_at;
        if (
          data.user_id &&
          data.email &&
          typeof expiresAt === "number" &&
          expiresAt > 0
        ) {
          setSession(
            {
              id: data.user_id,
              email: data.email,
              name: data.name ?? "",
              avatarUrl: data.avatar_url ?? "",
            },
            expiresAt,
          );

          // Verify age server-side using the DOB stored during client-side age gate
          const storedDob = localStorage.getItem("toqui_age_dob");
          if (storedDob) {
            try {
              await fetch(`${API_URL}/auth/verify-age`, {
                method: "POST",
                credentials: "include",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ date_of_birth: storedDob }),
              });
            } catch {
              // Non-blocking — the age interceptor will catch unverified users on RPC calls
            }
          }

          router.push("/trips");
        } else {
          router.push("/");
        }
      } catch {
        router.push("/");
      }
    })();
  }, [setSession, router]);

  return (
    <main
      id="main-content"
      className="min-h-screen flex items-center justify-center"
      aria-busy="true"
      role="status"
    >
      <div className="text-center">
        <div
          className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--color-accent)] mx-auto mb-4"
          aria-hidden="true"
        />
        <p className="text-[var(--color-text-secondary)]">
          Signing you in...
        </p>
      </div>
    </main>
  );
}
