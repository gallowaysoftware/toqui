import { useEffect, useState } from "react";
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
import { loadConfig } from "@/lib/config";

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
        <Stack.Screen name="shared/[token]" options={{ title: "Shared Trip" }} />
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
          error_message: error.message,
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

export default function RootLayout() {
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
}

const layoutStyles = StyleSheet.create({
  root: {
    flex: 1,
  },
});
