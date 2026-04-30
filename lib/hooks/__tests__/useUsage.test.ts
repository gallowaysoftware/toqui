import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";

// useUsage powers the daily-message-limit indicator throughout the app.
// A bug here either:
//   - hides the "you're at the limit" warning (user surprised when chat
//     RPC returns ResourceExhausted), or
//   - shows isAtLimit when they're not (false negative gates the AI)
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

import { useUsage, formatTimeUntilReset } from "../useUsage";

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper };
}

function jsonOk(data: unknown) {
  return { ok: true, status: 200, json: () => Promise.resolve(data) };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockAuth.accessToken = "test-token";
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useUsage", () => {
  it("returns DEFAULT (not loading) when no token", () => {
    mockAuth.accessToken = null;
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useUsage(), { wrapper });

    // Disabled query → never fetches; returns DEFAULT.
    expect(result.current.isLoading).toBe(false);
    expect(result.current.used).toBe(0);
    expect(result.current.limit).toBe(0);
    expect(result.current.tier).toBe("free");
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("returns DEFAULT when fetch fails", async () => {
    // The hook surfaces the error via useQuery's error state but our
    // shape returns DEFAULT to avoid blocking the chat UI on a
    // transient /api/usage outage. Pin this graceful-degradation
    // contract.
    mockAuthFetch.mockResolvedValueOnce({ ok: false, status: 500 });
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useUsage(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.used).toBe(0);
    expect(result.current.limit).toBe(0);
    expect(result.current.tier).toBe("free");
  });

  it("parses usage data and computes remaining/threshold flags", async () => {
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({
        used: 7,
        limit: 10,
        resets_at: "2026-04-30T00:00:00Z",
        tier: "free",
      }),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useUsage(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.used).toBe(7);
    expect(result.current.limit).toBe(10);
    expect(result.current.remainingMessages).toBe(3);
    expect(result.current.tier).toBe("free");
    // 7/10 = 0.7, NOT > 0.8.
    expect(result.current.isNearLimit).toBe(false);
    expect(result.current.isAtLimit).toBe(false);
    expect(result.current.resetsAt).toBeInstanceOf(Date);
  });

  it("flags isNearLimit when used > 80% of limit", async () => {
    // Pin the strict > 0.8 boundary. A future flip to >= 0.8 would
    // trigger the "near limit" UX warning at exactly 80% which is
    // arguably reasonable but a deliberate decision.
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({ used: 9, limit: 10, resets_at: "" }),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useUsage(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isNearLimit).toBe(true);
    expect(result.current.isAtLimit).toBe(false);
  });

  it("flags isAtLimit at exactly limit (>=, not >)", async () => {
    // 10/10 used: pin that >= triggers — a flip to > would let the
    // user fire one extra request before the UI gates.
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({ used: 10, limit: 10, resets_at: "" }),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useUsage(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isAtLimit).toBe(true);
    expect(result.current.remainingMessages).toBe(0);
  });

  it("clamps remainingMessages to >= 0 when used > limit", async () => {
    // Defensive: if used > limit (race between client + server), the
    // UI shouldn't show negative remaining.
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({ used: 15, limit: 10, resets_at: "" }),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useUsage(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.remainingMessages).toBe(0);
    expect(result.current.isAtLimit).toBe(true);
  });

  it("treats limit=0 as 'no limits' — no near/at flags fire", async () => {
    // Unlimited tiers (Explorer/Voyager) come back with limit=0.
    // Pin that the threshold checks gate on `limit > 0` so we don't
    // divide by zero AND don't show "at limit" for unlimited users.
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({ used: 500, limit: 0, resets_at: "", tier: "explorer" }),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useUsage(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.tier).toBe("explorer");
    expect(result.current.isNearLimit).toBe(false);
    expect(result.current.isAtLimit).toBe(false);
  });

  it("parses each tier value: pro / explorer / voyager / unknown→free", async () => {
    // Pin the parseTier allowlist — anything not in the known set
    // falls back to free. This is the right default (don't grant Pro
    // features for an unknown tier) and pins a security invariant.
    const tiers: Array<[string | undefined, string]> = [
      ["pro", "pro"],
      ["explorer", "explorer"],
      ["voyager", "voyager"],
      ["free", "free"],
      ["enterprise", "free"], // unknown → free
      [undefined, "free"], // missing → free
      ["", "free"], // empty → free
    ];
    for (const [input, want] of tiers) {
      mockAuthFetch.mockResolvedValueOnce(
        jsonOk({ used: 1, limit: 10, resets_at: "", tier: input }),
      );
      const { wrapper } = makeWrapper();
      const { result } = renderHook(() => useUsage(), { wrapper });
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.tier, `input=${JSON.stringify(input)}`).toBe(want);
    }
  });

  it("returns null resetsAt when API sends empty string", async () => {
    // Backend sends empty string when resets_at is unset (no daily
    // limit applies, e.g. unlimited tier). resetsAt should be null,
    // not a `new Date("")` (which is Invalid Date).
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({ used: 0, limit: 0, resets_at: "" }),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useUsage(), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.resetsAt).toBeNull();
  });

  it("calls /api/usage with the access token", async () => {
    mockAuthFetch.mockResolvedValueOnce(jsonOk({ used: 0, limit: 10, resets_at: "" }));
    const { wrapper } = makeWrapper();
    renderHook(() => useUsage(), { wrapper });

    await waitFor(() => {
      expect(mockAuthFetch).toHaveBeenCalledWith("http://api.test/api/usage", "test-token");
    });
  });
});

describe("formatTimeUntilReset", () => {
  // formatTimeUntilReset is the human-friendly countdown shown in the
  // usage indicator. Pin the boundary cases since the rounding behaviour
  // (ceil) is what makes "in 1m" appear instead of "in 0m" near zero.

  it("returns empty string for null input", () => {
    expect(formatTimeUntilReset(null)).toBe("");
  });

  it("returns 'soon' when reset is in the past", () => {
    expect(formatTimeUntilReset(new Date(Date.now() - 1000))).toBe("soon");
  });

  it("formats sub-hour as 'in Nm'", () => {
    expect(formatTimeUntilReset(new Date(Date.now() + 30 * 60_000))).toBe("in 30m");
  });

  it("formats whole hours as 'in Nh'", () => {
    expect(formatTimeUntilReset(new Date(Date.now() + 2 * 60 * 60_000))).toBe("in 2h");
  });

  it("formats hours + minutes as 'in Nh Mm'", () => {
    expect(formatTimeUntilReset(new Date(Date.now() + (3 * 60 + 22) * 60_000))).toBe("in 3h 22m");
  });

  it("uses ceil so near-zero returns 'in 1m' not 'in 0m'", () => {
    // 30 seconds remaining: ceil(0.5 min) = 1 min.
    expect(formatTimeUntilReset(new Date(Date.now() + 30_000))).toBe("in 1m");
  });
});
