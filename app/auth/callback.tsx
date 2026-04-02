import { View, Text, StyleSheet, ActivityIndicator, Pressable } from "react-native";
import { useEffect, useState } from "react";
import { Platform } from "react-native";
import { useRouter } from "expo-router";
import * as WebBrowser from "expo-web-browser";
import { useAuth } from "@/lib/auth";
import { authFetch } from "@/lib/authFetch";
import { getConfig } from "@/lib/config";

// Attempt to complete the auth session via the popup postMessage flow.
// If window.opener is available (popup not severed by COOP), this resolves
// the promptAsync() promise in the parent window and we're done.
WebBrowser.maybeCompleteAuthSession();

export default function AuthCallbackScreen() {
  const { login, setTokensManually } = useAuth();
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Fallback for when the popup flow fails (e.g. Google's COOP headers
    // sever window.opener). In that case maybeCompleteAuthSession() above
    // couldn't post the result back, so the popup stays open showing this
    // page. We extract the auth code from the URL and complete login here.
    if (Platform.OS !== "web") return;

    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    const provider = params.get("provider");
    const at = params.get("access_token");
    const rt = params.get("refresh_token");

    // Facebook backend redirect: tokens arrive as query params
    if (provider === "facebook" && at && rt) {
      setTokensManually(at, rt)
        .then(() => {
          const pendingRef = sessionStorage.getItem("toqui_pending_ref");
          if (pendingRef) {
            sessionStorage.removeItem("toqui_pending_ref");
            authFetch(`${getConfig().apiUrl}/api/referral/redeem`, at, {
              method: "POST",
              body: JSON.stringify({ code: pendingRef }),
            }).catch(() => {});
          }
          router.replace("/");
        })
        .catch((err) => {
          console.error("Facebook callback login failed:", err);
          setError("Sign-in failed. Please try again.");
        });
      return;
    }

    // Google OAuth: exchange auth code for tokens
    if (!code) return;

    // If maybeCompleteAuthSession() already handled it (popup closed), this
    // page won't be visible anyway. So it's safe to always attempt login.
    const redirectUri = `${window.location.origin}/auth/callback`;

    login(code, redirectUri)
      .then(() => {
        const pendingRef = sessionStorage.getItem("toqui_pending_ref");
        if (pendingRef) {
          sessionStorage.removeItem("toqui_pending_ref");
          const storedAt = sessionStorage.getItem("toqui_access_token");
          authFetch(`${getConfig().apiUrl}/api/referral/redeem`, storedAt, {
            method: "POST",
            body: JSON.stringify({ code: pendingRef }),
          }).catch(() => {});
        }
        router.replace("/");
      })
      .catch((err) => {
        console.error("OAuth callback login failed:", err);
        setError("Sign-in failed. Please try again.");
      });
  }, [login, setTokensManually, router]);

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
      <ActivityIndicator size="large" color="#BF4028" />
      <Text style={styles.text}>Signing you in...</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, justifyContent: "center", alignItems: "center", gap: 16 },
  text: { fontSize: 16, color: "#666" },
  errorText: { fontSize: 16, color: "#c00", textAlign: "center", paddingHorizontal: 24 },
  retryButton: {
    backgroundColor: "#BF4028",
    borderRadius: 8,
    paddingVertical: 12,
    paddingHorizontal: 24,
  },
  retryText: { color: "#fff", fontSize: 16, fontWeight: "600" },
});
