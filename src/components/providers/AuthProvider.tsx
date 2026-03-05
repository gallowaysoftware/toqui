"use client";

import { createContext, useContext, useEffect, useState, useCallback, useRef } from "react";
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { AuthService } from "@/gen/toqui/v1/auth_pb";

interface User {
  id: string;
  email: string;
  name: string;
  avatarUrl: string;
}

interface AuthContextValue {
  user: User | null;
  accessToken: string | null;
  isLoading: boolean;
  login: () => void;
  logout: () => void;
  setTokens: (accessToken: string, refreshToken: string, user: User) => void;
  refreshAccessToken: () => Promise<string | null>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}

function getTokenExpiry(token: string): number | null {
  try {
    const payload = JSON.parse(atob(token.split(".")[1]));
    return payload.exp ? payload.exp * 1000 : null;
  } catch {
    return null;
  }
}

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8090";

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [accessToken, setAccessToken] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const refreshTokenRef = useRef<string | null>(null);
  const isRefreshingRef = useRef<Promise<string | null> | null>(null);

  const clearAuth = useCallback(() => {
    setUser(null);
    setAccessToken(null);
    refreshTokenRef.current = null;
    localStorage.removeItem("toqui_auth");
    if (refreshTimerRef.current) {
      clearTimeout(refreshTimerRef.current);
    }
  }, []);

  const scheduleRefresh = useCallback((token: string, doRefresh: () => Promise<string | null>) => {
    if (refreshTimerRef.current) {
      clearTimeout(refreshTimerRef.current);
    }
    const expiry = getTokenExpiry(token);
    if (!expiry) return;

    // Refresh 5 minutes before expiry
    const delay = expiry - Date.now() - 5 * 60 * 1000;
    if (delay <= 0) {
      // Already near expiry, refresh immediately
      doRefresh();
      return;
    }
    refreshTimerRef.current = setTimeout(() => doRefresh(), delay);
  }, []);

  const refreshAccessToken = useCallback(async (): Promise<string | null> => {
    const rt = refreshTokenRef.current;
    if (!rt) return null;

    // Deduplicate concurrent refresh attempts
    if (isRefreshingRef.current) return isRefreshingRef.current;

    const promise = (async () => {
      try {
        const transport = createConnectTransport({ baseUrl: API_URL });
        const client = createClient(AuthService, transport);
        const res = await client.refreshToken({ refreshToken: rt });

        const newUser: User = {
          id: res.user?.id ?? "",
          email: res.user?.email ?? "",
          name: res.user?.name ?? "",
          avatarUrl: res.user?.avatarUrl ?? "",
        };

        setAccessToken(res.accessToken);
        setUser(newUser);
        refreshTokenRef.current = res.refreshToken;
        localStorage.setItem(
          "toqui_auth",
          JSON.stringify({ accessToken: res.accessToken, refreshToken: res.refreshToken, user: newUser }),
        );

        scheduleRefresh(res.accessToken, refreshAccessToken);
        return res.accessToken;
      } catch {
        clearAuth();
        return null;
      } finally {
        isRefreshingRef.current = null;
      }
    })();

    isRefreshingRef.current = promise;
    return promise;
  }, [clearAuth, scheduleRefresh]);

  useEffect(() => {
    const stored = localStorage.getItem("toqui_auth");
    if (stored) {
      try {
        const parsed = JSON.parse(stored);
        setUser(parsed.user);
        setAccessToken(parsed.accessToken);
        refreshTokenRef.current = parsed.refreshToken ?? null;

        if (parsed.accessToken) {
          scheduleRefresh(parsed.accessToken, refreshAccessToken);
        }
      } catch {
        localStorage.removeItem("toqui_auth");
      }
    }
    setIsLoading(false);
  }, [scheduleRefresh, refreshAccessToken]);

  useEffect(() => {
    return () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current);
    };
  }, []);

  const login = useCallback(() => {
    window.location.href = `${API_URL}/auth/google/login`;
  }, []);

  const setTokens = useCallback((accessToken: string, refreshToken: string, user: User) => {
    setUser(user);
    setAccessToken(accessToken);
    refreshTokenRef.current = refreshToken;
    localStorage.setItem(
      "toqui_auth",
      JSON.stringify({ accessToken, refreshToken, user }),
    );
    scheduleRefresh(accessToken, refreshAccessToken);
  }, [scheduleRefresh, refreshAccessToken]);

  return (
    <AuthContext.Provider value={{ user, accessToken, isLoading, login, logout: clearAuth, setTokens, refreshAccessToken }}>
      {children}
    </AuthContext.Provider>
  );
}
