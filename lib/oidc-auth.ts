import * as AuthSession from "expo-auth-session";
import * as WebBrowser from "expo-web-browser";
import { useCallback } from "react";
import { Platform } from "react-native";
import { useAuth } from "./auth";
import { useAuthProviders } from "./hooks/useAuthProviders";

// Complete the auth session for native popup flows.
WebBrowser.maybeCompleteAuthSession();

// Marker the web /auth/callback page reads to know a returning `?code=` belongs
// to the OIDC flow rather than Google: the full-page redirect discards the
// in-memory auth request, and both providers share the same callback URI.
export const OIDC_PENDING_KEY = "toqui_oidc_pending";

/**
 * Generic OIDC/SSO sign-in, mirroring `useGoogleAuth` but driven by the
 * server's advertised provider (issuer + client id from `GetAuthProviders`)
 * instead of a build-time constant. Discovery runs against the issuer's
 * `.well-known/openid-configuration`.
 *
 * IMPORTANT: only mount this hook when the server reports OIDC enabled (see
 * `OIDCSignInButton`, rendered conditionally). With no issuer there is nothing
 * to discover, and mounting it unconditionally would fire a spurious discovery
 * request on every signed-out screen.
 *
 * Web uses a full-page redirect with no PKCE — the confidential-client backend
 * holds the client secret and the redirect drops the in-memory verifier, same
 * as the Google path. Native keeps the PKCE verifier in the popup's memory.
 */
export function useOIDCAuth() {
  const { loginWithOIDC } = useAuth();
  const { data: providers } = useAuthProviders();
  const oidc = providers?.oidc;
  const enabled = oidc?.enabled === true;
  const issuer = oidc?.issuer ?? "";
  const clientId = oidc?.clientId ?? "";
  const name = oidc?.name || "SSO";

  const discovery = AuthSession.useAutoDiscovery(issuer);

  const redirectUri =
    Platform.OS === "web"
      ? typeof window !== "undefined"
        ? `${window.location.origin}/auth/callback`
        : ""
      : AuthSession.makeRedirectUri({ scheme: "toqui" });

  const [request, , promptAsync] = AuthSession.useAuthRequest(
    {
      clientId,
      scopes: ["openid", "profile", "email"],
      redirectUri,
      responseType: AuthSession.ResponseType.Code,
      // PKCE on native only; on web the full-page redirect loses the
      // code_verifier and the backend exchanges with its client secret.
      usePKCE: Platform.OS !== "web",
    },
    discovery,
  );

  const signIn = useCallback(async () => {
    if (!enabled || !request) return;

    if (Platform.OS === "web") {
      // Full-page redirect (Google-style) to avoid COOP severing the popup.
      // The /auth/callback page completes the code exchange on return.
      if (request.url) {
        try {
          sessionStorage.setItem(OIDC_PENDING_KEY, "1");
        } catch {
          // sessionStorage unavailable (private mode / disabled). The
          // callback falls back to the Google exchange; still redirect so the
          // user isn't stranded (login just fails clearly instead of silently).
        }
        window.location.href = request.url;
      }
      return;
    }

    const result = await promptAsync();
    if (result?.type === "success" && result.params.code) {
      try {
        await loginWithOIDC(result.params.code, redirectUri, request.codeVerifier);
      } catch (err) {
        console.error("OIDC login failed:", err);
      }
    }
  }, [enabled, request, promptAsync, loginWithOIDC, redirectUri]);

  return {
    signIn,
    isReady: enabled && !!request,
    name,
    enabled,
  };
}
