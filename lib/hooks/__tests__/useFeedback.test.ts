import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";

// useFeedback wraps the POST /api/feedback submission and assembles the
// context envelope (platform, app version, screen, theme, optional
// trip_id, web userAgent). A bug here either drops feedback silently
// (loss of bug reports) or leaks data into the context bag we don't
// want sent (e.g. chat content, destination names — privacy concern).
//
// Each test pins one specific contract.

const mockAuthFetch = vi.fn();

vi.mock("@/lib/authFetch", () => ({
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

vi.mock("@/lib/config", () => ({
  getConfig: () => ({ apiUrl: "http://api.test" }),
}));

const mockAuth = { accessToken: "test-token" as string | null };
vi.mock("@/lib/auth", () => ({
  useAuth: () => mockAuth,
}));

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({ mode: "light" }),
}));

vi.mock("expo-constants", () => ({
  default: { expoConfig: { version: "1.2.3" } },
}));

vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Platform: { OS: "web", select: (o: Record<string, unknown>) => o.web ?? o.default },
  };
});

import { useFeedback } from "../useFeedback";

beforeEach(() => {
  vi.clearAllMocks();
  mockAuth.accessToken = "test-token";
  // Stub navigator.userAgent for the web-platform path.
  Object.defineProperty(globalThis, "navigator", {
    configurable: true,
    value: { userAgent: "vitest-stub/1.0" },
  });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useFeedback", () => {
  it("starts in idle state — not submitting, no success/error", () => {
    const { result } = renderHook(() => useFeedback());
    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it("submits with the right URL, method, and Bearer token", async () => {
    mockAuthFetch.mockResolvedValueOnce({ ok: true, status: 200 });
    const { result } = renderHook(() => useFeedback());

    await act(async () => {
      await result.current.submit("bug", "form is broken", "TripDetail");
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "http://api.test/api/feedback",
      "test-token",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("assembles the context envelope with platform, version, screen, theme", async () => {
    mockAuthFetch.mockResolvedValueOnce({ ok: true, status: 200 });
    const { result } = renderHook(() => useFeedback());

    await act(async () => {
      await result.current.submit("bug", "msg", "TripDetail", "trip-uuid-1");
    });

    const [, , opts] = mockAuthFetch.mock.calls[0];
    const payload = JSON.parse(opts.body);
    expect(payload.type).toBe("bug");
    expect(payload.message).toBe("msg");
    // Context must NOT include any chat-content / destination — only
    // metadata. Pin the exact field set so a future addition is a
    // deliberate review event.
    expect(payload.context).toEqual({
      platform: "web",
      appVersion: "1.2.3",
      screen: "TripDetail",
      theme: "light",
      tripId: "trip-uuid-1",
      userAgent: "vitest-stub/1.0",
    });
  });

  it("omits userAgent on native (Platform.OS !== web)", async () => {
    // Override Platform mock to native for this test only.
    vi.doMock("react-native", () => ({
      Platform: { OS: "ios", select: (o: Record<string, unknown>) => o.ios ?? o.default },
    }));
    vi.resetModules();
    const { useFeedback: useFeedbackNative } = await import("../useFeedback");

    mockAuthFetch.mockResolvedValueOnce({ ok: true, status: 200 });
    const { result } = renderHook(() => useFeedbackNative());
    await act(async () => {
      await result.current.submit("bug", "msg", "Settings");
    });

    const [, , opts] = mockAuthFetch.mock.calls[0];
    const payload = JSON.parse(opts.body);
    expect(payload.context.platform).toBe("ios");
    expect(payload.context.userAgent).toBeUndefined();

    vi.doUnmock("react-native");
    vi.resetModules();
  });

  it("flips isSuccess=true on a successful submit", async () => {
    mockAuthFetch.mockResolvedValueOnce({ ok: true, status: 200 });
    const { result } = renderHook(() => useFeedback());

    await act(async () => {
      await result.current.submit("general", "great app!", "Home");
    });

    expect(result.current.isSuccess).toBe(true);
    expect(result.current.error).toBeNull();
    expect(result.current.isSubmitting).toBe(false);
  });

  it("captures error message and clears isSuccess on failure", async () => {
    mockAuthFetch.mockResolvedValueOnce({ ok: false, status: 500 });
    const { result } = renderHook(() => useFeedback());

    await act(async () => {
      await result.current.submit("bug", "msg", "Home");
    });

    expect(result.current.error).toContain("500");
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isSubmitting).toBe(false);
  });

  it("captures network errors without throwing to caller", async () => {
    // Network rejection (DNS, offline) — must NOT bubble up; the hook
    // sets `error` and the caller renders an error state. Pre-fix a
    // throw would crash the FeedbackModal component.
    mockAuthFetch.mockRejectedValueOnce(new Error("offline"));
    const { result } = renderHook(() => useFeedback());

    let threw: unknown = null;
    await act(async () => {
      try {
        await result.current.submit("bug", "msg", "Home");
      } catch (e) {
        threw = e;
      }
    });

    expect(threw).toBeNull();
    expect(result.current.error).toBe("offline");
    expect(result.current.isSuccess).toBe(false);
  });

  it("falls back to 'unknown' when expoConfig version is missing", async () => {
    vi.doMock("expo-constants", () => ({
      default: { expoConfig: undefined },
    }));
    vi.resetModules();
    const { useFeedback: useFeedbackNoVersion } = await import("../useFeedback");

    mockAuthFetch.mockResolvedValueOnce({ ok: true, status: 200 });
    const { result } = renderHook(() => useFeedbackNoVersion());
    await act(async () => {
      await result.current.submit("bug", "msg", "Home");
    });

    const [, , opts] = mockAuthFetch.mock.calls[0];
    const payload = JSON.parse(opts.body);
    expect(payload.context.appVersion).toBe("unknown");

    vi.doUnmock("expo-constants");
    vi.resetModules();
  });

  it("reset() clears isSuccess and error without affecting submit count", async () => {
    mockAuthFetch.mockResolvedValueOnce({ ok: false, status: 500 });
    const { result } = renderHook(() => useFeedback());

    await act(async () => {
      await result.current.submit("bug", "msg", "Home");
    });
    expect(result.current.error).not.toBeNull();

    act(() => {
      result.current.reset();
    });

    expect(result.current.error).toBeNull();
    expect(result.current.isSuccess).toBe(false);
  });

  it("isSubmitting flips to true during the request and false after", async () => {
    // Pin the submitting flag transitions — the FeedbackModal disables
    // its submit button on this flag, so a regression that left it
    // stuck `true` after the request would block subsequent submits.
    let resolveAuth: (value: { ok: boolean; status: number }) => void = () => {};
    const pending = new Promise<{ ok: boolean; status: number }>((res) => {
      resolveAuth = res;
    });
    mockAuthFetch.mockReturnValueOnce(pending);

    const { result } = renderHook(() => useFeedback());

    let submitPromise: Promise<unknown> = Promise.resolve();
    act(() => {
      submitPromise = result.current.submit("bug", "msg", "Home");
    });

    await waitFor(() => expect(result.current.isSubmitting).toBe(true));

    await act(async () => {
      resolveAuth({ ok: true, status: 200 });
      await submitPromise;
    });

    expect(result.current.isSubmitting).toBe(false);
    expect(result.current.isSuccess).toBe(true);
  });
});
