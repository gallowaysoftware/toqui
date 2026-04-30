import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";

// useTrialStatus translates the backend's /api/checkout/status response
// into the UI-facing trial state. A bug here either hides the trial
// banner from a user mid-trial (revenue leak: they don't see the
// "expires soon" upgrade prompt) or shows it indefinitely past
// expiry (UX confusion). Each test pins one failure mode.

vi.mock("@/lib/config", () => ({
  getConfig: () => ({ apiUrl: "http://api.test" }),
}));

const mockAuth = { accessToken: "test-token" as string | null };
vi.mock("@/lib/auth", () => ({
  useAuth: () => mockAuth,
}));

import { useTrialStatus } from "../useTrialStatus";

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const wrapper = ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper };
}

function mockJsonResponse(data: unknown, ok = true) {
  return {
    ok,
    status: ok ? 200 : 500,
    json: () => Promise.resolve(data),
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockAuth.accessToken = "test-token";
  globalThis.fetch = vi.fn();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useTrialStatus", () => {
  it("returns NO_TRIAL when token is missing", async () => {
    mockAuth.accessToken = null;
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useTrialStatus("trip-1"), { wrapper });

    // Query disabled when no token; should never fetch + return NO_TRIAL.
    expect(result.current.isTrialActive).toBe(false);
    expect(result.current.isTrialExpired).toBe(false);
    expect(result.current.daysRemaining).toBeNull();
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it("treats fetch failure as NO_TRIAL (graceful degradation)", async () => {
    // The hook silently treats non-OK responses as "no trial" rather
    // than surfacing an error — better UX than blocking the trip
    // detail screen on a transient checkout-status outage. Pin this
    // contract so a future "throw on error" change is deliberate.
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      mockJsonResponse({}, false),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useTrialStatus("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isTrialActive).toBe(false);
    expect(result.current.isTrialExpired).toBe(false);
  });

  it("returns NO_TRIAL when backend says trial_active=false and no end date", async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      mockJsonResponse({ unlocked: false, trial_active: false }),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useTrialStatus("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isTrialActive).toBe(false);
    expect(result.current.daysRemaining).toBeNull();
  });

  it("returns isTrialActive=true with no daysRemaining when trial_end is missing but trial_active=true", async () => {
    // Defensive shape: backend sends trial_active=true without trial_end.
    // Should still mark the trial as active so the UI doesn't pretend
    // the user is on free tier mid-trial.
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      mockJsonResponse({ unlocked: false, trial_active: true }),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useTrialStatus("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isTrialActive).toBe(true);
    expect(result.current.isTrialExpired).toBe(false);
    expect(result.current.daysRemaining).toBeNull();
  });

  it("computes daysRemaining when trial_end is in the future", async () => {
    // 2.5 days into the future: floor(2.5) = 2 days remaining.
    const trialEnd = new Date(Date.now() + 2.5 * 24 * 60 * 60 * 1000);
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      mockJsonResponse({
        unlocked: false,
        trial_active: true,
        trial_end: trialEnd.toISOString(),
      }),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useTrialStatus("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isTrialActive).toBe(true);
    expect(result.current.isTrialExpired).toBe(false);
    expect(result.current.daysRemaining).toBe(2);
    expect(result.current.isLastDay).toBe(false); // > 1 day
  });

  it("flags isLastDay when daysRemaining < 1 day (countdown UX)", async () => {
    // 12 hours remaining: floor(0.5) = 0 days, isLastDay=true.
    // The trip detail UI uses isLastDay to flip from "X days left"
    // to a more urgent "ends today" treatment.
    const trialEnd = new Date(Date.now() + 12 * 60 * 60 * 1000);
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      mockJsonResponse({
        unlocked: false,
        trial_active: true,
        trial_end: trialEnd.toISOString(),
      }),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useTrialStatus("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isTrialActive).toBe(true);
    expect(result.current.daysRemaining).toBe(0);
    expect(result.current.isLastDay).toBe(true);
  });

  it("flags isTrialExpired when trial_end is in the past", async () => {
    // 1 hour ago — expired. UI flips from "trial active" to "purchase
    // to continue" prompt.
    const trialEnd = new Date(Date.now() - 60 * 60 * 1000);
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      mockJsonResponse({
        unlocked: false,
        trial_active: true,
        trial_end: trialEnd.toISOString(),
      }),
    );
    const { wrapper } = makeWrapper();
    const { result } = renderHook(() => useTrialStatus("trip-1"), { wrapper });

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isTrialExpired).toBe(true);
    expect(result.current.isTrialActive).toBe(false);
    expect(result.current.daysRemaining).toBe(0);
  });

  it("calls the correct API endpoint with the trip_id query param + Bearer auth", async () => {
    (globalThis.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce(
      mockJsonResponse({ unlocked: false }),
    );
    const { wrapper } = makeWrapper();
    renderHook(() => useTrialStatus("trip-with-special?chars"), { wrapper });

    await waitFor(() => {
      expect(globalThis.fetch).toHaveBeenCalledWith(
        "http://api.test/api/checkout/status?trip_id=trip-with-special%3Fchars",
        { headers: { Authorization: "Bearer test-token" } },
      );
    });
  });
});
