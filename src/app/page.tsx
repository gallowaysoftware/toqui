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
    <main className="flex min-h-screen flex-col items-center justify-center p-8">
      <div className="max-w-2xl text-center">
        <h1 className="text-5xl font-bold tracking-tight text-white mb-4">{t("title")}</h1>
        <p className="text-xl text-gray-400 mb-8">{t("subtitle")}</p>
        <div className="flex gap-4 justify-center">
          <button
            onClick={login}
            className="rounded-full bg-blue-600 px-6 py-3 text-white font-medium hover:bg-blue-700 transition-colors"
          >
            {t("startPlanning")}
          </button>
          <button
            onClick={login}
            className="rounded-full border border-gray-600 px-6 py-3 text-gray-300 font-medium hover:bg-gray-800 transition-colors"
          >
            {tc("signIn")}
          </button>
        </div>
      </div>
    </main>
  );
}
