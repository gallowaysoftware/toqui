"use client";

import { useState } from "react";
import { MessageCircle, Zap } from "lucide-react";
import type { UsageInfo } from "@/lib/hooks/useUsage";

interface MessageLimitBannerProps {
  usage: UsageInfo;
}

export function MessageLimitBanner({ usage }: MessageLimitBannerProps) {
  const { remaining, limit, isAtLimit, isWarning } = usage;
  const [showComingSoon, setShowComingSoon] = useState(false);

  if (isAtLimit) {
    return (
      <div
        className="flex flex-col gap-1.5 px-4 py-2.5 bg-[var(--color-error-bg)] border-t border-[var(--color-border)]"
        role="alert"
      >
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2 text-sm text-[var(--color-error)]">
            <MessageCircle size={14} aria-hidden="true" />
            <span>Daily message limit reached</span>
          </div>
          <button
            className="flex items-center gap-1.5 px-3 py-1 rounded-lg bg-[var(--color-text-tertiary)] text-white text-xs font-medium cursor-default opacity-75"
            onClick={() => setShowComingSoon(true)}
            aria-describedby={showComingSoon ? "coming-soon-msg" : undefined}
          >
            <Zap size={12} aria-hidden="true" />
            Coming Soon
          </button>
        </div>
        {showComingSoon && (
          <p
            id="coming-soon-msg"
            className="text-xs text-[var(--color-text-secondary)]"
          >
            Trip Pro is coming soon! Your message limit will reset tomorrow.
          </p>
        )}
        {!showComingSoon && (
          <p className="text-xs text-[var(--color-text-tertiary)]">
            Your message limit resets daily at midnight.
          </p>
        )}
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
