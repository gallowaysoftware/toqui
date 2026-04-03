import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";

// ---------------------------------------------------------------------------
// Mock expo-location (must be before the import of useLocation)
// ---------------------------------------------------------------------------

const mockRequestForegroundPermissions = vi.fn();
const mockGetCurrentPositionAsync = vi.fn();

vi.mock("expo-location", () => ({
  requestForegroundPermissionsAsync: (...args: unknown[]) =>
    mockRequestForegroundPermissions(...args),
  getCurrentPositionAsync: (...args: unknown[]) =>
    mockGetCurrentPositionAsync(...args),
  Accuracy: { Balanced: 3 },
}));

// ---------------------------------------------------------------------------
// Mock navigator.geolocation (web path)
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

function simulateWebPosition(lat: number, lng: number, accuracy = 10) {
  mockGetCurrentPosition.mockImplementation(
    (success: PositionCallback) => {
      success({
        coords: {
          latitude: lat,
          longitude: lng,
          accuracy,
          altitude: null,
          altitudeAccuracy: null,
          heading: null,
          speed: null,
        },
        timestamp: Date.now(),
      } as GeolocationPosition);
    },
  );
}

function simulateWebError(code: number) {
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
// Web tests (Platform.OS === "web" in jsdom via react-native-web alias)
// ---------------------------------------------------------------------------

describe("useLocation (web)", () => {
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
    simulateWebPosition(40.7128, -74.006);

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
    simulateWebPosition(40.7128, -74.006);

    const { result } = renderHook(() => useLocation());
    act(() => result.current.startTracking());
    expect(result.current.isTracking).toBe(true);

    act(() => result.current.stopTracking());
    expect(result.current.isTracking).toBe(false);
  });

  it("sets permission denied when geolocation returns PERMISSION_DENIED", () => {
    simulateWebError(1);

    const { result } = renderHook(() => useLocation());
    act(() => result.current.startTracking());

    expect(result.current.permissionState).toBe("denied");
    expect(result.current.error).toBe("Location permission denied");
    expect(result.current.isTracking).toBe(false);
  });

  it("sets error for POSITION_UNAVAILABLE", () => {
    simulateWebError(2);

    const { result } = renderHook(() => useLocation());
    act(() => result.current.startTracking());

    expect(result.current.error).toBe("Location unavailable");
  });

  it("sets error for TIMEOUT", () => {
    simulateWebError(3);

    const { result } = renderHook(() => useLocation());
    act(() => result.current.startTracking());

    expect(result.current.error).toBe("Location request timed out");
  });

  it("polls location every 30 seconds", () => {
    simulateWebPosition(40.7128, -74.006);

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
    simulateWebPosition(40.7128, -74.006);

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

// ---------------------------------------------------------------------------
// Native tests (expo-location path)
// ---------------------------------------------------------------------------

describe("useLocation (native)", () => {
  let platformSpy: ReturnType<typeof vi.spyOn>;

  // Flush microtask queue without touching fake timers
  const tick = () => vi.waitFor(() => {});

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.clearAllMocks();

    // Patch Platform.OS to simulate native
    const RN = await import("react-native");
    platformSpy = vi.spyOn(RN.Platform, "OS", "get").mockReturnValue("ios" as never);
  });

  afterEach(() => {
    vi.useRealTimers();
    platformSpy.mockRestore();
  });

  it("requests foreground permissions and gets position on native", async () => {
    mockRequestForegroundPermissions.mockResolvedValue({ status: "granted" });
    mockGetCurrentPositionAsync.mockResolvedValue({
      coords: { latitude: 48.8566, longitude: 2.3522, accuracy: 5 },
      timestamp: Date.now(),
    });

    const { result } = renderHook(() => useLocation());

    await act(async () => {
      result.current.startTracking();
      await tick();
    });

    expect(mockRequestForegroundPermissions).toHaveBeenCalled();
    expect(mockGetCurrentPositionAsync).toHaveBeenCalled();
    expect(result.current.location).toEqual({
      latitude: 48.8566,
      longitude: 2.3522,
      accuracy: 5,
    });
    expect(result.current.permissionState).toBe("granted");
    expect(result.current.isTracking).toBe(true);
  });

  it("sets denied when native permission is not granted", async () => {
    mockRequestForegroundPermissions.mockResolvedValue({ status: "denied" });

    const { result } = renderHook(() => useLocation());

    await act(async () => {
      result.current.startTracking();
      await tick();
    });

    expect(result.current.permissionState).toBe("denied");
    expect(result.current.error).toBe("Location permission denied");
    expect(result.current.isTracking).toBe(false);
    expect(mockGetCurrentPositionAsync).not.toHaveBeenCalled();
  });

  it("polls native position every 30 seconds", async () => {
    mockRequestForegroundPermissions.mockResolvedValue({ status: "granted" });
    mockGetCurrentPositionAsync.mockResolvedValue({
      coords: { latitude: 48.8566, longitude: 2.3522, accuracy: 5 },
      timestamp: Date.now(),
    });

    const { result } = renderHook(() => useLocation());

    await act(async () => {
      result.current.startTracking();
      await tick();
    });

    expect(mockGetCurrentPositionAsync).toHaveBeenCalledTimes(1);

    await act(async () => {
      vi.advanceTimersByTime(30_000);
      await tick();
    });
    expect(mockGetCurrentPositionAsync).toHaveBeenCalledTimes(2);
  });

  it("stops polling on stopTracking", async () => {
    mockRequestForegroundPermissions.mockResolvedValue({ status: "granted" });
    mockGetCurrentPositionAsync.mockResolvedValue({
      coords: { latitude: 48.8566, longitude: 2.3522, accuracy: 5 },
      timestamp: Date.now(),
    });

    const { result } = renderHook(() => useLocation());

    await act(async () => {
      result.current.startTracking();
      await tick();
    });

    act(() => result.current.stopTracking());
    expect(result.current.isTracking).toBe(false);

    mockGetCurrentPositionAsync.mockClear();
    act(() => vi.advanceTimersByTime(60_000));
    expect(mockGetCurrentPositionAsync).not.toHaveBeenCalled();
  });
});
