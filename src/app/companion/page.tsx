"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/providers/AuthProvider";
import { ChatContainer } from "@/components/chat/ChatContainer";
import { ThemeToggleButton } from "@/components/theme/ThemeToggle";

export default function CompanionPage() {
  const { user, isLoading: authLoading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/");
    }
  }, [authLoading, user, router]);

  if (authLoading || !user) {
    return (
      <div className="h-screen flex items-center justify-center" aria-busy="true" role="status">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-[var(--color-accent)]" />
        <span className="sr-only">Loading...</span>
      </div>
    );
  }

  return (
    <div id="main-content" className="h-screen flex flex-col">
      <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)] px-4 py-3 flex-shrink-0 flex items-center justify-between">
        <h1 className="text-lg font-semibold text-[var(--color-text-primary)]">Travel Companion</h1>
        <ThemeToggleButton />
      </header>
      <ChatContainer tripId="" mode="companion" />
    </div>
  );
}
