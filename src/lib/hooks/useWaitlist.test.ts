import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement } from "react";
import { useJoinWaitlist, useWaitlistStatus } from "./useWaitlist";

const mockFetch = vi.fn();
global.fetch = mockFetch;

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return createElement(QueryClientProvider, { client: queryClient }, children);
  };
}

describe("useJoinWaitlist", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sends POST request with email to /waitlist", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ position: 5, invite_code: "TOQUI-1234" }),
    });

    const { result } = renderHook(() => useJoinWaitlist(), {
      wrapper: createWrapper(),
    });

    result.current.mutate({ email: "user@example.com" });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/waitlist"),
      expect.objectContaining({
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: "user@example.com" }),
      }),
    );

    expect(result.current.data).toEqual({
      position: 5,
      invite_code: "TOQUI-1234",
    });
  });

  it("throws error when request fails", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => "Server error",
    });

    const { result } = renderHook(() => useJoinWaitlist(), {
      wrapper: createWrapper(),
    });

    result.current.mutate({ email: "user@example.com" });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe("Server error");
  });

  it("provides fallback error message when body is empty", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 400,
      text: async () => "",
    });

    const { result } = renderHook(() => useJoinWaitlist(), {
      wrapper: createWrapper(),
    });

    result.current.mutate({ email: "user@example.com" });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe("Failed to join waitlist (400)");
  });
});

describe("useWaitlistStatus", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches waitlist status for given email", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ position: 3, accepted: false }),
    });

    const { result } = renderHook(() => useWaitlistStatus("user@example.com"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining("/waitlist/status?email=user%40example.com"),
    );

    expect(result.current.data).toEqual({ position: 3, accepted: false });
  });

  it("does not fetch when email is null", () => {
    renderHook(() => useWaitlistStatus(null), {
      wrapper: createWrapper(),
    });

    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("throws error when status request fails", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
    });

    const { result } = renderHook(() => useWaitlistStatus("user@example.com"), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe("Failed to check waitlist status (404)");
  });
});
