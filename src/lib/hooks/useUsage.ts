"use client";

import { useState, useCallback, useEffect, useMemo } from "react";

const DAILY_LIMIT = 30;
const STORAGE_KEY = "toqui_daily_usage";
const WARNING_THRESHOLD = 5;

interface StoredUsage {
  date: string;
  count: number;
}

function getTodayKey(): string {
  return new Date().toISOString().slice(0, 10);
}

function loadUsage(): StoredUsage {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      const parsed: StoredUsage = JSON.parse(stored);
      if (parsed.date === getTodayKey()) {
        return parsed;
      }
    }
  } catch {
    // Ignore corrupt data
  }
  return { date: getTodayKey(), count: 0 };
}

function saveUsage(usage: StoredUsage): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(usage));
}

export interface UsageInfo {
  /** Messages sent today */
  used: number;
  /** Daily message limit */
  limit: number;
  /** Messages remaining today */
  remaining: number;
  /** Whether the user is at or over the limit */
  isAtLimit: boolean;
  /** Whether the user is approaching the limit (<= WARNING_THRESHOLD remaining) */
  isWarning: boolean;
  /** Record a sent message (increments the counter) */
  recordMessage: () => void;
  /** Mark as exhausted from a backend RESOURCE_EXHAUSTED error */
  markExhausted: () => void;
}

export function useUsage(): UsageInfo {
  const [used, setUsed] = useState(0);

  useEffect(() => {
    const stored = loadUsage();
    // eslint-disable-next-line react-hooks/set-state-in-effect -- localStorage not available during SSR
    setUsed(stored.count);
  }, []);

  const recordMessage = useCallback(() => {
    setUsed((prev) => {
      const newCount = prev + 1;
      saveUsage({ date: getTodayKey(), count: newCount });
      return newCount;
    });
  }, []);

  const markExhausted = useCallback(() => {
    setUsed(DAILY_LIMIT);
    saveUsage({ date: getTodayKey(), count: DAILY_LIMIT });
  }, []);

  // Memoize the return value so consumers using the whole object as a
  // dependency (e.g., useCallback deps) get a stable reference when `used`
  // hasn't changed.
  return useMemo(() => {
    const remaining = Math.max(0, DAILY_LIMIT - used);
    const isAtLimit = remaining === 0;
    const isWarning = !isAtLimit && remaining <= WARNING_THRESHOLD;

    return {
      used,
      limit: DAILY_LIMIT,
      remaining,
      isAtLimit,
      isWarning,
      recordMessage,
      markExhausted,
    };
  }, [used, recordMessage, markExhausted]);
}
