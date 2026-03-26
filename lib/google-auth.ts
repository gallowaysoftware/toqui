import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import { useCallback } from "react";
import { useAuth } from "./auth";

// Complete the auth session on web (needed for redirect-based flows)
WebBrowser.maybeCompleteAuthSession();

const GOOGLE_CLIENT_ID = process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID ?? "";

// Google OAuth discovery document
const discovery = AuthSession.useAutoDiscovery
  ? undefined // Let expo-auth-session auto-discover
  : {
      authorizationEndpoint: "https://accounts.google.com/o/oauth2/v2/auth",
      tokenEndpoint: "https://oauth2.googleapis.com/token",
      revocationEndpoint: "https://oauth2.googleapis.com/revoke",
    };

export function useGoogleAuth() {
  const { login } = useAuth();

  const redirectUri = AuthSession.makeRedirectUri({
    scheme: "toqui",
  });

  const [request, response, promptAsync] = AuthSession.useAuthRequest(
    {
      clientId: GOOGLE_CLIENT_ID,
      scopes: ["openid", "profile", "email"],
      redirectUri,
      responseType: AuthSession.ResponseType.Code,
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
