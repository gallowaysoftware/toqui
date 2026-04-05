/**
 * Privacy-first analytics wrapper around PostHog.
 *
 * Design decisions:
 * - User IDs are pseudonymised (SHA-256 hashed) before being sent to PostHog.
 * - No PII ($set with email/name) is ever attached to identify calls.
 * - EU hosting endpoint (eu.i.posthog.com) for data residency.
 * - Session replay masks all text inputs.
 * - Cookie-less mode (persistence set to "memory").
 * - Gracefully no-ops when EXPO_PUBLIC_POSTHOG_KEY is empty (dev/test).
 * - Allowlist-based property filter: only known-safe keys are forwarded.
 */

import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useMemo,
  useCallback,
} from "react";
import type { ReactNode } from "react";
import posthog from "posthog-js";
import type { PostHog } from "posthog-js";

import { getConfig } from "./config";

// ---------------------------------------------------------------------------
// Allowlist of safe analytics property keys.
// Any property NOT in this set is stripped before sending to PostHog.
// ---------------------------------------------------------------------------
const SAFE_PROPERTIES = new Set([
  "source",
  "mode",
  "action",
  "platform",
  "auth_provider",
  "has_dates",
  "from_template",
  "template_id",
  "method",
  "item_count",
  "day_count",
  "amount",
  "screen",
  "error_type",
  "partner",
  "category",
  "trigger",
  "count",
  "is_first",
  "price_variant",
  "remaining",
  "$lib",
  "$lib_version",
]);

/**
 * Keep only known-safe properties from an event payload.
 * Any property not in the SAFE_PROPERTIES allowlist is stripped.
 */
export function stripSensitiveProps(
  props: Record<string, unknown> | undefined,
): Record<string, unknown> | undefined {
  if (!props) return props;
  const clean: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(props)) {
    if (SAFE_PROPERTIES.has(key)) {
      clean[key] = value;
    }
  }
  return clean;
}

// ---------------------------------------------------------------------------
// User-ID pseudonymisation (SHA-256)
// ---------------------------------------------------------------------------

/**
 * Deterministic SHA-256 hash for pseudonymising user IDs before sending to
 * PostHog. Falls back to a simple hash when crypto.subtle is unavailable.
 */
export async function hashUserId(userId: string): Promise<string> {
  if (typeof crypto !== "undefined" && crypto.subtle) {
    const data = new TextEncoder().encode(userId);
    const hash = await crypto.subtle.digest("SHA-256", data);
    const hex = Array.from(new Uint8Array(hash))
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
    return `u_${hex.slice(0, 16)}`; // 64 bits = no birthday problem under billions
  }
  // Fallback for environments without crypto.subtle
  let h = 0;
  for (let i = 0; i < userId.length; i++) {
    h = (Math.imul(31, h) + userId.charCodeAt(i)) | 0;
  }
  return `u_${Math.abs(h).toString(16).padStart(8, "0")}`;
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

interface AnalyticsContext {
  /** Track a named event. Properties are automatically sanitised. */
  track: (event: string, properties?: Record<string, unknown>) => void;
  /** Identify the current user (hashed). Call after login. */
  identify: (userId: string) => void;
  /** Reset identity (call on logout). */
  reset: () => void;
  /** Read a PostHog feature flag value. Returns undefined when unavailable. */
  getFeatureFlag: (key: string) => string | boolean | undefined;
}

const noop: AnalyticsContext = {
  track: () => {},
  identify: () => {},
  reset: () => {},
  getFeatureFlag: () => undefined,
};

const Ctx = createContext<AnalyticsContext>(noop);

export function useAnalytics(): AnalyticsContext {
  return useContext(Ctx);
}

// ---------------------------------------------------------------------------
// Provider
// ---------------------------------------------------------------------------

interface AnalyticsProviderProps {
  children: ReactNode;
}

export function AnalyticsProvider({ children }: AnalyticsProviderProps) {
  const clientRef = useRef<PostHog | null>(null);

  // Initialise PostHog once on mount
  useEffect(() => {
    const key = getConfig().posthogKey;
    if (!key) return; // analytics disabled (dev / test)

    posthog.init(key, {
      api_host: "https://eu.i.posthog.com",
      // Cookie-less: keep state in memory only
      persistence: "memory",
      // Never autocapture — we use explicit events only
      autocapture: false,
      // Disable automatic pageview — we fire session_start manually
      capture_pageview: false,
      capture_pageleave: false,
      // Session replay privacy
      session_recording: {
        maskAllInputs: true,
        maskTextSelector: "*",
      },
      // Disable surveys + toolbar to minimise bundle
      advanced_disable_decide: true,
    });

    clientRef.current = posthog;

    return () => {
      // PostHog doesn't have a destroy, but we can clear the ref
      clientRef.current = null;
    };
  }, []);

  const track = useCallback(
    (event: string, properties?: Record<string, unknown>) => {
      clientRef.current?.capture(event, stripSensitiveProps(properties));
    },
    [],
  );

  const identify = useCallback((userId: string) => {
    // hashUserId is async — fire and forget (identify is not a hot path)
    void hashUserId(userId).then((hashed) => {
      // Identify WITHOUT any $set properties — no PII
      clientRef.current?.identify(hashed);
    });
  }, []);

  const reset = useCallback(() => {
    clientRef.current?.reset();
  }, []);

  const getFeatureFlag = useCallback(
    (key: string): string | boolean | undefined => {
      const val = clientRef.current?.getFeatureFlag(key);
      if (val === null || val === undefined) return undefined;
      return val as string | boolean;
    },
    [],
  );

  const value = useMemo(
    () => ({ track, identify, reset, getFeatureFlag }),
    [track, identify, reset, getFeatureFlag],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
