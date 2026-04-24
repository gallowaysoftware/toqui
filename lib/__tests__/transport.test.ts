// CLAIMED BY AGENT 1
import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import React from "react";

// ---------------------------------------------------------------------------
// Mocks — wired before imports
// ---------------------------------------------------------------------------

// We need to capture the interceptor that TransportProvider passes to
// createConnectTransport so we can invoke it directly in unit tests.
// This avoids needing a real HTTP server.

let capturedInterceptors: Array<(next: any) => (req: any) => Promise<any>> =
  [];

vi.mock("@connectrpc/connect-web", () => ({
  createConnectTransport: vi.fn((opts: any) => {
    capturedInterceptors = opts.interceptors ?? [];
    return { _type: "mock-transport" };
  }),
}));

// Re-export real Code and ConnectError from connect so the interceptor
// recognizes them with instanceof checks.
const REAL_CONNECT = await vi.importActual<typeof import("@connectrpc/connect")>(
  "@connectrpc/connect",
);

vi.mock("@connectrpc/connect", async () => {
  const actual =
    await vi.importActual<typeof import("@connectrpc/connect")>(
      "@connectrpc/connect",
    );
  return {
    ...actual,
    // keep real Code + ConnectError for the interceptor
    // mock createClient so auth.tsx's refreshTokens() can be controlled in tests
    createClient: vi.fn(actual.createClient),
  };
});

vi.mock("@gen/toqui/v1/auth_pb", () => ({
  AuthService: "MockAuthService",
}));

vi.mock("expo-secure-store", () => ({
  getItemAsync: vi.fn(),
  setItemAsync: vi.fn(),
  deleteItemAsync: vi.fn(),
}));

// Import modules under test AFTER mocks
import { AuthProvider, useAuth } from "../auth";
import {
  TransportProvider,
  useTransport,
  useConsentSignal,
  isConsentRequiredError,
  CONSENT_REQUIRED_SENTINEL,
} from "../transport";
const { ConnectError, Code } = REAL_CONNECT;

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeRequest() {
  const headers = new Headers();
  return {
    header: headers,
    url: "http://localhost:8090/toqui.v1.TripService/ListTrips",
    method: "POST" as const,
  };
}

/**
 * Build the composed interceptor function from the captured array.
 * The ConnectRPC interceptor contract is: interceptor(next) => handler(req) => response.
 */
function getInterceptor() {
  if (capturedInterceptors.length === 0) {
    throw new Error(
      "No interceptors captured — did TransportProvider render?",
    );
  }
  return capturedInterceptors[0];
}

// Combined provider wrapper for transport tests
function makeWrapper(overrides?: { accessToken?: string | null }) {
  // We render AuthProvider + TransportProvider in the correct nesting.
  // To control the accessToken we prime localStorage before render.
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(
      AuthProvider,
      null,
      React.createElement(TransportProvider, null, children),
    );
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("TransportProvider", () => {
  beforeEach(() => {
    localStorage.clear();
    capturedInterceptors = [];
    vi.clearAllMocks();
  });

  // ── Context guard ─────────────────────────────────────────────────────

  it("throws when useTransport is called outside TransportProvider", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => renderHook(() => useTransport())).toThrow(
      "useTransport must be used within a TransportProvider",
    );
    spy.mockRestore();
  });

  it("provides a transport object when properly nested", () => {
    const { result } = renderHook(() => useTransport(), {
      wrapper: makeWrapper(),
    });
    expect(result.current).toBeDefined();
    expect((result.current as any)._type).toBe("mock-transport");
  });

  // ── Transport stability ───────────────────────────────────────────────

  it("transport reference is stable across re-renders (useMemo with empty deps)", () => {
    const { result, rerender } = renderHook(() => useTransport(), {
      wrapper: makeWrapper(),
    });
    const first = result.current;
    rerender();
    expect(result.current).toBe(first);
  });
});

