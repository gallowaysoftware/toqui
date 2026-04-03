import { useEffect, useState } from "react";
import { Platform, View, StyleSheet } from "react-native";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "@/lib/auth";
import { TransportProvider } from "@/lib/transport";
import { I18nProvider } from "@/lib/i18n";
import { ThemeProvider, useTheme } from "@/lib/theme";
import { AgeGate } from "@/components/auth/AgeGate";
import { OfflineBanner } from "@/components/OfflineBanner";
import { loadConfig } from "@/lib/config";
import * as Sentry from "@sentry/react-native";

Sentry.init({
  dsn: "https://9a4287e61882165484eb43ee7e4b100f@o4511157132001280.ingest.de.sentry.io/4511157139013712",

  // Privacy: do NOT send PII (emails, IPs, cookies)
  sendDefaultPii: false,

  enableLogs: true,

  // Session Replay: mask all text for privacy (travel data is sensitive)
  replaysSessionSampleRate: 0.1,
  replaysOnErrorSampleRate: 1,
  integrations: [
    Sentry.mobileReplayIntegration({
      maskAllText: true,
      maskAllImages: false,
    }),
    Sentry.feedbackIntegration(),
  ],

  // Privacy: strip PII from error reports before sending
  beforeSend(event) {
    if (event.user) {
      delete event.user.email;
      delete event.user.username;
      delete event.user.ip_address;
    }
    // Remove breadcrumb data that might contain travel info
    if (event.breadcrumbs) {
      event.breadcrumbs = event.breadcrumbs.map(b => ({
        ...b,
        data: undefined,
      }));
    }
    return event;
  },
});

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

export default Sentry.wrap(function RootLayout() {
  const [configLoaded, setConfigLoaded] = useState(false);

  useEffect(() => {
    loadConfig().then(() => setConfigLoaded(true));
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
            <TransportProvider>
              <AgeGate>
                <View style={layoutStyles.root}>
                  <OfflineBanner />
                  <ThemedStack />
                </View>
              </AgeGate>
            </TransportProvider>
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
