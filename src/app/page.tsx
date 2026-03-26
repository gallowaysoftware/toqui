"use client";

import { useEffect } from "react";
import { useAuth } from "@/components/providers/AuthProvider";
import { useRouter } from "next/navigation";

export default function Home() {
  const { user, isLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (isLoading) return;
    if (user) {
      router.replace("/trips");
    } else {
      // Marketing site handles the landing page
      window.location.href = "https://toqui.travel";
    }
  }, [isLoading, user, router]);

  return (
    <div className="min-h-screen flex items-center justify-center" aria-busy="true" role="status">
      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--color-accent)]" />
      <span className="sr-only">Loading...</span>
    </div>
  );
}
