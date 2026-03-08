"use client";

import { useState, useEffect } from "react";
import { Info, X } from "lucide-react";

const DISMISSED_KEY = "toqui_ai_notice_dismissed";

export function AITransparencyNotice() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const dismissed = localStorage.getItem(DISMISSED_KEY);
    if (!dismissed) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- localStorage not available during SSR
      setVisible(true);
    }
  }, []);

  if (!visible) {
    return null;
  }

  const handleDismiss = () => {
    setVisible(false);
    localStorage.setItem(DISMISSED_KEY, "1");
  };

  return (
    <div
      className="flex items-start gap-2 mx-4 mt-3 mb-1 px-3 py-2.5 rounded-lg bg-[var(--color-surface-tertiary)] border border-[var(--color-border)]"
      role="note"
      aria-label="AI transparency notice"
    >
      <Info
        size={14}
        className="text-[var(--color-text-secondary)] mt-0.5 flex-shrink-0"
        aria-hidden="true"
      />
      <p className="text-xs text-[var(--color-text-secondary)] leading-relaxed flex-1">
        Toqui uses AI to generate responses. Information may not always be accurate. Verify
        important details before making travel decisions.
      </p>
      <button
        onClick={handleDismiss}
        className="text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)] transition-colors flex-shrink-0 mt-0.5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] rounded"
        aria-label="Dismiss AI notice"
      >
        <X size={14} />
      </button>
    </div>
  );
}
