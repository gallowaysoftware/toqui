import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  useMemo,
} from "react";
import { Platform } from "react-native";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { Timestamp } from "@bufbuild/protobuf/wkt";
import { AuthService } from "@gen/toqui/v1/auth_pb";

import { getConfig } from "./config";

// Token storage: SecureStore on native (Keychain/Keystore), localStorage on web.
// localStorage persists across browser sessions so users stay logged in between
// visits. The refresh token has a 30-day server-side expiry which bounds the
// persistence window. This is an acceptable tradeoff for a single-origin app
// with no third-party scripts — the XSS surface is minimal and the UX gain
// (not forcing re-login on every tab close) is significant.
const tokenStorage = {
  async get(key: string): Promise<string | null> {
    if (Platform.OS === "web") {
      return localStorage.getItem(key);
    }
    const { getItemAsync } = await import("expo-secure-store");
    return getItemAsync(key);
  },
  async set(key: string, value: string): Promise<void> {
    if (Platform.OS === "web") {
      localStorage.setItem(key, value);
      return;
    }
    const { setItemAsync } = await import("expo-secure-store");
    await setItemAsync(key, value);
  },
  async delete(key: string): Promise<void> {
    if (Platform.OS === "web") {
      localStorage.removeItem(key);
      return;
    }
    const { deleteItemAsync } = await import("expo-secure-store");
    await deleteItemAsync(key);
  },
};

export type SubscriptionTier = "free" | "pro";

export interface AuthUser {
  id: string;
  email: string;
  name: string;
  tier: SubscriptionTier;
  // ISO 8601 string (or null) — when the user completed age verification
  // on the backend. Set by login/refresh from User.age_verified_at on the
  // proto. Consumed by AgeGate to skip the modal for returning users who
  // verified on another device/session.
  ageVerifiedAt: string | null;
}

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  isLoading: boolean;
  login: (googleAuthCode: string, redirectUri?: string) => Promise<{ consentPending: boolean }>;
  user: AuthUser | null;
  logout: () => Promise<void>;
  refreshTokens: () => Promise<string | null>;
  setTokensManually: (access: string, refresh: string) => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

// Convert a google.protobuf.Timestamp (or any object with a toDate() method,
// which is how our tests stub it) to an ISO-8601 string. Returns null for
// unset / nullish inputs. Kept forgiving of shape so it works with both the
// real @bufbuild/protobuf Timestamp and simple test doubles.
function toIsoOrNull(
  ts: Timestamp | { toDate: () => Date } | undefined | null,
): string | null {
  if (!ts) return null;
  if (typeof (ts as { toDate?: unknown }).toDate === "function") {
    return (ts as { toDate: () => Date }).toDate().toISOString();
  }
  try {
    return timestampDate(ts as Timestamp).toISOString();
  } catch {
    return null;
  }
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [refreshToken, setRefreshToken] = useState<string | null>(null);
  const [user, setUser] = useState<AuthUser | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Load persisted tokens on mount
  useEffect(() => {
    (async () => {
      const [at, rt, userJson] = await Promise.all([
        tokenStorage.get("toqui_access_token"),
        tokenStorage.get("toqui_refresh_token"),
        tokenStorage.get("toqui_user"),
      ]);
      if (at) setAccessToken(at);
      if (rt) setRefreshToken(rt);
      if (userJson) {
        try {
          const parsed = JSON.parse(userJson);
          const tier = parsed.tier === "pro" ? "pro" : "free" as const;
          const ageVerifiedAt =
            typeof parsed.ageVerifiedAt === "string" ? parsed.ageVerifiedAt : null;
          setUser({ ...parsed, tier, ageVerifiedAt });
        } catch { /* ignore corrupt data */ }
      }
      setIsLoading(false);
    })();
  }, []);

  const setTokensManually = useCallback(
    async (access: string, refresh: string) => {
      setAccessToken(access);
      setRefreshToken(refresh);
      await tokenStorage.set("toqui_access_token", access);
      await tokenStorage.set("toqui_refresh_token", refresh);
    },
    [],
  );

  const login = useCallback(async (googleAuthCode: string, redirectUri?: string) => {
    const transport = createConnectTransport({ baseUrl: getConfig().apiUrl });
    const client = createClient(AuthService, transport);
    const res = await client.googleLogin({
      code: googleAuthCode,
      redirectUri: redirectUri ?? "",
    });
    setAccessToken(res.accessToken);
    setRefreshToken(res.refreshToken);
    if (res.user) {
      const tier = res.user.subscriptionTier === "pro" ? "pro" : "free" as const;
      const ageVerifiedAt = toIsoOrNull(res.user.ageVerifiedAt);
      const u: AuthUser = {
        id: res.user.id,
        email: res.user.email,
        name: res.user.name,
        tier,
        ageVerifiedAt,
      };
      setUser(u);
      await tokenStorage.set("toqui_user", JSON.stringify(u));
    }
    await tokenStorage.set("toqui_access_token", res.accessToken);
    await tokenStorage.set("toqui_refresh_token", res.refreshToken);
    // Surface consentPending so callers (auth/callback.tsx) can fire
    // the right analytics event — signup_completed for first-time
    // users (consent not yet recorded), signin_completed otherwise.
    // Pre-fix this leaked signup_completed on every returning sign-in
    // (toqui#190 LB-8) which polluted the funnel-conversion metric.
    return { consentPending: res.consentPending };
  }, []);

  const refreshTokens = useCallback(async (): Promise<string | null> => {
    const rt = await tokenStorage.get("toqui_refresh_token");
    if (!rt) return null;
    try {
      const transport = createConnectTransport({ baseUrl: getConfig().apiUrl });
      const client = createClient(AuthService, transport);
      const res = await client.refreshToken({ refreshToken: rt });
      setAccessToken(res.accessToken);
      setRefreshToken(res.refreshToken);
      // Sync the user snapshot from the refresh response so server-side
      // state (tier upgrades, age_verified_at) propagates without requiring
      // the user to sign back in.
      if (res.user) {
        const tier = res.user.subscriptionTier === "pro" ? "pro" : "free" as const;
        const ageVerifiedAt = toIsoOrNull(res.user.ageVerifiedAt);
        const u: AuthUser = {
          id: res.user.id,
          email: res.user.email,
          name: res.user.name,
          tier,
          ageVerifiedAt,
        };
        setUser(u);
        await tokenStorage.set("toqui_user", JSON.stringify(u));
      }
      await tokenStorage.set("toqui_access_token", res.accessToken);
      await tokenStorage.set("toqui_refresh_token", res.refreshToken);
      return res.accessToken;
    } catch (err) {
      console.error("Token refresh failed:", err);
      // Clear all stale auth state — tokens AND user
      setAccessToken(null);
      setRefreshToken(null);
      setUser(null);
      await tokenStorage.delete("toqui_access_token");
      await tokenStorage.delete("toqui_refresh_token");
      await tokenStorage.delete("toqui_user");
      return null;
    }
  }, []);

  const logout = useCallback(async () => {
    setAccessToken(null);
    setRefreshToken(null);
    setUser(null);
    await Promise.all([
      tokenStorage.delete("toqui_access_token"),
      tokenStorage.delete("toqui_refresh_token"),
      tokenStorage.delete("toqui_user"),
    ]);
  }, []);

  const value = useMemo(
    () => ({ accessToken, refreshToken, user, isLoading, login, logout, refreshTokens, setTokensManually }),
    [accessToken, refreshToken, user, isLoading, login, logout, refreshTokens, setTokensManually],
  );

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}
