import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";

// useSubscription manages the Explorer/Voyager subscription lifecycle
// (load current state, start checkout, cancel, open billing portal).
// A bug here either:
//   - leaks a wrong subscription tier into the UI (false-grants
//     premium features → revenue leak)
//   - drops cancellation state (user thinks they cancelled, gets
//     billed again → support ticket + refund)
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

const mockOpenURL = vi.fn();
vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Linking: { openURL: (url: string) => mockOpenURL(url) },
  };
});

import { useSubscription } from "../useSubscription";

function jsonOk(data: unknown) {
  return { ok: true, status: 200, json: () => Promise.resolve(data) };
}

function jsonFail(status = 500, body: unknown = {}) {
  return { ok: false, status, json: () => Promise.resolve(body) };
}

beforeEach(() => {
  vi.clearAllMocks();
  mockAuth.accessToken = "test-token";
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useSubscription — load", () => {
  it("does not fetch when no token; flips isLoading false immediately", async () => {
    mockAuth.accessToken = null;
    const { result } = renderHook(() => useSubscription());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.subscription).toBeNull();
    expect(mockAuthFetch).not.toHaveBeenCalled();
  });

  it("loads subscription with proper field translation", async () => {
    // The backend uses snake_case (current_period_end), the UI uses
    // camelCase. Pin the rename so a future field-name regression is
    // caught (would silently break the renew/cancel UX).
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({
        tier: "explorer",
        status: "active",
        billing_period: "annual",
        current_period_end: "2027-04-30T00:00:00Z",
        cancel_at_period_end: false,
      }),
    );
    const { result } = renderHook(() => useSubscription());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.subscription).toMatchObject({
      tier: "explorer",
      status: "active",
      billingPeriod: "annual",
      cancelAtPeriodEnd: false,
    });
    expect(result.current.subscription?.currentPeriodEnd).toBeInstanceOf(Date);
  });

  it("handles missing current_period_end as null Date (free / inactive paths)", async () => {
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({
        tier: "free",
        status: "inactive",
        billing_period: null,
        current_period_end: null,
        cancel_at_period_end: false,
      }),
    );
    const { result } = renderHook(() => useSubscription());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.subscription?.currentPeriodEnd).toBeNull();
    expect(result.current.subscription?.billingPeriod).toBeNull();
  });

  it("captures error on non-OK fetch", async () => {
    mockAuthFetch.mockResolvedValueOnce(jsonFail(500));
    const { result } = renderHook(() => useSubscription());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.error).toContain("500");
    expect(result.current.subscription).toBeNull();
  });

  it("captures network error message", async () => {
    mockAuthFetch.mockRejectedValueOnce(new Error("offline"));
    const { result } = renderHook(() => useSubscription());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.error).toBe("offline");
  });
});

describe("useSubscription.subscribe", () => {
  it("posts the right tier + billing_period and opens the checkout URL", async () => {
    // Mount with a successful initial load.
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({
        tier: "free",
        status: "inactive",
        billing_period: null,
        current_period_end: null,
        cancel_at_period_end: false,
      }),
    );
    const { result } = renderHook(() => useSubscription());
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({ url: "https://checkout.stripe.com/abc" }),
    );

    await act(async () => {
      await result.current.subscribe("explorer", true);
    });

    // Pin the request shape — tier=explorer, billing_period=annual.
    const checkoutCall = mockAuthFetch.mock.calls[1];
    expect(checkoutCall[0]).toBe("http://api.test/api/subscription/checkout");
    const opts = checkoutCall[2];
    expect(opts.method).toBe("POST");
    const body = JSON.parse(opts.body);
    expect(body).toEqual({ tier: "explorer", billing_period: "annual" });

    expect(mockOpenURL).toHaveBeenCalledWith("https://checkout.stripe.com/abc");
  });

  it("translates annual=false → billing_period 'monthly'", async () => {
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({
        tier: "free",
        status: "inactive",
        billing_period: null,
        current_period_end: null,
        cancel_at_period_end: false,
      }),
    );
    const { result } = renderHook(() => useSubscription());
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    mockAuthFetch.mockResolvedValueOnce(jsonOk({ url: "x" }));

    await act(async () => {
      await result.current.subscribe("voyager", false);
    });

    const body = JSON.parse(mockAuthFetch.mock.calls[1][2].body);
    expect(body.billing_period).toBe("monthly");
    expect(body.tier).toBe("voyager");
  });

  it("re-throws error on checkout failure (so caller can react)", async () => {
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({
        tier: "free",
        status: "inactive",
        billing_period: null,
        current_period_end: null,
        cancel_at_period_end: false,
      }),
    );
    const { result } = renderHook(() => useSubscription());
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    mockAuthFetch.mockResolvedValueOnce(jsonFail(403));

    let threw: unknown = null;
    await act(async () => {
      try {
        await result.current.subscribe("explorer", true);
      } catch (e) {
        threw = e;
      }
    });

    expect(threw).toBeInstanceOf(Error);
    expect(result.current.error).toContain("403");
    expect(mockOpenURL).not.toHaveBeenCalled();
  });
});

