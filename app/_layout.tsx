import { useEffect, useState } from "react";
import { View, StyleSheet } from "react-native";
import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "@/lib/auth";
import { TransportProvider } from "@/lib/transport";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { I18nProvider } from "@/lib/i18n";
import { ThemeProvider, useTheme } from "@/lib/theme";
import { AIDisclaimerGate } from "@/components/auth/AIDisclaimerGate";
import { OfflineBanner } from "@/components/OfflineBanner";
import { loadConfig, onConfigChange } from "@/lib/config";

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
        <Stack.Screen name="server-setup" options={{ headerShown: false }} />
      </Stack>
    </>
  );
}

export default function RootLayout() {
  const [configLoaded, setConfigLoaded] = useState(false);
  // Bumped when the user switches servers (native bring-your-own-server).
  // Keying the provider tree on it remounts everything, so the ConnectRPC
  // transports are rebuilt against the new API URL.
  const [configEpoch, setConfigEpoch] = useState(0);

  useEffect(() => {
    loadConfig().then(() => {
      setConfigLoaded(true);
    });
  }, []);

  useEffect(() => onConfigChange(() => setConfigEpoch((n) => n + 1)), []);

  if (!configLoaded) return null;

  return (
    <ThemeProvider key={configEpoch}>
      <I18nProvider>
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <TransportProvider>
              <AIDisclaimerGate>
                <ErrorBoundary>
                  <View style={layoutStyles.root}>
                    <OfflineBanner />
                    <ThemedStack />
                  </View>
                </ErrorBoundary>
              </AIDisclaimerGate>
            </TransportProvider>
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
