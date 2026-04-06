import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

// ---------- mocks ----------

const mockAuth = { accessToken: "test-token" };
vi.mock("@/lib/auth", () => ({
  useAuth: () => mockAuth,
}));

vi.mock("@/lib/config", () => ({
  getConfig: () => ({ apiUrl: "https://api.test" }),
}));

// Mock authFetch — we test the hook logic, not the fetch wrapper
const mockAuthFetch = vi.fn();
vi.mock("@/lib/authFetch", () => ({
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

import { useCheckout } from "../useCheckout";

// ---------- helpers ----------

function jsonResponse(body: unknown, status = 200): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
  } as Response;
}

// ---------- tests ----------

describe("useCheckout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ---- initCheckout ----

  describe("initCheckout", () => {
    it("POSTs to /api/checkout with trip_id and returns Stripe checkout URL", async () => {
      const body = { url: "https://checkout.stripe.com/c/pay/cs_test_123" };
      mockAuthFetch.mockResolvedValue(jsonResponse(body));

      const { result } = renderHook(() => useCheckout("trip-1"));

      let response: unknown;
      await act(async () => {
        response = await result.current.initCheckout();
      });

      expect(response).toEqual(body);
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "https://api.test/api/checkout",
        "test-token",
        {
          method: "POST",
          body: JSON.stringify({ trip_id: "trip-1" }),
        },
      );
      expect(result.current.isLoading).toBe(false);
      expect(result.current.error).toBeNull();
    });

    it("sets error state on non-OK response", async () => {
      mockAuthFetch.mockResolvedValue(jsonResponse({}, 500));

      const { result } = renderHook(() => useCheckout("trip-1"));

      await act(async () => {
        await expect(result.current.initCheckout()).rejects.toThrow(
          "Checkout init failed: 500",
        );
      });

      expect(result.current.error).toBe("Checkout init failed: 500");
      expect(result.current.isLoading).toBe(false);
    });

    it("sets error state on network failure", async () => {
      mockAuthFetch.mockRejectedValue(new TypeError("Network request failed"));

      const { result } = renderHook(() => useCheckout("trip-1"));

      await act(async () => {
        await expect(result.current.initCheckout()).rejects.toThrow(
          "Network request failed",
        );
      });

      expect(result.current.error).toBe("Network request failed");
    });

    it("sets generic error for non-Error throws", async () => {
      mockAuthFetch.mockRejectedValue("something weird");

      const { result } = renderHook(() => useCheckout("trip-1"));

      await act(async () => {
        await expect(result.current.initCheckout()).rejects.toBe(
          "something weird",
        );
      });

      expect(result.current.error).toBe("Checkout failed");
    });

    it("clears previous error on new call", async () => {
      // First call fails
      mockAuthFetch.mockResolvedValueOnce(jsonResponse({}, 500));
      const { result } = renderHook(() => useCheckout("trip-1"));

      await act(async () => {
        await result.current.initCheckout().catch(() => {});
      });
      expect(result.current.error).toBe("Checkout init failed: 500");

      // Second call succeeds — error should be cleared
      mockAuthFetch.mockResolvedValueOnce(
        jsonResponse({ url: "https://checkout.stripe.com/c/pay/cs_test_456" }),
      );
      await act(async () => {
        await result.current.initCheckout();
      });
      expect(result.current.error).toBeNull();
    });
  });

  // ---- checkStatus ----

  describe("checkStatus", () => {
    it("GETs /api/checkout/status with trip_id query param", async () => {
      const body = { unlocked: true, priceCents: 1200, currency: "USD" };
      mockAuthFetch.mockResolvedValue(jsonResponse(body));

      const { result } = renderHook(() => useCheckout("trip-1"));

      let response: unknown;
      await act(async () => {
        response = await result.current.checkStatus();
      });

      expect(response).toEqual(body);
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "https://api.test/api/checkout/status?trip_id=trip-1",
        "test-token",
      );
    });

    it("URL-encodes the trip_id parameter", async () => {
      mockAuthFetch.mockResolvedValue(
        jsonResponse({ unlocked: false, priceCents: 1200, currency: "USD" }),
      );

      const { result } = renderHook(() => useCheckout("trip with spaces & special=chars"));

      await act(async () => {
        await result.current.checkStatus();
      });

      expect(mockAuthFetch).toHaveBeenCalledWith(
        "https://api.test/api/checkout/status?trip_id=trip%20with%20spaces%20%26%20special%3Dchars",
        "test-token",
      );
    });

    it("sets error on status check failure", async () => {
      mockAuthFetch.mockResolvedValue(jsonResponse({}, 503));

      const { result } = renderHook(() => useCheckout("trip-1"));

      await act(async () => {
        await expect(result.current.checkStatus()).rejects.toThrow(
          "Status check failed: 503",
        );
      });

      expect(result.current.error).toBe("Status check failed: 503");
    });
  });

  // ---- loading state ----

  describe("loading state", () => {
    it("is true during an in-flight request", async () => {
      let resolve: (value: Response) => void;
      mockAuthFetch.mockReturnValue(
        new Promise<Response>((r) => {
          resolve = r;
        }),
      );

      const { result } = renderHook(() => useCheckout("trip-1"));
      expect(result.current.isLoading).toBe(false);

      let promise: Promise<unknown>;
      act(() => {
        promise = result.current.initCheckout();
      });

      // Loading should be true while request is in-flight
      expect(result.current.isLoading).toBe(true);

      // Resolve the request
      await act(async () => {
        resolve!(jsonResponse({ url: "https://checkout.stripe.com/c/pay/cs_test_789" }));
        await promise;
      });

      expect(result.current.isLoading).toBe(false);
    });
  });
});
