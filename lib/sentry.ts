/**
 * Privacy-first Sentry error tracking.
 *
 * Design decisions:
 * - PII (email, name, IP) is stripped from every event via beforeSend.
 * - Breadcrumb data is removed to prevent leaking travel info (destinations, dates).
 * - sendDefaultPii is disabled.
 * - Gracefully no-ops when EXPO_PUBLIC_SENTRY_DSN is empty (dev/test).
 */

import * as Sentry from "@sentry/react-native";

import { getConfig } from "./config";

/**
 * Strip PII and sensitive travel data from Sentry events.
 * Exported for testing.
 */
export function beforeSend(
  event: Sentry.ErrorEvent,
): Sentry.ErrorEvent | null {
  // Remove user PII if accidentally attached
  if (event.user) {
    delete event.user.email;
    delete event.user.username;
    delete event.user.ip_address;
  }

  // Strip breadcrumb data that might contain travel info
  // (destinations, chat content, dates, etc.)
  if (event.breadcrumbs) {
    event.breadcrumbs = event.breadcrumbs.map((b) => ({
      ...b,
      data: undefined,
    }));
  }

  return event;
}

/**
 * Initialise Sentry. No-ops when DSN is empty (dev / test environments).
 * Must be called before the React tree renders.
 */
export function initSentry(): void {
  const dsn = getConfig().sentryDsn;
  if (!dsn) return;

  Sentry.init({
    dsn,
    environment: __DEV__ ? "development" : "production",
    // Sample 10% of transactions for performance monitoring
    tracesSampleRate: 0.1,
    beforeSend,
    // Never send PII automatically
    sendDefaultPii: false,
  });
}

/**
 * Capture an exception in Sentry with optional context.
 * No-ops when Sentry is not initialised.
 */
export function captureException(
  error: unknown,
  context?: Record<string, unknown>,
): void {
  if (context) {
    Sentry.withScope((scope) => {
      for (const [key, value] of Object.entries(context)) {
        scope.setExtra(key, value);
      }
      Sentry.captureException(error);
    });
  } else {
    Sentry.captureException(error);
  }
}

export { Sentry };
