import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import { useCallback } from "react";
import { Platform } from "react-native";
import { useAuth } from "./auth";
import { getConfig } from "./config";

// Complete the auth session on web (needed for redirect-based flows)
WebBrowser.maybeCompleteAuthSession();

// Google's well-known discovery endpoints
const discovery: AuthSession.DiscoveryDocument = {
  authorizationEndpoint: "https://accounts.google.com/o/oauth2/v2/auth",
  tokenEndpoint: "https://oauth2.googleapis.com/token",
  revocationEndpoint: "https://oauth2.googleapis.com/revoke",
};

export function useGoogleAuth() {
  const { login } = useAuth();
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
      usePKCE: true,
    },
    discovery,
  );

  const signIn = useCallback(async () => {
    const result = await promptAsync();
    if (result?.type === "success" && result.params.code) {
      await login(result.params.code, redirectUri);
    }
  }, [promptAsync, login, redirectUri]);

  return {
    signIn,
    isReady: !!request,
    redirectUri,
  };
}
