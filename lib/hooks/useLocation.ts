import { useState, useEffect, useCallback, useRef } from "react";
import { Platform } from "react-native";
import * as Location from "expo-location";

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
 * Cross-platform location hook.
 *
 * On native (iOS/Android) uses expo-location for reliable permission handling
 * and geolocation. On web, uses the browser Geolocation API.
 */
export function useLocation(): UseLocationReturn {
  const [location, setLocation] = useState<LocationCoords | null>(null);
  const [isTracking, setIsTracking] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [permissionState, setPermissionState] = useState<
    "prompt" | "granted" | "denied" | "unavailable"
  >("prompt");

  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const isWeb = Platform.OS === "web";

  // Check if web geolocation is available (only relevant on web)
  const isWebAvailable =
    isWeb && typeof navigator !== "undefined" && !!navigator.geolocation;

  // Query the permission state on web via the Permissions API.
  useEffect(() => {
    if (!isWeb) return;
    if (!isWebAvailable) {
      setPermissionState("unavailable");
      return;
    }

    if (navigator.permissions) {
      let permStatus: PermissionStatus | null = null;
      const handleChange = () => {
        if (permStatus) {
          setPermissionState(
            permStatus.state === "granted"
              ? "granted"
              : permStatus.state === "denied"
                ? "denied"
                : "prompt",
          );
        }
      };

      navigator.permissions
        .query({ name: "geolocation" })
        .then((result) => {
          permStatus = result;
          setPermissionState(
            result.state === "granted"
              ? "granted"
              : result.state === "denied"
                ? "denied"
                : "prompt",
          );
          result.addEventListener("change", handleChange);
        })
        .catch(() => {
          // Permissions API not supported — stay at "prompt"
        });

      return () => {
        if (permStatus) {
          permStatus.removeEventListener("change", handleChange);
        }
      };
    }
  }, [isWeb, isWebAvailable]);

  const clearTracking = useCallback(() => {
    if (intervalRef.current !== null) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
  }, []);

  const updateLocation = useCallback(
    (coords: { latitude: number; longitude: number; accuracy: number | null }) => {
      setLocation(coords);
      setError(null);
    },
    [],
  );

  // ── Web implementation ──────────────────────────────────────────────────

  const handleWebPosition = useCallback(
    (pos: GeolocationPosition) => {
      updateLocation({
        latitude: pos.coords.latitude,
        longitude: pos.coords.longitude,
        accuracy: pos.coords.accuracy,
      });
      setPermissionState("granted");
    },
    [updateLocation],
  );

  const handleWebError = useCallback(
    (err: GeolocationPositionError) => {
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
    },
    [clearTracking],
  );

  const startWebTracking = useCallback(() => {
    if (!isWebAvailable) {
      setPermissionState("unavailable");
      setError("Geolocation is not supported");
      return;
    }

    setIsTracking(true);
    setError(null);

    navigator.geolocation.getCurrentPosition(handleWebPosition, handleWebError, {
      enableHighAccuracy: true,
      timeout: 10_000,
      maximumAge: UPDATE_INTERVAL_MS,
    });

    const id = setInterval(() => {
      navigator.geolocation.getCurrentPosition(
        handleWebPosition,
        handleWebError,
        {
          enableHighAccuracy: false,
          timeout: 10_000,
          maximumAge: UPDATE_INTERVAL_MS,
        },
      );
    }, UPDATE_INTERVAL_MS);
    intervalRef.current = id;
  }, [isWebAvailable, handleWebPosition, handleWebError]);

  // ── Native implementation (expo-location) ───────────────────────────────

  const fetchNativePosition = useCallback(async () => {
    try {
      const pos = await Location.getCurrentPositionAsync({
        accuracy: Location.Accuracy.Balanced,
      });
      updateLocation({
        latitude: pos.coords.latitude,
        longitude: pos.coords.longitude,
        accuracy: pos.coords.accuracy,
      });
    } catch {
      setError("Location unavailable");
    }
  }, [updateLocation]);

  const startNativeTracking = useCallback(async () => {
    setIsTracking(true);
    setError(null);

    try {
      const { status } = await Location.requestForegroundPermissionsAsync();
      if (status !== "granted") {
        setPermissionState("denied");
        setError("Location permission denied");
        setIsTracking(false);
        return;
      }

      setPermissionState("granted");

      // Get an immediate position
      await fetchNativePosition();

      // Poll every 30 seconds
      const id = setInterval(() => {
        void fetchNativePosition();
      }, UPDATE_INTERVAL_MS);
      intervalRef.current = id;
    } catch {
      setError("Location unavailable");
      setIsTracking(false);
    }
  }, [fetchNativePosition]);

  // ── Unified API ─────────────────────────────────────────────────────────

  const startTracking = useCallback(() => {
    if (isWeb) {
      startWebTracking();
    } else {
      void startNativeTracking();
    }
  }, [isWeb, startWebTracking, startNativeTracking]);

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
