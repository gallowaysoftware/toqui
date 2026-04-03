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
  }),
}));

import {
  hashUserId,
  stripSensitiveProps,
  AnalyticsProvider,
  useAnalytics,
} from "../analytics";

describe("hashUserId", () => {
  it("returns a string prefixed with u_", () => {
    const result = hashUserId("user-123");
    expect(result).toMatch(/^u_[a-z0-9]+$/);
  });

  it("is deterministic", () => {
    expect(hashUserId("abc")).toBe(hashUserId("abc"));
  });

  it("produces different hashes for different IDs", () => {
    expect(hashUserId("user-1")).not.toBe(hashUserId("user-2"));
  });

  it("never returns the raw user ID", () => {
    const id = "550e8400-e29b-41d4-a716-446655440000";
    const hashed = hashUserId(id);
    expect(hashed).not.toContain(id);
  });
});

describe("stripSensitiveProps", () => {
  it("removes sensitive keys", () => {
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
      safe_prop: 42,
    };
    const result = stripSensitiveProps(input);
    expect(result).toEqual({ platform: "web", safe_prop: 42 });
  });

  it("returns undefined for undefined input", () => {
    expect(stripSensitiveProps(undefined)).toBeUndefined();
  });

  it("passes through safe properties unchanged", () => {
    const input = { platform: "ios", has_dates: true, count: 5 };
    expect(stripSensitiveProps(input)).toEqual(input);
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

  it("initialises PostHog with EU host", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(AnalyticsProvider, null, children);
    renderHook(() => useAnalytics(), { wrapper });
    expect(mockInit).toHaveBeenCalledWith(
      "phc_test_key",
      expect.objectContaining({
        api_host: "https://eu.i.posthog.com",
        persistence: "memory",
        autocapture: false,
      }),
    );
  });

  it("track fires posthog.capture with sanitised properties", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(AnalyticsProvider, null, children);
    const { result } = renderHook(() => useAnalytics(), { wrapper });
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

  it("identify sends hashed user ID", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(AnalyticsProvider, null, children);
    const { result } = renderHook(() => useAnalytics(), { wrapper });
    act(() => {
      result.current.identify("user-uuid-123");
    });
    const hashedId = hashUserId("user-uuid-123");
    expect(mockIdentify).toHaveBeenCalledWith(hashedId);
  });

  it("reset calls posthog.reset", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(AnalyticsProvider, null, children);
    const { result } = renderHook(() => useAnalytics(), { wrapper });
    act(() => {
      result.current.reset();
    });
    expect(mockReset).toHaveBeenCalled();
  });
});
