// CLAIMED BY AGENT 1
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import React from "react";

// ---------------------------------------------------------------------------
// Mocks — must be declared before any import that touches the modules
// ---------------------------------------------------------------------------

// Mock ConnectRPC: createConnectTransport and createClient
const mockGoogleLogin = vi.fn();
const mockRefreshToken = vi.fn();

vi.mock("@connectrpc/connect-web", () => ({
  createConnectTransport: vi.fn(() => "mock-transport"),
}));

vi.mock("@connectrpc/connect", () => ({
  createClient: vi.fn(() => ({
    googleLogin: mockGoogleLogin,
    refreshToken: mockRefreshToken,
  })),
}));

vi.mock("@gen/toqui/v1/auth_pb", () => ({
  AuthService: "MockAuthService",
}));

// We run in jsdom so Platform.OS === "web" and localStorage is used.
// No need to mock expo-secure-store for web path, but mock the module
// in case the import is resolved statically.
vi.mock("expo-secure-store", () => ({
  getItemAsync: vi.fn(),
  setItemAsync: vi.fn(),
  deleteItemAsync: vi.fn(),
}));

// Now import the module under test
import { AuthProvider, useAuth } from "../auth";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function wrapper({ children }: { children: React.ReactNode }) {
  return React.createElement(AuthProvider, null, children);
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("AuthProvider", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
  });

  afterEach(() => {
    localStorage.clear();
  });

  // ── useAuth outside provider ──────────────────────────────────────────

  it("throws when useAuth is called outside AuthProvider", () => {
    // Suppress React error boundary noise
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => renderHook(() => useAuth())).toThrow(
      "useAuth must be used within AuthProvider",
    );
    spy.mockRestore();
  });

  // ── Initial state: no persisted data ──────────────────────────────────

  it("starts in loading state then resolves with null tokens", async () => {
    const { result } = renderHook(() => useAuth(), { wrapper });

    // isLoading should be true initially
    expect(result.current.isLoading).toBe(true);

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.accessToken).toBeNull();
    expect(result.current.refreshToken).toBeNull();
    expect(result.current.user).toBeNull();
  });

  // ── Hydration from localStorage ─────────────────────────────────────

  it("restores tokens and user from localStorage on mount", async () => {
    const user = { id: "u1", email: "a@b.com", name: "Alice" };
    localStorage.setItem("toqui_access_token", "at-123");
    localStorage.setItem("toqui_refresh_token", "rt-456");
    localStorage.setItem("toqui_user", JSON.stringify(user));

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.accessToken).toBe("at-123");
    expect(result.current.refreshToken).toBe("rt-456");
    expect(result.current.user).toEqual({ ...user, ageVerifiedAt: null });
  });

  it("restores ageVerifiedAt from localStorage on mount when present", async () => {
    const user = {
      id: "u1",
      email: "a@b.com",
      name: "Alice",
      ageVerifiedAt: "2026-04-20T12:00:00.000Z",
    };
    localStorage.setItem("toqui_user", JSON.stringify(user));

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    expect(result.current.user?.ageVerifiedAt).toBe("2026-04-20T12:00:00.000Z");
  });

  it("handles corrupt user JSON gracefully on hydration", async () => {
    localStorage.setItem("toqui_access_token", "at-123");
    localStorage.setItem("toqui_refresh_token", "rt-456");
    localStorage.setItem("toqui_user", "NOT-JSON{{{");

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    // Tokens should still load; user should be null because parse failed
    expect(result.current.accessToken).toBe("at-123");
    expect(result.current.user).toBeNull();
  });

  // ── Login flow ────────────────────────────────────────────────────────

  it("login stores tokens and user in state and localStorage", async () => {
    mockGoogleLogin.mockResolvedValueOnce({
      accessToken: "new-at",
      refreshToken: "new-rt",
      user: { id: "u2", email: "b@c.com", name: "Bob" },
    });

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.login("google-code-xyz", "http://redirect");
    });

    expect(result.current.accessToken).toBe("new-at");
    expect(result.current.refreshToken).toBe("new-rt");
    expect(result.current.user).toEqual({
      id: "u2",
      email: "b@c.com",
      name: "Bob",
      ageVerifiedAt: null,
    });

    // Verify persistence
    expect(localStorage.getItem("toqui_access_token")).toBe("new-at");
    expect(localStorage.getItem("toqui_refresh_token")).toBe("new-rt");
    expect(JSON.parse(localStorage.getItem("toqui_user")!)).toEqual({
      id: "u2",
      email: "b@c.com",
      name: "Bob",
      ageVerifiedAt: null,
    });
  });

  it("login maps ageVerifiedAt from Timestamp proto to ISO string", async () => {
    const verifiedDate = new Date("2026-04-20T12:30:00.000Z");
    mockGoogleLogin.mockResolvedValueOnce({
      accessToken: "at",
      refreshToken: "rt",
      user: {
        id: "u3",
        email: "c@d.com",
        name: "Carol",
        ageVerifiedAt: { toDate: () => verifiedDate },
      },
    });

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.login("code");
    });

    expect(result.current.user?.ageVerifiedAt).toBe(verifiedDate.toISOString());
    const stored = JSON.parse(localStorage.getItem("toqui_user")!);
    expect(stored.ageVerifiedAt).toBe(verifiedDate.toISOString());
  });

  it("login maps missing ageVerifiedAt to null", async () => {
    mockGoogleLogin.mockResolvedValueOnce({
      accessToken: "at",
      refreshToken: "rt",
      user: {
        id: "u4",
        email: "d@e.com",
        name: "Dave",
        // ageVerifiedAt absent
      },
    });

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.login("code");
    });

    expect(result.current.user?.ageVerifiedAt).toBeNull();
  });

  it("login with no user in response does not crash and does not store user", async () => {
    mockGoogleLogin.mockResolvedValueOnce({
      accessToken: "at-no-user",
      refreshToken: "rt-no-user",
      user: undefined, // server returned no user
    });

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.login("code");
    });

    expect(result.current.accessToken).toBe("at-no-user");
    expect(result.current.user).toBeNull();
    // user key should NOT have been written
    expect(localStorage.getItem("toqui_user")).toBeNull();
  });

  it("login passes redirectUri to googleLogin RPC", async () => {
    mockGoogleLogin.mockResolvedValueOnce({
      accessToken: "at",
      refreshToken: "rt",
    });

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.login("the-code", "https://app.toqui.travel/callback");
    });

    expect(mockGoogleLogin).toHaveBeenCalledWith({
      code: "the-code",
      redirectUri: "https://app.toqui.travel/callback",
      // attribution is sent on every login (empty string when no
      // launch-campaign cookie is present). The backend treats empty
      // input as "no attribution" — see internal/attribution/Parse().
      attribution: "",
    });
  });

  it("login defaults redirectUri to empty string when omitted", async () => {
    mockGoogleLogin.mockResolvedValueOnce({
      accessToken: "at",
      refreshToken: "rt",
    });

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.login("code-only");
    });

    expect(mockGoogleLogin).toHaveBeenCalledWith({
      code: "code-only",
      redirectUri: "",
      attribution: "",
    });
  });

  // ── Attribution forwarding ────────────────────────────────────────────

  it("login forwards captured attribution from localStorage and clears it", async () => {
    // Marketing site stores attribution as raw JSON in localStorage; the
    // app reads the cookie first, falls back to localStorage and re-encodes.
    const payload = JSON.stringify({
      utm_source: "producthunt",
      utm_medium: "launch",
      captured_at: "2026-05-06T00:00:00.000Z",
    });
    localStorage.setItem("toqui_attribution", payload);

    mockGoogleLogin.mockResolvedValueOnce({
      accessToken: "at",
      refreshToken: "rt",
    });

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.login("code");
    });

    const call = mockGoogleLogin.mock.calls[0][0];
    expect(call.attribution).toBe(btoa(payload));
    // One-shot semantics: localStorage cleared so a returning user
    // doesn't get re-attributed to the original campaign.
    expect(localStorage.getItem("toqui_attribution")).toBeNull();
  });

  // ── Logout flow ───────────────────────────────────────────────────────

  it("logout clears state and all localStorage keys", async () => {
    localStorage.setItem("toqui_access_token", "at");
    localStorage.setItem("toqui_refresh_token", "rt");
    localStorage.setItem("toqui_user", '{"id":"1","email":"a@b","name":"A"}');

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    // Verify hydration first
    expect(result.current.accessToken).toBe("at");

    await act(async () => {
      await result.current.logout();
    });

    expect(result.current.accessToken).toBeNull();
    expect(result.current.refreshToken).toBeNull();
    expect(result.current.user).toBeNull();

    expect(localStorage.getItem("toqui_access_token")).toBeNull();
    expect(localStorage.getItem("toqui_refresh_token")).toBeNull();
    expect(localStorage.getItem("toqui_user")).toBeNull();
  });

  // ── Token refresh ─────────────────────────────────────────────────────

  it("refreshTokens exchanges refresh token and updates both state and storage", async () => {
    localStorage.setItem("toqui_refresh_token", "old-rt");

    mockRefreshToken.mockResolvedValueOnce({
      accessToken: "fresh-at",
      refreshToken: "fresh-rt",
    });

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    let newToken: string | null = null;
    await act(async () => {
      newToken = await result.current.refreshTokens();
    });

    expect(newToken).toBe("fresh-at");
    expect(result.current.accessToken).toBe("fresh-at");
    expect(result.current.refreshToken).toBe("fresh-rt");

    // Storage is also updated
    expect(localStorage.getItem("toqui_access_token")).toBe("fresh-at");
    expect(localStorage.getItem("toqui_refresh_token")).toBe("fresh-rt");
  });

  it("refreshTokens reads refresh token from storage, not from state", async () => {
    // This is important: refreshTokens reads from tokenStorage.get, not
    // from the React state `refreshToken`. This means even if React state
    // is stale (e.g., due to a concurrent update), the storage value is
    // the source of truth.
    localStorage.setItem("toqui_refresh_token", "storage-rt");

    mockRefreshToken.mockResolvedValueOnce({
      accessToken: "at",
      refreshToken: "rt",
    });

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.refreshTokens();
    });

    // The RPC should have been called with the storage value
    expect(mockRefreshToken).toHaveBeenCalledWith({
      refreshToken: "storage-rt",
    });
  });

  it("refreshTokens returns null and clears tokens when no refresh token exists", async () => {
    // No refresh token in storage
    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    let newToken: string | null = "should-be-null";
    await act(async () => {
      newToken = await result.current.refreshTokens();
    });

    expect(newToken).toBeNull();
    expect(mockRefreshToken).not.toHaveBeenCalled();
  });

  it("refreshTokens updates user.ageVerifiedAt from the response", async () => {
    localStorage.setItem("toqui_refresh_token", "old-rt");
    // Seed prior user without age verification
    localStorage.setItem(
      "toqui_user",
      JSON.stringify({ id: "u5", email: "e@f.com", name: "Eve", ageVerifiedAt: null }),
    );

    const verifiedDate = new Date("2026-04-24T10:00:00.000Z");
    mockRefreshToken.mockResolvedValueOnce({
      accessToken: "fresh-at",
      refreshToken: "fresh-rt",
      user: {
        id: "u5",
        email: "e@f.com",
        name: "Eve",
        ageVerifiedAt: { toDate: () => verifiedDate },
      },
    });

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.refreshTokens();
    });

    expect(result.current.user?.ageVerifiedAt).toBe(verifiedDate.toISOString());
    const stored = JSON.parse(localStorage.getItem("toqui_user")!);
    expect(stored.ageVerifiedAt).toBe(verifiedDate.toISOString());
  });

  it("refreshTokens clears all tokens on RPC failure", async () => {
    localStorage.setItem("toqui_access_token", "stale-at");
    localStorage.setItem("toqui_refresh_token", "bad-rt");

    mockRefreshToken.mockRejectedValueOnce(new Error("token revoked"));

    const spy = vi.spyOn(console, "error").mockImplementation(() => {});

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    let newToken: string | null = "should-be-null";
    await act(async () => {
      newToken = await result.current.refreshTokens();
    });

    expect(newToken).toBeNull();
    expect(result.current.accessToken).toBeNull();
    expect(result.current.refreshToken).toBeNull();
    expect(localStorage.getItem("toqui_access_token")).toBeNull();
    expect(localStorage.getItem("toqui_refresh_token")).toBeNull();

    spy.mockRestore();
  });

  // ── setTokensManually ─────────────────────────────────────────────────

  it("setTokensManually updates state and persists to storage", async () => {
    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.setTokensManually("manual-at", "manual-rt");
    });

    expect(result.current.accessToken).toBe("manual-at");
    expect(result.current.refreshToken).toBe("manual-rt");
    expect(localStorage.getItem("toqui_access_token")).toBe("manual-at");
    expect(localStorage.getItem("toqui_refresh_token")).toBe("manual-rt");
  });

  // ── Context memoization ───────────────────────────────────────────────

  it("context value is referentially stable when nothing changes", async () => {
    const { result, rerender } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    const first = result.current;
    rerender();
    const second = result.current;

    // useMemo should keep the same reference since deps haven't changed
    expect(first.login).toBe(second.login);
    expect(first.logout).toBe(second.logout);
    expect(first.refreshTokens).toBe(second.refreshTokens);
  });

  // ── Login propagation error ───────────────────────────────────────────

  it("login propagates RPC errors to the caller", async () => {
    mockGoogleLogin.mockRejectedValueOnce(new Error("network down"));

    const { result } = renderHook(() => useAuth(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await expect(
      act(async () => {
        await result.current.login("bad-code");
      }),
    ).rejects.toThrow("network down");

    // State should remain clean — no partial writes
    expect(result.current.accessToken).toBeNull();
    expect(localStorage.getItem("toqui_access_token")).toBeNull();
  });
});
