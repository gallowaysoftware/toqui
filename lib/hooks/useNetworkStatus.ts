import { useState, useEffect } from "react";
import { Platform } from "react-native";

import { getConfig } from "@/lib/config";

export interface NetworkStatus {
  isConnected: boolean;
  isInternetReachable: boolean | null;
}

/**
 * Cross-platform network status hook.
 *
 * - Web: uses `navigator.onLine` + `online`/`offline` events.
 * - Native (without @react-native-community/netinfo): polls a tiny endpoint
 *   every 30 seconds to check reachability.
 */
export function useNetworkStatus(): NetworkStatus {
  const [status, setStatus] = useState<NetworkStatus>(() => {
    if (Platform.OS === "web" && typeof window !== "undefined") {
      return {
        isConnected: navigator.onLine,
        isInternetReachable: navigator.onLine,
      };
    }
    // Assume connected until proven otherwise on native
    return { isConnected: true, isInternetReachable: null };
  });

  useEffect(() => {
    if (Platform.OS === "web" && typeof window !== "undefined") {
      const handleOnline = () =>
        setStatus({ isConnected: true, isInternetReachable: true });
      const handleOffline = () =>
        setStatus({ isConnected: false, isInternetReachable: false });

      window.addEventListener("online", handleOnline);
      window.addEventListener("offline", handleOffline);

      return () => {
        window.removeEventListener("online", handleOnline);
        window.removeEventListener("offline", handleOffline);
      };
    }

    // Native fallback: poll a lightweight endpoint every 30 seconds
    let mounted = true;

    async function check() {
      try {
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), 5000);
        // Probe our own backend (privacy: no third-party connectivity
        // beacons) — reachability of the API is also what actually matters.
        await fetch(`${getConfig().apiUrl}/livez`, {
          method: "GET",
          signal: controller.signal,
        });
        clearTimeout(timeout);
        if (mounted) {
          setStatus({ isConnected: true, isInternetReachable: true });
        }
      } catch {
        if (mounted) {
          setStatus({ isConnected: false, isInternetReachable: false });
        }
      }
    }

    void check();
    const interval = setInterval(check, 30_000);

    return () => {
      mounted = false;
      clearInterval(interval);
    };
  }, []);

  return status;
}
