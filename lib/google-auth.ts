import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import { useCallback } from "react";
import { Platform } from "react-native";
import { useAuth } from "./auth";
import { useAnalytics } from "./analytics";
import { getConfig } from "./config";

// Complete the auth session for native popup flows.
WebBrowser.maybeCompleteAuthSession();

// Google's well-known discovery endpoints
const discovery: AuthSession.DiscoveryDocument = {
  authorizationEndpoint: "https://accounts.google.com/o/oauth2/v2/auth",
  tokenEndpoint: "https://oauth2.googleapis.com/token",
  revocationEndpoint: "https://oauth2.googleapis.com/revoke",
};

export function useGoogleAuth() {
  const { login } = useAuth();
  const { track } = useAnalytics();
  const { googleClientId } = getConfig();

  // On web, use the explicit origin to match Google Console's authorized redirect URIs.
  // On native, use the scheme-based URI for deep linking.
  const redirectUri = Platform.OS === "web"
    ? `${window.location.origin}/auth/callback`
    : AuthSession.makeRedirectUri({ scheme: "toqui" });

  const [request, , promptAsync] = AuthSession.useAuthRequest(
    {
      clientId: googleClientId,
      scopes: ["openid", "profile", "email"],
      redirectUri,
      responseType: AuthSession.ResponseType.Code,
      // PKCE is disabled on web since we use full-page redirect and the
      // backend exchanges the code with its own client_secret. On native
      // the popup flow keeps the code_verifier in memory so PKCE works.
      usePKCE: Platform.OS !== "web",
    },
    discovery,
  );

  const signIn = useCallback(async () => {
    track("signup_started", { method: "google", platform: Platform.OS });

    if (Platform.OS === "web") {
      // Full-page redirect instead of popup. Google's COOP headers sever
      // window.opener in popups, breaking expo-auth-session's postMessage
      // flow. Redirecting the whole page avoids cross-window communication.
      // The /auth/callback page handles the code exchange on return.
      if (request?.url) {
        window.location.href = request.url;
      }
      return;
    }

    // Native: popup flow works fine
    const result = await promptAsync();
    if (result?.type === "success" && result.params.code) {
      try {
        await login(result.params.code, redirectUri);
      } catch (err) {
        console.error("Google login failed:", err);
      }
    }
  }, [promptAsync, login, redirectUri, request, track]);

  return {
    signIn,
    isReady: !!request,
    redirectUri,
  };
}
