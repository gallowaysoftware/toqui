/**
 * Privacy-first analytics wrapper around PostHog.
 *
 * Design decisions:
 * - User IDs are pseudonymised (hashed) before being sent to PostHog.
 * - No PII ($set with email/name) is ever attached to identify calls.
 * - EU hosting endpoint (eu.i.posthog.com) for data residency.
 * - Session replay masks all text inputs.
 * - Cookie-less mode (persistence set to "memory").
 * - Gracefully no-ops when EXPO_PUBLIC_POSTHOG_KEY is empty (dev/test).
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
// Sensitive property keys that must never leave the device
// ---------------------------------------------------------------------------
const SENSITIVE_KEYS = new Set([
  "destination",
  "destination_name",
  "chat_content",
  "message",
  "message_content",
  "travel_dates",
  "start_date",
  "end_date",
  "booking_details",
  "email",
  "name",
  "user_name",
  "user_email",
  "phone",
]);

/**
 * Strip any accidentally-included sensitive properties from an event payload.
 */
export function stripSensitiveProps(
  props: Record<string, unknown> | undefined,
): Record<string, unknown> | undefined {
  if (!props) return props;
  const clean: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(props)) {
    if (!SENSITIVE_KEYS.has(key)) {
      clean[key] = value;
    }
  }
  return clean;
}

// ---------------------------------------------------------------------------
// User-ID pseudonymisation
// ---------------------------------------------------------------------------

/**
 * Deterministic hash for pseudonymising user IDs before sending to PostHog.
 * This is NOT cryptographic — it's a simple string-hash to decouple the
 * analytics identity from the real database UUID.
 */
export function hashUserId(userId: string): string {
  let hash = 0;
  for (let i = 0; i < userId.length; i++) {
    const char = userId.charCodeAt(i);
    hash = (hash << 5) - hash + char;
    hash |= 0; // Convert to 32-bit integer
  }
  return `u_${Math.abs(hash).toString(36)}`;
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
}

const noop: AnalyticsContext = {
  track: () => {},
  identify: () => {},
  reset: () => {},
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
    const hashed = hashUserId(userId);
    // Identify WITHOUT any $set properties — no PII
    clientRef.current?.identify(hashed);
  }, []);

  const reset = useCallback(() => {
    clientRef.current?.reset();
  }, []);

  const value = useMemo(
    () => ({ track, identify, reset }),
    [track, identify, reset],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}
