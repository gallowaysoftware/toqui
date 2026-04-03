import { useEffect, useState, useRef } from "react";
import { Platform, View, StyleSheet } from "react-native";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider, useAuth } from "@/lib/auth";
import { TransportProvider } from "@/lib/transport";
import { AnalyticsProvider, useAnalytics } from "@/lib/analytics";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { I18nProvider } from "@/lib/i18n";
import { ThemeProvider, useTheme } from "@/lib/theme";
import { AgeGate } from "@/components/auth/AgeGate";
import { OfflineBanner } from "@/components/OfflineBanner";
import { loadConfig, getConfig } from "@/lib/config";
import * as Sentry from "@sentry/react-native";

// ---------------------------------------------------------------------------
// Sentry initialisation — deferred until runtime config is loaded so we can
// read the DSN from config.json (injected at container start) rather than
// hard-coding it into the JS bundle.
// ---------------------------------------------------------------------------

/** Regex patterns whose trailing content likely contains user travel data. */
const TRAVEL_DATA_PATTERNS =
  /(?:trip to|destination|itinerary for|travel(?:ing)? to)\s+.+/gi;

function initSentry() {
  const dsn = getConfig().sentryDsn;
  if (!dsn) return; // dev mode — no DSN, skip init

  Sentry.init({
    dsn,

    environment: __DEV__ ? "development" : "production",

    // Privacy: do NOT send PII (emails, IPs, cookies)
    sendDefaultPii: false,

    enableLogs: true,

    // Session Replay: mask all text AND images for privacy
    // (travel photos, maps, booking confirmations should not be captured)
    replaysSessionSampleRate: 0.1,
    replaysOnErrorSampleRate: 1,
    integrations: [
      Sentry.mobileReplayIntegration({
        maskAllText: true,
        maskAllImages: true,
      }),
      Sentry.feedbackIntegration(),
    ],

    // Privacy: strip PII and user-generated content from error reports
    beforeSend(event) {
      // --- user fields ---
      if (event.user) {
        delete event.user.email;
        delete event.user.username;
        delete event.user.ip_address;
      }

      // --- exception values: scrub travel data from messages ---
      if (event.exception?.values) {
        for (const ex of event.exception.values) {
          if (ex.value) {
            ex.value = ex.value.replace(TRAVEL_DATA_PATTERNS, (match) => {
              const keyword = match.split(/\s+/)[0]; // keep the keyword
              return `${keyword} [REDACTED]`;
            });
          }
        }
      }

      // --- request query string ---
      if (event.request?.query_string) {
        event.request.query_string = "[REDACTED]";
      }

      // --- breadcrumbs: keep structure, strip data & sanitise message ---
      if (event.breadcrumbs) {
        event.breadcrumbs = event.breadcrumbs.map((b) => ({
          category: b.category,
          type: b.type,
          timestamp: b.timestamp,
          // Strip `data` entirely and sanitise `message` (remove URL query params)
          message: b.message
            ? b.message.replace(/\?[^\s]*/g, "?[REDACTED]")
            : undefined,
        }));
      }

      return event;
    },
  });
}

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 60,
      retry: 1,
    },
  },
});

function ThemedStack() {
  const { colors, isDark } = useTheme();
  return (
    <>
      <StatusBar style={isDark ? "light" : "dark"} />
      <Stack
        screenOptions={{
          headerStyle: { backgroundColor: colors.accent },
          headerTintColor: "#fff",
          headerTitleStyle: { fontWeight: "bold" },
          contentStyle: { backgroundColor: colors.surfaceSecondary },
        }}
      >
        <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
        <Stack.Screen name="trips/[tripId]" options={{ headerShown: false }} />
        <Stack.Screen name="auth/callback" options={{ headerShown: false }} />
        <Stack.Screen
          name="shared/[token]"
          options={{ title: "Shared Trip" }}
        />
        <Stack.Screen name="privacy" options={{ title: "Privacy Policy" }} />
        <Stack.Screen name="terms" options={{ title: "Terms of Service" }} />
        <Stack.Screen name="onboarding" options={{ headerShown: false }} />
      </Stack>
    </>
  );
}

/**
 * ErrorBoundary wrapper that reports errors to analytics.
 */
function AnalyticsErrorBoundary({ children }: { children: React.ReactNode }) {
  const { track } = useAnalytics();
  return (
    <ErrorBoundary
      onError={(error) => {
        track("error_encountered", {
          error_name: error.name,
        });
      }}
    >
      {children}
    </ErrorBoundary>
  );
}

/**
 * Fires session_start on mount and auto-identifies authenticated users.
 */
function AnalyticsBootstrap() {
  const { track, identify } = useAnalytics();
  const { user } = useAuth();

  useEffect(() => {
    track("session_start", { platform: Platform.OS });
  }, [track]);

  useEffect(() => {
    if (user?.id) {
      identify(user.id);
    }
  }, [user?.id, identify]);

  return null;
}

export default Sentry.wrap(function RootLayout() {
  const [configLoaded, setConfigLoaded] = useState(false);
  const sentryInitialised = useRef(false);

  useEffect(() => {
    loadConfig().then(() => {
      setConfigLoaded(true);
      if (!sentryInitialised.current) {
        sentryInitialised.current = true;
        initSentry();
      }
    });
  }, []);

  useEffect(() => {
    if (Platform.OS === "web" && typeof window !== "undefined") {
      const params = new URLSearchParams(window.location.search);
      const ref = params.get("ref");
      if (ref) {
        sessionStorage.setItem("toqui_pending_ref", ref);
      }
    }
  }, []);

  if (!configLoaded) return null;

  return (
    <ThemeProvider>
      <I18nProvider>
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <AnalyticsProvider>
              <TransportProvider>
                <AgeGate>
                  <AnalyticsErrorBoundary>
                    <AnalyticsBootstrap />
                    <View style={layoutStyles.root}>
                      <OfflineBanner />
                      <ThemedStack />
                    </View>
                  </AnalyticsErrorBoundary>
                </AgeGate>
              </TransportProvider>
            </AnalyticsProvider>
          </AuthProvider>
        </QueryClientProvider>
      </I18nProvider>
    </ThemeProvider>
  );
});

const layoutStyles = StyleSheet.create({
  root: {
    flex: 1,
  },
});
