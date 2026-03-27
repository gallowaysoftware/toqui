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
import { AuthService } from "@gen/toqui/v1/auth_pb";

import { getConfig } from "./config";

// Token storage: SecureStore on native (Keychain/Keystore), sessionStorage on web.
// sessionStorage is used instead of localStorage so tokens don't persist across
// browser sessions, reducing the window for XSS token theft.
// TODO: For web, consider keeping the HttpOnly cookie flow from the backend
// and only using Bearer tokens on native platforms.
const tokenStorage = {
  async get(key: string): Promise<string | null> {
    if (Platform.OS === "web") {
      return sessionStorage.getItem(key);
    }
    const { getItemAsync } = await import("expo-secure-store");
    return getItemAsync(key);
  },
  async set(key: string, value: string): Promise<void> {
    if (Platform.OS === "web") {
      sessionStorage.setItem(key, value);
      return;
    }
    const { setItemAsync } = await import("expo-secure-store");
    await setItemAsync(key, value);
  },
  async delete(key: string): Promise<void> {
    if (Platform.OS === "web") {
      sessionStorage.removeItem(key);
      return;
    }
    const { deleteItemAsync } = await import("expo-secure-store");
    await deleteItemAsync(key);
  },
};

interface AuthState {
  accessToken: string | null;
  refreshToken: string | null;
  isLoading: boolean;
  login: (googleAuthCode: string, redirectUri?: string) => Promise<void>;
  user: { id: string; email: string; name: string } | null;
  logout: () => Promise<void>;
  refreshTokens: () => Promise<string | null>;
  setTokensManually: (access: string, refresh: string) => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [refreshToken, setRefreshToken] = useState<string | null>(null);
  const [user, setUser] = useState<{ id: string; email: string; name: string } | null>(null);
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
        try { setUser(JSON.parse(userJson)); } catch { /* ignore corrupt data */ }
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
      const u = { id: res.user.id, email: res.user.email, name: res.user.name };
      setUser(u);
      await tokenStorage.set("toqui_user", JSON.stringify(u));
    }
    await tokenStorage.set("toqui_access_token", res.accessToken);
    await tokenStorage.set("toqui_refresh_token", res.refreshToken);
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
      await tokenStorage.set("toqui_access_token", res.accessToken);
      await tokenStorage.set("toqui_refresh_token", res.refreshToken);
      return res.accessToken;
    } catch (err) {
      console.error("Token refresh failed:", err);
      // Clear stale tokens
      setAccessToken(null);
      setRefreshToken(null);
      await tokenStorage.delete("toqui_access_token");
      await tokenStorage.delete("toqui_refresh_token");
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
