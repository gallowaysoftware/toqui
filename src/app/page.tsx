"use client";

import { useEffect } from "react";
import { useTranslations } from "next-intl";
import { useAuth } from "@/components/providers/AuthProvider";
import { useRouter } from "next/navigation";

export default function Home() {
  const t = useTranslations("home");
  const tc = useTranslations("common");
  const { user, isLoading, login } = useAuth();
  const router = useRouter();

  // If logged in, go straight to trips
  useEffect(() => {
    if (!isLoading && user) {
      router.push("/trips");
    }
  }, [isLoading, user, router]);

  if (!isLoading && user) return null;

  return (
    <main id="main-content" className="flex min-h-screen flex-col items-center justify-center p-8">
      <div className="max-w-2xl text-center">
        <h1 className="text-5xl font-bold tracking-tight text-[var(--color-text-primary)] mb-4">{t("title")}</h1>
        <p className="text-xl text-[var(--color-text-tertiary)] mb-8">{t("subtitle")}</p>
        <div className="flex gap-4 justify-center">
          <button
            onClick={login}
            className="rounded-full bg-[var(--color-accent)] px-6 py-3 text-white font-medium hover:bg-[var(--color-accent-hover)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2"
          >
            {t("startPlanning")}
          </button>
          <button
            onClick={login}
            className="rounded-full border border-[var(--color-border-strong)] px-6 py-3 text-[var(--color-text-secondary)] font-medium hover:bg-[var(--color-surface-tertiary)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2"
          >
            {tc("signIn")}
          </button>
        </div>
      </div>
    </main>
  );
}
