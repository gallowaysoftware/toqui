import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockAuthFetch = vi.fn();

vi.mock("@/lib/authFetch", () => ({
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

vi.mock("@/lib/config", () => ({
  getConfig: () => ({ apiUrl: "http://localhost:8090" }),
}));

const mockAuth = { accessToken: "test-token" as string | null };
vi.mock("@/lib/auth", () => ({
  useAuth: () => mockAuth,
}));

import { useReferral } from "../useReferral";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function mockJsonResponse(data: unknown, ok = true) {
  return {
    ok,
    status: ok ? 200 : 500,
    json: () => Promise.resolve(data),
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useReferral", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.accessToken = "test-token";
  });

  it("fetches referral data when authenticated", async () => {
    const data = {
      code: "ABC123",
      link: "https://toqui.travel?ref=ABC123",
      successful_referrals: 3,
      rewards_earned: 1,
    };
    mockAuthFetch.mockResolvedValue(mockJsonResponse(data));

    const { result } = renderHook(() => useReferral());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.code).toBe("ABC123");
    expect(result.current.successfulReferrals).toBe(3);
    expect(result.current.rewardsEarned).toBe(1);
    expect(result.current.error).toBeNull();
  });

  it("calls the correct API endpoint", async () => {
    mockAuthFetch.mockResolvedValue(
      mockJsonResponse({ code: "X", link: "", successful_referrals: 0, rewards_earned: 0 }),
    );

    renderHook(() => useReferral());

    await waitFor(() => {
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "http://localhost:8090/api/referral",
        "test-token",
      );
    });
  });

  it("does NOT fetch when accessToken is null", async () => {
    mockAuth.accessToken = null;

    const { result } = renderHook(() => useReferral());

    // Wait a tick to ensure no async fetch was triggered
    await new Promise((r) => setTimeout(r, 50));
    expect(mockAuthFetch).not.toHaveBeenCalled();
    expect(result.current.isLoading).toBe(false);
    expect(result.current.code).toBeNull();
  });

  it("sets error when API returns non-ok response", async () => {
    mockAuthFetch.mockResolvedValue(mockJsonResponse(null, false));

    const { result } = renderHook(() => useReferral());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.error).toContain("Failed to fetch referral data");
    expect(result.current.code).toBeNull();
  });

  it("sets error when API fetch throws", async () => {
    mockAuthFetch.mockRejectedValue(new Error("Network error"));

    const { result } = renderHook(() => useReferral());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.error).toBe("Network error");
    expect(result.current.code).toBeNull();
  });

  it("returns defaults when data has not loaded", () => {
    mockAuth.accessToken = null;
    const { result } = renderHook(() => useReferral());
    expect(result.current.code).toBeNull();
    expect(result.current.link).toBeNull();
    expect(result.current.successfulReferrals).toBe(0);
    expect(result.current.rewardsEarned).toBe(0);
  });

  it("provides a redeemCode function that posts to the correct endpoint", async () => {
    mockAuthFetch
      .mockResolvedValueOnce(
        mockJsonResponse({ code: "ABC", link: "", successful_referrals: 0, rewards_earned: 0 }),
      )
      .mockResolvedValueOnce(mockJsonResponse({}, true));

    const { result } = renderHook(() => useReferral());

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.redeemCode("FRIEND99");
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "http://localhost:8090/api/referral/redeem",
      "test-token",
      {
        method: "POST",
        body: JSON.stringify({ code: "FRIEND99" }),
      },
    );
  });

  it("redeemCode throws when API returns non-ok response", async () => {
    mockAuthFetch
      .mockResolvedValueOnce(
        mockJsonResponse({ code: "ABC", link: "", successful_referrals: 0, rewards_earned: 0 }),
      )
      .mockResolvedValueOnce(mockJsonResponse(null, false));

    const { result } = renderHook(() => useReferral());

    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await expect(
      act(async () => {
        await result.current.redeemCode("BADCODE");
      }),
    ).rejects.toThrow("Failed to redeem code");
  });
});
