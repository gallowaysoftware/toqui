import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import { useCallback } from "react";
import { Platform } from "react-native";
import { useAuth } from "./auth";
import { getConfig } from "./config";

// Complete the auth session for native popup flows.
WebBrowser.maybeCompleteAuthSession();

// Facebook OAuth discovery document
const discovery: AuthSession.DiscoveryDocument = {
  authorizationEndpoint: "https://www.facebook.com/v19.0/dialog/oauth",
  tokenEndpoint: "https://graph.facebook.com/v19.0/oauth/access_token",
};

export function useFacebookAuth() {
  const { facebookLogin } = useAuth();
  const { facebookClientId, apiUrl } = getConfig();

  const redirectUri = Platform.OS === "web"
    ? `${window.location.origin}/auth/callback`
    : AuthSession.makeRedirectUri({ scheme: "toqui" });

  const [request, , promptAsync] = AuthSession.useAuthRequest(
    {
      clientId: facebookClientId,
      scopes: ["email", "public_profile"],
      redirectUri,
      responseType: AuthSession.ResponseType.Token,
    },
    discovery,
  );

  const signIn = useCallback(async () => {
    if (Platform.OS === "web") {
      // On web, redirect to the backend's Facebook OAuth endpoint.
      // The backend handles the full OAuth flow (authorization + token exchange)
      // and redirects back to /auth/callback with tokens.
      window.location.href = `${apiUrl}/auth/facebook/login?redirect_uri=${encodeURIComponent(window.location.origin + "/auth/callback")}`;
      return;
    }

    // Native: popup flow to get Facebook access token
    const result = await promptAsync();
    if (result?.type === "success" && result.params.access_token) {
      try {
        await facebookLogin(result.params.access_token);
      } catch (err) {
        console.error("Facebook login failed:", err);
      }
    }
  }, [promptAsync, facebookLogin, apiUrl]);

  return {
    signIn,
    isReady: Platform.OS === "web" ? true : !!request,
    redirectUri,
  };
}
