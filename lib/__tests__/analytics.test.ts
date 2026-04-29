import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import React from "react";

// Mock posthog-js before importing the module under test
const mockCapture = vi.fn();
const mockIdentify = vi.fn();
const mockReset = vi.fn();
const mockInit = vi.fn();

vi.mock("posthog-js", () => ({
  default: {
    init: (...args: unknown[]) => mockInit(...args),
    capture: (...args: unknown[]) => mockCapture(...args),
    identify: (...args: unknown[]) => mockIdentify(...args),
    reset: (...args: unknown[]) => mockReset(...args),
  },
}));

// Mock config — default to empty key (analytics disabled)
let mockPosthogKey = "";
vi.mock("../config", () => ({
  getConfig: () => ({
    apiUrl: "http://localhost:8090",
    googleClientId: "",
    posthogKey: mockPosthogKey,
    sentryDsn: "",
  }),
}));

import {
  hashUserId,
  stripSensitiveProps,
  AnalyticsProvider,
  useAnalytics,
} from "../analytics";

describe("hashUserId", () => {
  it("returns a string prefixed with u_", async () => {
    const result = await hashUserId("user-123");
    expect(result).toMatch(/^u_[a-f0-9]+$/);
  });

  it("is deterministic", async () => {
    const a = await hashUserId("abc");
    const b = await hashUserId("abc");
    expect(a).toBe(b);
  });

  it("produces different hashes for different IDs", async () => {
    const a = await hashUserId("user-1");
    const b = await hashUserId("user-2");
    expect(a).not.toBe(b);
  });

  it("never returns the raw user ID", async () => {
    const id = "550e8400-e29b-41d4-a716-446655440000";
    const hashed = await hashUserId(id);
    expect(hashed).not.toContain(id);
  });

  it("returns a 16-char hex hash (64-bit) when crypto.subtle is available", async () => {
    const result = await hashUserId("test-user");
    // u_ prefix + 16 hex chars
    expect(result).toMatch(/^u_[a-f0-9]{16}$/);
  });
});

describe("stripSensitiveProps", () => {
  it("only keeps allowlisted keys", () => {
    const input = {
      destination: "Paris",
      email: "user@test.com",
      name: "Alice",
      chat_content: "hello",
      message: "hi",
      travel_dates: "2025-01-01",
      start_date: "2025-01-01",
      end_date: "2025-01-02",
      booking_details: "hotel xyz",
      platform: "web",
      unknown_prop: 42,
    };
    const result = stripSensitiveProps(input);
    // Only platform is in the allowlist; unknown_prop is NOT allowlisted
    expect(result).toEqual({ platform: "web" });
  });

  it("returns undefined for undefined input", () => {
    expect(stripSensitiveProps(undefined)).toBeUndefined();
  });

  it("passes through allowlisted properties unchanged", () => {
    const input = { platform: "ios", has_dates: true, item_count: 5 };
    expect(stripSensitiveProps(input)).toEqual(input);
  });

  it("passes through funnel tracking properties (trigger, count, is_first)", () => {
    const input = { trigger: "inline", count: 2, is_first: true, destination: "Paris" };
    expect(stripSensitiveProps(input)).toEqual({ trigger: "inline", count: 2, is_first: true });
  });

  it("strips properties not in the allowlist", () => {
    const input = {
      platform: "web",
      source: "companion",
      custom_dangerous_field: "secret",
      user_data: "private",
    };
    expect(stripSensitiveProps(input)).toEqual({
      platform: "web",
      source: "companion",
    });
  });
});

describe("AnalyticsProvider (key empty = no-op)", () => {
  beforeEach(() => {
    mockPosthogKey = "";
    vi.clearAllMocks();
  });

  it("does not initialise PostHog when key is empty", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(AnalyticsProvider, null, children);
    renderHook(() => useAnalytics(), { wrapper });
    expect(mockInit).not.toHaveBeenCalled();
  });

  it("track is a no-op when key is empty", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(AnalyticsProvider, null, children);
    const { result } = renderHook(() => useAnalytics(), { wrapper });
    act(() => {
      result.current.track("test_event", { foo: "bar" });
    });
    expect(mockCapture).not.toHaveBeenCalled();
  });
});

describe("AnalyticsProvider (key present)", () => {
  beforeEach(() => {
    mockPosthogKey = "phc_test_key";
    vi.clearAllMocks();
  });

  it("initialises PostHog with EU host", async () => {
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(AnalyticsProvider, null, children);
    renderHook(() => useAnalytics(), { wrapper });

    // Init runs after the dynamic import("posthog-js") resolves —
    // wait for it instead of asserting synchronously (lazy-load
    // change for issue #204).
    await vi.waitFor(() => {
      expect(mockInit).toHaveBeenCalledWith(
        "phc_test_key",
        expect.objectContaining({
          api_host: "https://eu.i.posthog.com",
          persistence: "memory",
          autocapture: false,
        }),
      );
    });
  });

  it("track fires posthog.capture with only allowlisted properties (after init)", async () => {
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(AnalyticsProvider, null, children);
    const { result } = renderHook(() => useAnalytics(), { wrapper });

    // Wait for the dynamic import to resolve before tracking, otherwise
    // the call gets queued and the immediate assertion would race the
    // init promise.
    await vi.waitFor(() => expect(mockInit).toHaveBeenCalled());

    act(() => {
      result.current.track("trip_created", {
        has_dates: true,
        destination: "Paris",
      });
    });
    expect(mockCapture).toHaveBeenCalledWith("trip_created", {
      has_dates: true,
    });
  });

  it("track calls fired BEFORE init resolves are queued and replayed (issue #204)", async () => {
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(AnalyticsProvider, null, children);
    const { result } = renderHook(() => useAnalytics(), { wrapper });

    // Fire BEFORE awaiting init — this exercises the queue path.
    // Without queueing, the very first session_start event of every
    // visit (fired in app/_layout.tsx on mount) would be silently
    // dropped while the SDK loads.
    act(() => {
      result.current.track("session_start", { platform: "web" });
    });

    // Now wait for init + flush.
    await vi.waitFor(() => {
      expect(mockCapture).toHaveBeenCalledWith("session_start", {
        platform: "web",
      });
    });
  });

  it("identify sends SHA-256 hashed user ID", async () => {
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(AnalyticsProvider, null, children);
    const { result } = renderHook(() => useAnalytics(), { wrapper });

    // Wait for init before identify so the call doesn't get queued
    // (queued identify would still hash + identify, but the
    // microtask timing differs and makes this test brittle).
    await vi.waitFor(() => expect(mockInit).toHaveBeenCalled());

    act(() => {
      result.current.identify("user-uuid-123");
    });

    // hashUserId is async inside identify; wait for microtask queue
    await vi.waitFor(() => {
      expect(mockIdentify).toHaveBeenCalled();
    });

    const hashedId = await hashUserId("user-uuid-123");
    expect(mockIdentify).toHaveBeenCalledWith(hashedId);
  });

  it("reset calls posthog.reset (after init)", async () => {
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(AnalyticsProvider, null, children);
    const { result } = renderHook(() => useAnalytics(), { wrapper });

    await vi.waitFor(() => expect(mockInit).toHaveBeenCalled());

    act(() => {
      result.current.reset();
    });
    expect(mockReset).toHaveBeenCalled();
  });
});