describe("useSubscription.cancel", () => {
  it("flips cancelAtPeriodEnd to true on success without losing other fields", async () => {
    // Mount with active subscription.
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({
        tier: "explorer",
        status: "active",
        billing_period: "monthly",
        current_period_end: "2027-04-30T00:00:00Z",
        cancel_at_period_end: false,
      }),
    );
    const { result } = renderHook(() => useSubscription());
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    mockAuthFetch.mockResolvedValueOnce({ ok: true, status: 204 });

    await act(async () => {
      await result.current.cancel();
    });

    // Pin: cancel only flips the cancellation flag; other fields
    // remain (so the UI keeps showing access until period_end).
    expect(result.current.subscription).toMatchObject({
      tier: "explorer",
      status: "active",
      billingPeriod: "monthly",
      cancelAtPeriodEnd: true,
    });
  });

  it("re-throws and surfaces error on failure", async () => {
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({
        tier: "explorer",
        status: "active",
        billing_period: "monthly",
        current_period_end: "2027-04-30T00:00:00Z",
        cancel_at_period_end: false,
      }),
    );
    const { result } = renderHook(() => useSubscription());
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    mockAuthFetch.mockResolvedValueOnce(jsonFail(500));

    let threw: unknown = null;
    await act(async () => {
      try {
        await result.current.cancel();
      } catch (e) {
        threw = e;
      }
    });

    expect(threw).toBeInstanceOf(Error);
    expect(result.current.error).toContain("500");
    // Important: failed cancel must NOT have flipped local state
    // (would mislead the UI into showing "cancelled" when it isn't).
    expect(result.current.subscription?.cancelAtPeriodEnd).toBe(false);
  });
});

describe("useSubscription.manageSubscription", () => {
  it("opens the Stripe billing portal URL", async () => {
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({
        tier: "voyager",
        status: "active",
        billing_period: "annual",
        current_period_end: "2028-01-01T00:00:00Z",
        cancel_at_period_end: false,
      }),
    );
    const { result } = renderHook(() => useSubscription());
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({ url: "https://billing.stripe.com/portal" }),
    );

    await act(async () => {
      await result.current.manageSubscription();
    });

    expect(mockAuthFetch).toHaveBeenLastCalledWith(
      "http://api.test/api/subscription/portal",
      "test-token",
      expect.objectContaining({ method: "POST" }),
    );
    expect(mockOpenURL).toHaveBeenCalledWith("https://billing.stripe.com/portal");
  });

  it("re-throws on portal failure", async () => {
    mockAuthFetch.mockResolvedValueOnce(
      jsonOk({
        tier: "voyager",
        status: "active",
        billing_period: "annual",
        current_period_end: null,
        cancel_at_period_end: false,
      }),
    );
    const { result } = renderHook(() => useSubscription());
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    mockAuthFetch.mockResolvedValueOnce(jsonFail(500));

    let threw: unknown = null;
    await act(async () => {
      try {
        await result.current.manageSubscription();
      } catch (e) {
        threw = e;
      }
    });

    expect(threw).toBeInstanceOf(Error);
    expect(mockOpenURL).not.toHaveBeenCalled();
  });
});
