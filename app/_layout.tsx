import { Stack } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { AuthProvider } from "@/lib/auth";
import { TransportProvider } from "@/lib/transport";
import { I18nProvider } from "@/lib/i18n";

export default function RootLayout() {
  return (
    <I18nProvider>
      <AuthProvider>
        <TransportProvider>
          <StatusBar style="light" />
          <Stack
            screenOptions={{
              headerStyle: { backgroundColor: "#e8654a" },
              headerTintColor: "#fff",
              headerTitleStyle: { fontWeight: "bold" },
            }}
          >
            <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
            <Stack.Screen name="trips/[tripId]" options={{ headerShown: false }} />
            <Stack.Screen name="auth/callback" options={{ headerShown: false }} />
            <Stack.Screen name="shared/[token]" options={{ title: "Shared Trip" }} />
            <Stack.Screen name="privacy" options={{ title: "Privacy Policy" }} />
            <Stack.Screen name="terms" options={{ title: "Terms of Service" }} />
            <Stack.Screen name="waitlist" options={{ title: "Waitlist" }} />
          </Stack>
        </TransportProvider>
      </AuthProvider>
    </I18nProvider>
  );
}
