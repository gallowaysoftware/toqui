"use client";

import { useCallback } from "react";
import { Compass, MapPin, Utensils, Calendar, Plane, Sun, HelpCircle, Camera } from "lucide-react";
import type { LucideIcon } from "lucide-react";

interface Prompt {
  icon: LucideIcon;
  label: string;
  message: string;
}

const planningPrompts: Prompt[] = [
  {
    icon: Compass,
    label: "Plan a trip",
    message: "I want to plan a trip but I'm not sure where to go. Can you help me pick a destination?",
  },
  {
    icon: Calendar,
    label: "Build an itinerary",
    message: "Help me build a day-by-day itinerary for my trip.",
  },
  {
    icon: Utensils,
    label: "Food recommendations",
    message: "What are the must-try local dishes and best restaurants at my destination?",
  },
  {
    icon: Plane,
    label: "Travel logistics",
    message: "Help me figure out flights, transportation, and getting around.",
  },
];

const companionPrompts: Prompt[] = [
  {
    icon: MapPin,
    label: "What's nearby",
    message: "What interesting places are near me right now?",
  },
  {
    icon: Utensils,
    label: "Find food",
    message: "I'm hungry — what are the best restaurants nearby?",
  },
  {
    icon: HelpCircle,
    label: "Local tips",
    message: "Any local customs or tips I should know about while I'm here?",
  },
  {
    icon: Camera,
    label: "Things to do",
    message: "What activities or sights should I check out today?",
  },
];

const selectionPrompts: Prompt[] = [
  {
    icon: Sun,
    label: "Beach getaway",
    message: "I want to plan a relaxing beach vacation somewhere warm.",
  },
  {
    icon: Compass,
    label: "Adventure trip",
    message: "I'm looking for an adventurous trip with hiking, nature, and outdoor activities.",
  },
  {
    icon: Utensils,
    label: "Food & culture",
    message: "Plan me a trip focused on amazing food and cultural experiences.",
  },
  {
    icon: MapPin,
    label: "City break",
    message: "I want a fun city break — great nightlife, shopping, and sightseeing.",
  },
];

const promptsByMode: Record<string, Prompt[]> = {
  planning: planningPrompts,
  companion: companionPrompts,
  selection: selectionPrompts,
};

interface SuggestedPromptsProps {
  mode: string;
  onSelect: (message: string) => void;
  disabled?: boolean;
}

export function SuggestedPrompts({ mode, onSelect, disabled }: SuggestedPromptsProps) {
  const prompts = promptsByMode[mode] ?? planningPrompts;

  const handleClick = useCallback(
    (message: string) => {
      if (!disabled) {
        onSelect(message);
      }
    },
    [onSelect, disabled],
  );

  return (
    <div className="flex flex-col items-center justify-center py-8 px-4">
      <h2 className="text-lg font-semibold text-[var(--color-text-primary)] mb-1">
        {mode === "companion" ? "How can I help?" : "What would you like to do?"}
      </h2>
      <p className="text-sm text-[var(--color-text-tertiary)] mb-6">
        {mode === "companion"
          ? "Ask me anything while you travel."
          : "Pick a suggestion or type your own message."}
      </p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 w-full max-w-lg">
        {prompts.map((prompt) => (
          <PromptCard
            key={prompt.label}
            icon={prompt.icon}
            label={prompt.label}
            onClick={() => handleClick(prompt.message)}
            disabled={disabled}
          />
        ))}
      </div>
    </div>
  );
}

function PromptCard({
  icon: Icon,
  label,
  onClick,
  disabled,
}: {
  icon: LucideIcon;
  label: string;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="flex items-center gap-3 rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] px-4 py-3 text-left text-sm font-medium text-[var(--color-text-secondary)] hover:border-[var(--color-accent)] hover:text-[var(--color-accent)] transition-colors disabled:opacity-50 disabled:cursor-not-allowed focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2"
    >
      <Icon size={18} className="flex-shrink-0" aria-hidden="true" />
      {label}
    </button>
  );
}
