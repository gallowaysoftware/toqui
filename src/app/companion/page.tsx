"use client";

import { ChatContainer } from "@/components/chat/ChatContainer";
import { ThemeToggleButton } from "@/components/theme/ThemeToggle";

export default function CompanionPage() {
  return (
    <div className="h-screen flex flex-col">
      <header className="bg-[var(--color-surface)] border-b border-[var(--color-border)] px-4 py-3 flex-shrink-0 flex items-center justify-between">
        <h1 className="text-lg font-semibold text-[var(--color-text-primary)]">Travel Companion</h1>
        <ThemeToggleButton />
      </header>
      <ChatContainer tripId="" mode="companion" />
    </div>
  );
}
