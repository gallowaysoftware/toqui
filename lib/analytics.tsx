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
 *
 * Platform note: this module imports `posthog-js`, the browser SDK. It
 * relies on `window`, `document`, and the browser's localStorage. On
 * iOS/Android (React Native), those globals don't exist and the SDK
 * either crashes on init or silently no-ops, depending on bundler
 * shimming. The provider uses Platform.OS to gate initialisation to web
 * — native builds intentionally run with analytics disabled until we
 * swap in `posthog-react-native`. That swap is its own PR (different
 * SDK shape — no `posthog.init`, different session-replay config) and
 * isn't worth doing while native isn't shipping yet.
 */

import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useMemo,
  useCallback,
} from "react";
import { Platform } from "react-native";
import type { ReactNode } from "react";
// posthog-js is loaded dynamically inside the provider so it doesn't ship
// in the entry chunk. ~33 KB gzipped saved on first paint for every visitor
// (most haven't consented yet at first paint) — see issue #204.
// Type-only import: TS strips this at build, no runtime cost.
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
  "format", // itinerary_exported: "pdf" | "calendar"
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

// Calls fired before the dynamic import resolves get queued and replayed
// on first init. Without buffering, the very first session_start event
// (fired in app/_layout.tsx on mount, which can race with the dynamic
// import) would be silently dropped.
type PendingCall =
  | { kind: "track"; event: string; properties?: Record<string, unknown> }
  | { kind: "identify"; userId: string }
  | { kind: "reset" };

export function AnalyticsProvider({ children }: AnalyticsProviderProps) {
  const clientRef = useRef<PostHog | null>(null);
  const pendingRef = useRef<PendingCall[]>([]);

  // Lazy-load posthog-js: it's a ~33 KB gzipped browser SDK that we don't
  // need until after first paint, and many users will never consent (so
  // analytics never fires for them). Static-import was forcing every
  // visitor to download the SDK even when they bounce immediately.
  useEffect(() => {
    // posthog-js is browser-only — gate on Platform so native builds
    // (which never ship right now anyway) don't crash on `window`.
    if (Platform.OS !== "web") return;
    const key = getConfig().posthogKey;
    if (!key) return; // analytics disabled (dev / test)

    let cancelled = false;
    void import("posthog-js").then(({ default: posthog }) => {
      if (cancelled) return;

      posthog.init(key, {
        api_host: "https://eu.i.posthog.com",
        // Cookie-less: keep state in memory only
        persistence: "memory",
        // Never autocapture — we use explicit events only
        autocapture: false,
        // Disable automatic pageview — we fire session_start manually
        capture_pageview: false,
        capture_pageleave: false,
        // Session replay is intentionally OFF. The previous config set
        // session_recording masking flags, but `advanced_disable_decide:
        // true` (below) disables the /decide endpoint that PostHog uses
        // to *enable* recording from the server side — so masking config
        // was theatre, recording wasn't running anyway. Rather than wire
        // a half-working feature, we keep it disabled and the privacy
        // policy stays true to "we don't run session replay". When/if we
        // turn it on, this is the only line that needs to flip.
        disable_session_recording: true,
        // Disable surveys + toolbar to minimise bundle
        advanced_disable_decide: true,
      });

      clientRef.current = posthog;

      // Flush queued calls in arrival order. session_start is almost
      // always first in the queue; without this the first event of
      // every session would be silently dropped.
      const queued = pendingRef.current;
      pendingRef.current = [];
      for (const call of queued) {
        switch (call.kind) {
          case "track":
            posthog.capture(call.event, stripSensitiveProps(call.properties));
            break;
          case "identify":
            void hashUserId(call.userId).then((hashed) => {
              posthog.identify(hashed);
            });
            break;
          case "reset":
            posthog.reset();
            break;
        }
      }
    });

    return () => {
      cancelled = true;
      // PostHog doesn't have a destroy, but we can clear the ref
      clientRef.current = null;
    };
  }, []);

  const track = useCallback(
    (event: string, properties?: Record<string, unknown>) => {
      if (clientRef.current) {
        clientRef.current.capture(event, stripSensitiveProps(properties));
      } else {
        pendingRef.current.push({ kind: "track", event, properties });
      }
    },
    [],
  );

  const identify = useCallback((userId: string) => {
    if (clientRef.current) {
      // hashUserId is async — fire and forget (identify is not a hot path)
      void hashUserId(userId).then((hashed) => {
        // Identify WITHOUT any $set properties — no PII
        clientRef.current?.identify(hashed);
      });
    } else {
      pendingRef.current.push({ kind: "identify", userId });
    }
  }, []);

  const reset = useCallback(() => {
    if (clientRef.current) {
      clientRef.current.reset();
    } else {
      pendingRef.current.push({ kind: "reset" });
    }
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
