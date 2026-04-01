import { useState, useEffect } from "react";
import { Platform } from "react-native";

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
        await fetch("https://clients3.google.com/generate_204", {
          method: "HEAD",
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