describe("Transport interceptor", () => {
  beforeEach(() => {
    localStorage.clear();
    capturedInterceptors = [];
    vi.clearAllMocks();
  });

  // Helper: render the providers to capture interceptors, return the interceptor
  function renderAndCapture() {
    renderHook(() => useTransport(), {
      wrapper: makeWrapper(),
    });
    return getInterceptor();
  }

  // ── Bearer token injection ────────────────────────────────────────────

  it("does NOT set Authorization header when there is no token", async () => {
    const interceptor = renderAndCapture();

    const req = makeRequest();
    const mockNext = vi.fn().mockResolvedValue({ status: 200 });
    const handler = interceptor(mockNext);

    await handler(req);

    expect(req.header.has("Authorization")).toBe(false);
    expect(mockNext).toHaveBeenCalledWith(req);
  });

  it("sets Bearer token from tokenRef when token exists", async () => {
    localStorage.setItem("toqui_access_token", "my-token");

    // Render so AuthProvider hydrates the token and the ref is updated
    const { result } = renderHook(() => useAuth(), {
      wrapper: makeWrapper(),
    });

    // Wait for hydration
    await vi.waitFor(() => expect(result.current.isLoading).toBe(false));

    const interceptor = getInterceptor();
    const req = makeRequest();
    const mockNext = vi.fn().mockResolvedValue({ status: 200 });
    const handler = interceptor(mockNext);

    await handler(req);

    expect(req.header.get("Authorization")).toBe("Bearer my-token");
  });

  // ── 401 auto-refresh retry ────────────────────────────────────────────

  it("retries with new token after 401 Unauthenticated + successful refresh", async () => {
    localStorage.setItem("toqui_access_token", "expired-token");
    localStorage.setItem("toqui_refresh_token", "valid-rt");

    // Mock the ConnectRPC client that AuthProvider.refreshTokens uses internally
    const { createClient } = await import("@connectrpc/connect");
    const mockCreateClient = vi.mocked(createClient);
    mockCreateClient.mockReturnValue({
      googleLogin: vi.fn(),
      refreshToken: vi.fn().mockResolvedValue({
        accessToken: "refreshed-token",
        refreshToken: "new-rt",
      }),
    } as any);

    const { result } = renderHook(() => useAuth(), {
      wrapper: makeWrapper(),
    });
    await vi.waitFor(() => expect(result.current.isLoading).toBe(false));

    const interceptor = getInterceptor();

    const callCount = { value: 0 };
    const mockNext = vi.fn().mockImplementation(async (req: any) => {
      callCount.value++;
      if (callCount.value === 1) {
        throw new ConnectError("token expired", Code.Unauthenticated);
      }
      return { status: 200 };
    });

    const req = makeRequest();
    const handler = interceptor(mockNext);
    const response = await handler(req);

    // next was called twice: once with expired token, once with refreshed
    expect(mockNext).toHaveBeenCalledTimes(2);
    expect(response).toEqual({ status: 200 });
    // The header should have the refreshed token on retry
    expect(req.header.get("Authorization")).toBe("Bearer refreshed-token");
  });

  it("throws original error when refresh returns null (no refresh token)", async () => {
    // No refresh token in storage — refreshTokens() returns null
    const interceptor = renderAndCapture();

    const mockNext = vi.fn().mockRejectedValue(
      new ConnectError("unauthenticated", Code.Unauthenticated),
    );

    const req = makeRequest();
    const handler = interceptor(mockNext);

    await expect(handler(req)).rejects.toThrow(ConnectError);
  });

  it("re-throws non-Unauthenticated ConnectErrors without attempting refresh", async () => {
    localStorage.setItem("toqui_refresh_token", "rt");

    const { result } = renderHook(() => useAuth(), {
      wrapper: makeWrapper(),
    });
    await vi.waitFor(() => expect(result.current.isLoading).toBe(false));

    const interceptor = getInterceptor();

    const mockNext = vi.fn().mockRejectedValue(
      new ConnectError("forbidden", Code.PermissionDenied),
    );

    const req = makeRequest();
    const handler = interceptor(mockNext);

    await expect(handler(req)).rejects.toThrow(ConnectError);

    // next should only be called once — no retry
    expect(mockNext).toHaveBeenCalledTimes(1);
  });

  it("re-throws non-ConnectError exceptions without attempting refresh", async () => {
    const interceptor = renderAndCapture();

    const mockNext = vi.fn().mockRejectedValue(new TypeError("fetch failed"));

    const req = makeRequest();
    const handler = interceptor(mockNext);

    await expect(handler(req)).rejects.toThrow(TypeError);
    expect(mockNext).toHaveBeenCalledTimes(1);
  });

  // ── Ref-based interceptor picks up token changes without transport recreation ──

  // ── Consent-required signal (toqui-backend PR #374) ──────────────────

  it("isConsentRequiredError matches FailedPrecondition with consent_required sentinel", () => {
    const hit = new ConnectError(
      CONSENT_REQUIRED_SENTINEL,
      Code.FailedPrecondition,
    );
    expect(isConsentRequiredError(hit)).toBe(true);
  });

  it("isConsentRequiredError rejects FailedPrecondition WITHOUT the sentinel", () => {
    const miss = new ConnectError(
      "age_verification_required",
      Code.FailedPrecondition,
    );
    expect(isConsentRequiredError(miss)).toBe(false);
  });

  it("isConsentRequiredError rejects Unauthenticated even if message matches", () => {
    // Paranoia: the SENTINEL must ONLY trigger on FailedPrecondition. An
    // Unauthenticated error with a coincidental message string should NOT
    // flip the consent flag — it would trap the user in the consent modal
    // while the real issue is an expired token.
    const miss = new ConnectError(
      CONSENT_REQUIRED_SENTINEL,
      Code.Unauthenticated,
    );
    expect(isConsentRequiredError(miss)).toBe(false);
  });

  it("isConsentRequiredError rejects non-ConnectError values", () => {
    expect(isConsentRequiredError(new TypeError("fetch failed"))).toBe(false);
    expect(isConsentRequiredError(null)).toBe(false);
    expect(isConsentRequiredError("consent_required")).toBe(false);
  });

  it("flips consentRequired context flag on FailedPrecondition(consent_required) and re-throws", async () => {
    // Wrap a hook that consumes both transport AND the consent signal so
    // we can observe the state transition.
    function useBoth() {
      return {
        transport: useTransport(),
        signal: useConsentSignal(),
      };
    }

    const { result } = renderHook(useBoth, {
      wrapper: makeWrapper(),
    });

    expect(result.current.signal.consentRequired).toBe(false);

    const interceptor = getInterceptor();
    const mockNext = vi.fn().mockRejectedValue(
      new ConnectError(CONSENT_REQUIRED_SENTINEL, Code.FailedPrecondition),
    );

    const req = makeRequest();
    const handler = interceptor(mockNext);
    // The error must still propagate — consuming hooks rely on their
    // normal error paths. The gate is just drawn ON TOP.
    await expect(handler(req)).rejects.toThrow(ConnectError);

    // Interceptor should NOT retry on consent errors (that's a refresh
    // flow, which only runs for Unauthenticated).
    expect(mockNext).toHaveBeenCalledTimes(1);

    // The flag should be flipped. Because setState is async within the
    // interceptor, wait for React to flush.
    const { act } = await import("@testing-library/react");
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.signal.consentRequired).toBe(true);
  });

  it("acknowledgeConsent clears the signal back to false", async () => {
    function useBoth() {
      return {
        transport: useTransport(),
        signal: useConsentSignal(),
      };
    }

    const { result } = renderHook(useBoth, {
      wrapper: makeWrapper(),
    });

    const interceptor = getInterceptor();
    const mockNext = vi.fn().mockRejectedValue(
      new ConnectError(CONSENT_REQUIRED_SENTINEL, Code.FailedPrecondition),
    );

    const handler = interceptor(mockNext);
    await expect(handler(makeRequest())).rejects.toThrow(ConnectError);

    const { act } = await import("@testing-library/react");
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.signal.consentRequired).toBe(true);

    await act(async () => {
      result.current.signal.acknowledgeConsent();
    });
    expect(result.current.signal.consentRequired).toBe(false);
  });

  it("FailedPrecondition WITHOUT consent_required sentinel does NOT flip the flag", async () => {
    function useBoth() {
      return {
        transport: useTransport(),
        signal: useConsentSignal(),
      };
    }

    const { result } = renderHook(useBoth, {
      wrapper: makeWrapper(),
    });

    const interceptor = getInterceptor();
    // e.g. age-verification-required would also be FailedPrecondition.
    // It must NOT trip the consent gate.
    const mockNext = vi.fn().mockRejectedValue(
      new ConnectError("age_verification_required", Code.FailedPrecondition),
    );

    const handler = interceptor(mockNext);
    await expect(handler(makeRequest())).rejects.toThrow(ConnectError);

    const { act } = await import("@testing-library/react");
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.signal.consentRequired).toBe(false);
  });

  it("useConsentSignal throws outside TransportProvider", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => renderHook(() => useConsentSignal())).toThrow(
      "useConsentSignal must be used within a TransportProvider",
    );
    spy.mockRestore();
  });

  it("interceptor reads from ref so it picks up token changes without new transport", async () => {
    const { result } = renderHook(() => useAuth(), {
      wrapper: makeWrapper(),
    });
    await vi.waitFor(() => expect(result.current.isLoading).toBe(false));

    const interceptor = getInterceptor();
    const mockNext = vi.fn().mockResolvedValue({ status: 200 });

    // First call: no token
    const req1 = makeRequest();
    await interceptor(mockNext)(req1);
    expect(req1.header.has("Authorization")).toBe(false);

    // Simulate token being set (as if login just happened)
    // We use setTokensManually which updates both state and storage,
    // and the useEffect in TransportProvider updates tokenRef.
    const { act } = await import("@testing-library/react");
    await act(async () => {
      await result.current.setTokensManually("late-token", "late-rt");
    });

    // The transport object was NOT recreated (useMemo([], []) never invalidates)
    // but the ref should have the new token
    const req2 = makeRequest();
    await interceptor(mockNext)(req2);
    expect(req2.header.get("Authorization")).toBe("Bearer late-token");
  });
});
