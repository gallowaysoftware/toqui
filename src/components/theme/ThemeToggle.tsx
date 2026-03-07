"use client";

import { Moon, Sun, Monitor } from "lucide-react";
import { useTheme, type Theme } from "@/components/providers/ThemeProvider";

const options: { value: Theme; label: string; icon: typeof Sun }[] = [
  { value: "light", label: "Light", icon: Sun },
  { value: "dark", label: "Dark", icon: Moon },
  { value: "system", label: "System", icon: Monitor },
];

/**
 * Compact theme toggle button that cycles through light -> dark -> system.
 * Used in the header/nav area.
 */
export function ThemeToggleButton() {
  const { theme, setTheme } = useTheme();

  const cycle = () => {
    const order: Theme[] = ["light", "dark", "system"];
    const next = order[(order.indexOf(theme) + 1) % order.length];
    setTheme(next);
  };

  const current = options.find((o) => o.value === theme) ?? options[2];
  const Icon = current.icon;

  return (
    <button
      onClick={cycle}
      className="p-2 rounded-lg text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-tertiary)] hover:text-[var(--color-text-primary)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
      aria-label={`Theme: ${current.label}. Click to change.`}
      title={`Theme: ${current.label}`}
    >
      <Icon size={18} aria-hidden="true" />
    </button>
  );
}

/**
 * Expanded theme selector for use in the settings page.
 * Shows all three options as a segmented control.
 */
export function ThemeSelector() {
  const { theme, setTheme } = useTheme();

  return (
    <div className="flex rounded-lg border border-[var(--color-border)] overflow-hidden" role="group" aria-label="Theme selection">
      {options.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          onClick={() => setTheme(value)}
          className={`flex items-center gap-2 px-4 py-2 text-sm font-medium transition-colors flex-1 justify-center focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-inset ${
            theme === value
              ? "bg-[var(--color-accent)] text-white"
              : "bg-[var(--color-surface)] text-[var(--color-text-secondary)] hover:bg-[var(--color-surface-tertiary)]"
          }`}
          aria-pressed={theme === value}
        >
          <Icon size={16} aria-hidden="true" />
          {label}
        </button>
      ))}
    </div>
  );
}
