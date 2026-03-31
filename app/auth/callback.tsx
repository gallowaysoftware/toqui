import { View, Text, StyleSheet, ActivityIndicator, Pressable } from "react-native";
import { useEffect, useState } from "react";
import { Platform } from "react-native";
import { useRouter } from "expo-router";
import * as WebBrowser from "expo-web-browser";
import { useAuth } from "@/lib/auth";
import { useTheme } from "@/lib/theme";

// Attempt to complete the auth session via the popup postMessage flow.
// If window.opener is available (popup not severed by COOP), this resolves
// the promptAsync() promise in the parent window and we're done.
WebBrowser.maybeCompleteAuthSession();

export default function AuthCallbackScreen() {
  const { login } = useAuth();
  const router = useRouter();
  const { colors } = useTheme();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Fallback for when the popup flow fails (e.g. Google's COOP headers
    // sever window.opener). In that case maybeCompleteAuthSession() above
    // couldn't post the result back, so the popup stays open showing this
    // page. We extract the auth code from the URL and complete login here.
    if (Platform.OS !== "web") return;

    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    if (!code) return;

    // If maybeCompleteAuthSession() already handled it (popup closed), this
    // page won't be visible anyway. So it's safe to always attempt login.
    const redirectUri = `${window.location.origin}/auth/callback`;

    login(code, redirectUri)
      .then(() => {
        router.replace("/");
      })
      .catch((err) => {
        console.error("OAuth callback login failed:", err);
        setError("Sign-in failed. Please try again.");
      });
  }, [login, router]);

  const styles = StyleSheet.create({
    container: { flex: 1, justifyContent: "center", alignItems: "center", gap: 16, backgroundColor: colors.surface },
    text: { fontSize: 16, color: colors.textSecondary },
    errorText: { fontSize: 16, color: colors.error, textAlign: "center", paddingHorizontal: 24 },
    retryButton: {
      backgroundColor: colors.accent,
      borderRadius: 8,
      paddingVertical: 12,
      paddingHorizontal: 24,
    },
    retryText: { color: "#fff", fontSize: 16, fontWeight: "600" },
  });

  if (error) {
    return (
      <View style={styles.container}>
        <Text style={styles.errorText}>{error}</Text>
        <Pressable style={styles.retryButton} onPress={() => router.replace("/")}>
          <Text style={styles.retryText}>Back to home</Text>
        </Pressable>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <ActivityIndicator size="large" color={colors.accent} />
      <Text style={styles.text}>Signing you in...</Text>
    </View>
  );
}
