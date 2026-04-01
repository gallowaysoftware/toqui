import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

// ---------------------------------------------------------------------------
// Mock navigator.geolocation
// ---------------------------------------------------------------------------

const mockGetCurrentPosition = vi.fn();
const mockClearWatch = vi.fn();

const geolocationMock = {
  getCurrentPosition: mockGetCurrentPosition,
  watchPosition: vi.fn(),
  clearWatch: mockClearWatch,
};

Object.defineProperty(globalThis, "navigator", {
  value: {
    geolocation: geolocationMock,
    permissions: undefined,
  },
  writable: true,
  configurable: true,
});

import { useLocation } from "../useLocation";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function simulatePosition(lat: number, lng: number, accuracy = 10) {
  mockGetCurrentPosition.mockImplementation(
    (success: PositionCallback) => {
      success({
        coords: { latitude: lat, longitude: lng, accuracy, altitude: null, altitudeAccuracy: null, heading: null, speed: null },
        timestamp: Date.now(),
      } as GeolocationPosition);
    },
  );
}

function simulateError(code: number) {
  mockGetCurrentPosition.mockImplementation(
    (_success: PositionCallback, error: PositionErrorCallback) => {
      error({
        code,
        message: "test error",
        PERMISSION_DENIED: 1,
        POSITION_UNAVAILABLE: 2,
        TIMEOUT: 3,
      } as GeolocationPositionError);
    },
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("useLocation", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("starts with default state (not tracking, no location)", () => {
    const { result } = renderHook(() => useLocation());
    expect(result.current.isTracking).toBe(false);
    expect(result.current.location).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it("obtains location when startTracking is called", () => {
    simulatePosition(40.7128, -74.006);

    const { result } = renderHook(() => useLocation());
    act(() => result.current.startTracking());

    expect(result.current.isTracking).toBe(true);
    expect(result.current.location).toEqual({
      latitude: 40.7128,
      longitude: -74.006,
      accuracy: 10,
    });
    expect(result.current.error).toBeNull();
    expect(result.current.permissionState).toBe("granted");
  });

  it("stops tracking when stopTracking is called", () => {
    simulatePosition(40.7128, -74.006);

    const { result } = renderHook(() => useLocation());
    act(() => result.current.startTracking());
    expect(result.current.isTracking).toBe(true);

    act(() => result.current.stopTracking());
    expect(result.current.isTracking).toBe(false);
  });

  it("sets permission denied when geolocation returns PERMISSION_DENIED", () => {
    simulateError(1);

    const { result } = renderHook(() => useLocation());
    act(() => result.current.startTracking());

    expect(result.current.permissionState).toBe("denied");
    expect(result.current.error).toBe("Location permission denied");
    expect(result.current.isTracking).toBe(false);
  });

  it("sets error for POSITION_UNAVAILABLE", () => {
    simulateError(2);

    const { result } = renderHook(() => useLocation());
    act(() => result.current.startTracking());

    expect(result.current.error).toBe("Location unavailable");
  });

  it("sets error for TIMEOUT", () => {
    simulateError(3);

    const { result } = renderHook(() => useLocation());
    act(() => result.current.startTracking());

    expect(result.current.error).toBe("Location request timed out");
  });

  it("polls location every 30 seconds", () => {
    simulatePosition(40.7128, -74.006);

    const { result } = renderHook(() => useLocation());
    act(() => result.current.startTracking());

    // Initial call
    expect(mockGetCurrentPosition).toHaveBeenCalledTimes(1);

    // Advance 30 seconds
    act(() => vi.advanceTimersByTime(30_000));
    expect(mockGetCurrentPosition).toHaveBeenCalledTimes(2);

    // Advance another 30 seconds
    act(() => vi.advanceTimersByTime(30_000));
    expect(mockGetCurrentPosition).toHaveBeenCalledTimes(3);
  });

  it("clears interval on unmount", () => {
    simulatePosition(40.7128, -74.006);

    const { result, unmount } = renderHook(() => useLocation());
    act(() => result.current.startTracking());

    expect(mockGetCurrentPosition).toHaveBeenCalledTimes(1);

    unmount();

    // Advancing time should not trigger more calls
    act(() => vi.advanceTimersByTime(60_000));
    expect(mockGetCurrentPosition).toHaveBeenCalledTimes(1);
  });

  it("reports unavailable when geolocation is missing", () => {
    const origNav = globalThis.navigator;
    Object.defineProperty(globalThis, "navigator", {
      value: { geolocation: undefined },
      writable: true,
      configurable: true,
    });

    const { result } = renderHook(() => useLocation());
    act(() => result.current.startTracking());

    expect(result.current.permissionState).toBe("unavailable");
    expect(result.current.error).toBe("Geolocation is not supported");

    // Restore
    Object.defineProperty(globalThis, "navigator", {
      value: origNav,
      writable: true,
      configurable: true,
    });
  });
});
