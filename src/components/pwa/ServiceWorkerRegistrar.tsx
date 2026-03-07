"use client";

import { useEffect } from "react";

/**
 * Registers the service worker on mount.
 * Rendered in the root layout so SW registration happens on every page load.
 */
export function ServiceWorkerRegistrar() {
  useEffect(() => {
    if ("serviceWorker" in navigator && navigator.serviceWorker) {
      navigator.serviceWorker.register("/sw.js").catch((error) => {
        console.error("Service worker registration failed:", error);
      });
    }
  }, []);

  return null;
}
