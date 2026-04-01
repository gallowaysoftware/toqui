import { useState, useEffect, useCallback, useRef } from "react";
import { Platform } from "react-native";

export interface LocationCoords {
  latitude: number;
  longitude: number;
  accuracy: number | null;
}

export interface UseLocationReturn {
  /** Current location coordinates, or null if not yet obtained */
  location: LocationCoords | null;
  /** Whether location tracking is currently active */
  isTracking: boolean;
  /** Error message if location access failed */
  error: string | null;
  /** Permission state: "prompt" | "granted" | "denied" | "unavailable" */
  permissionState: "prompt" | "granted" | "denied" | "unavailable";
  /** Start tracking location */
  startTracking: () => void;
  /** Stop tracking location */
  stopTracking: () => void;
}

const UPDATE_INTERVAL_MS = 30_000;

/**
 * Cross-platform location hook using the web Geolocation API.
 *
 * expo-location is not installed, so this uses navigator.geolocation on all
 * platforms.  On native (iOS/Android via Expo), the Geolocation polyfill is
 * available through React Native.
 */
export function useLocation(): UseLocationReturn {
  const [location, setLocation] = useState<LocationCoords | null>(null);
  const [isTracking, setIsTracking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [permissionState, setPermissionState] = useState<
    "prompt" | "granted" | "denied" | "unavailable"
  >("prompt");

  const watchIdRef = useRef<number | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  // Check if geolocation is available
  const isAvailable = typeof navigator !== "undefined" && !!navigator.geolocation;

  // Query the permission state (web Permissions API — not available on all
  // platforms, so we fall back gracefully).
  useEffect(() => {
    if (!isAvailable) {
      setPermissionState("unavailable");
      return;
    }

    if (Platform.OS === "web" && navigator.permissions) {
      navigator.permissions
        .query({ name: "geolocation" })
        .then((result) => {
          setPermissionState(result.state === "granted" ? "granted" : result.state === "denied" ? "denied" : "prompt");
          result.addEventListener("change", () => {
            setPermissionState(
              result.state === "granted"
                ? "granted"
                : result.state === "denied"
                  ? "denied"
                  : "prompt",
            );
          });
        })
        .catch(() => {
          // Permissions API not supported — stay at "prompt"
        });
    }
  }, [isAvailable]);

  const clearTracking = useCallback(() => {
    if (watchIdRef.current !== null) {
      navigator.geolocation.clearWatch(watchIdRef.current);
      watchIdRef.current = null;
    }
    if (intervalRef.current !== null) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
  }, []);

  const handlePosition = useCallback((pos: GeolocationPosition) => {
    setLocation({
      latitude: pos.coords.latitude,
      longitude: pos.coords.longitude,
      accuracy: pos.coords.accuracy,
    });
    setError(null);
    setPermissionState("granted");
  }, []);

  const handleError = useCallback((err: GeolocationPositionError) => {
    if (err.code === err.PERMISSION_DENIED) {
      setPermissionState("denied");
      setError("Location permission denied");
      setIsTracking(false);
      clearTracking();
    } else if (err.code === err.POSITION_UNAVAILABLE) {
      setError("Location unavailable");
    } else if (err.code === err.TIMEOUT) {
      setError("Location request timed out");
    }
  }, [clearTracking]);

  const startTracking = useCallback(() => {
    if (!isAvailable) {
      setPermissionState("unavailable");
      setError("Geolocation is not supported");
      return;
    }

    setIsTracking(true);
    setError(null);

    // Get an immediate position
    navigator.geolocation.getCurrentPosition(handlePosition, handleError, {
      enableHighAccuracy: true,
      timeout: 10_000,
      maximumAge: UPDATE_INTERVAL_MS,
    });

    // Poll every 30 seconds instead of continuous watchPosition (battery)
    const id = setInterval(() => {
      navigator.geolocation.getCurrentPosition(handlePosition, handleError, {
        enableHighAccuracy: false,
        timeout: 10_000,
        maximumAge: UPDATE_INTERVAL_MS,
      });
    }, UPDATE_INTERVAL_MS);
    intervalRef.current = id;
  }, [isAvailable, handlePosition, handleError]);

  const stopTracking = useCallback(() => {
    setIsTracking(false);
    clearTracking();
  }, [clearTracking]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      clearTracking();
    };
  }, [clearTracking]);

  return {
    location,
    isTracking,
    error,
    permissionState,
    startTracking,
    stopTracking,
  };
}
