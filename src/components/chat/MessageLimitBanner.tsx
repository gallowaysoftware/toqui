"use client";

import { MessageCircle, Zap } from "lucide-react";
import type { UsageInfo } from "@/lib/hooks/useUsage";

interface MessageLimitBannerProps {
  usage: UsageInfo;
}

export function MessageLimitBanner({ usage }: MessageLimitBannerProps) {
  const { remaining, limit, isAtLimit, isWarning } = usage;

  if (isAtLimit) {
    return (
      <div
        className="flex items-center justify-between gap-3 px-4 py-2.5 bg-[var(--color-error-bg)] border-t border-[var(--color-border)]"
        role="alert"
      >
        <div className="flex items-center gap-2 text-sm text-[var(--color-error)]">
          <MessageCircle size={14} aria-hidden="true" />
          <span>Daily message limit reached</span>
        </div>
        <button
          className="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-[var(--color-accent)] text-white text-xs font-medium hover:bg-[var(--color-accent-hover)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2"
          onClick={() => {
            // Upgrade flow will be wired up in a future issue
          }}
        >
          <Zap size={12} aria-hidden="true" />
          Upgrade to Trip Pro
        </button>
      </div>
    );
  }

  const colorClass = isWarning
    ? "text-[var(--color-warning-text)]"
    : "text-[var(--color-text-tertiary)]";

  return (
    <div
      className={`flex items-center gap-2 px-4 py-1.5 text-xs border-t border-[var(--color-border)] ${colorClass}`}
      role="status"
      aria-live="polite"
    >
      <MessageCircle size={12} aria-hidden="true" />
      <span>
        {remaining} of {limit} messages remaining today
      </span>
    </div>
  );
}
