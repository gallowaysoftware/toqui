"use client";

import {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  useRef,
} from "react";

interface User {
  id: string;
  email: string;
  name: string;
  avatarUrl: string;
}

interface AuthContextValue {
  user: User | null;
  isLoading: boolean;
  login: () => void;
  logout: () => Promise<void>;
  setSession: (user: User, expiresAt: number) => void;
  refreshAccessToken: () => Promise<boolean>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}

const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8090";

// Storage key for user info only (no tokens — tokens are in HttpOnly cookies).
const USER_STORAGE_KEY = "toqui_user";

interface StoredSession {
  user: User;
  expiresAt: number; // Unix timestamp (seconds) of access token expiry
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(
    undefined,
  );
  const isRefreshingRef = useRef<Promise<boolean> | null>(null);

  const clearAuth = useCallback(() => {
    setUser(null);
    localStorage.removeItem(USER_STORAGE_KEY);
    if (refreshTimerRef.current) {
      clearTimeout(refreshTimerRef.current);
    }
  }, []);

  const scheduleRefresh = useCallback(
    (expiresAt: number, doRefresh: () => Promise<boolean>) => {
      if (refreshTimerRef.current) {
        clearTimeout(refreshTimerRef.current);
      }

      // expiresAt is Unix timestamp in seconds; convert to ms
      const expiryMs = expiresAt * 1000;

      // Refresh 5 minutes before expiry
      const delay = expiryMs - Date.now() - 5 * 60 * 1000;
      if (delay <= 0) {
        // Already near expiry, refresh immediately
        void doRefresh();
        return;
      }
      refreshTimerRef.current = setTimeout(() => {
        void doRefresh();
      }, delay);
    },
    [],
  );

  const refreshAccessToken = useCallback(async (): Promise<boolean> => {
    // Deduplicate concurrent refresh attempts
    if (isRefreshingRef.current) return isRefreshingRef.current;

    const promise = (async () => {
      try {
        const res = await fetch(`${API_URL}/auth/refresh`, {
          method: "POST",
          credentials: "include",
        });

        if (!res.ok) {
          clearAuth();
          return false;
        }

        const data = await res.json();

        // Validate expires_at to prevent infinite refresh loops.
        const expiresAt = data.expires_at;
        if (typeof expiresAt !== "number" || expiresAt <= 0) {
          clearAuth();
          return false;
        }

        const newUser: User = {
          id: data.user?.id ?? "",
          email: data.user?.email ?? "",
          name: data.user?.name ?? "",
          avatarUrl: data.user?.avatar_url ?? "",
        };

        setUser(newUser);
        localStorage.setItem(
          USER_STORAGE_KEY,
          JSON.stringify({
            user: newUser,
            expiresAt,
          } satisfies StoredSession),
        );

        scheduleRefresh(expiresAt, refreshAccessToken);
        return true;
      } catch {
        clearAuth();
        return false;
      } finally {
        isRefreshingRef.current = null;
      }
    })();

    isRefreshingRef.current = promise;
    return promise;
  }, [clearAuth, scheduleRefresh]);

  // Hydrate user from localStorage on mount.
  // Tokens are in HttpOnly cookies (not in localStorage).
  useEffect(() => {
    const stored = localStorage.getItem(USER_STORAGE_KEY);
    if (stored) {
      try {
        const parsed: StoredSession = JSON.parse(stored);
        setUser(parsed.user);

        if (parsed.expiresAt) {
          scheduleRefresh(parsed.expiresAt, refreshAccessToken);
        }
      } catch {
        localStorage.removeItem(USER_STORAGE_KEY);
      }
    }
    setIsLoading(false);
  }, [scheduleRefresh, refreshAccessToken]);

  // Cleanup timer on unmount.
  useEffect(() => {
    return () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current);
    };
  }, []);

  const login = useCallback(() => {
    window.location.href = `${API_URL}/auth/google/login`;
  }, []);

  const logout = useCallback(async () => {
    // Revoke refresh token + clear cookies server-side.
    try {
      await fetch(`${API_URL}/auth/logout`, {
        method: "POST",
        credentials: "include",
      });
    } catch {
      // Best-effort — clear local state regardless.
    }
    clearAuth();
  }, [clearAuth]);

  const setSession = useCallback(
    (newUser: User, expiresAt: number) => {
      setUser(newUser);
      localStorage.setItem(
        USER_STORAGE_KEY,
        JSON.stringify({ user: newUser, expiresAt } satisfies StoredSession),
      );
      scheduleRefresh(expiresAt, refreshAccessToken);
    },
    [scheduleRefresh, refreshAccessToken],
  );

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        login,
        logout,
        setSession,
        refreshAccessToken,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
