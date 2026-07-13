import { Platform } from "react-native";
import AsyncStorage from "@react-native-async-storage/async-storage";

interface AppConfig {
  apiUrl: string;
  googleClientId: string;
  /**
   * Public-facing URL where this Toqui instance serves the web frontend.
   * Used for share links + OG meta tags. Self-hosters set this via the
   * EXPO_PUBLIC_PUBLIC_URL env var (read at build time on native, runtime
   * on web via /config.json). On web, an empty value means "derive from
   * window.location.origin at use time" — which is the right default for
   * any setup where the user accesses the app at the same origin that
   * shares are served from.
   */
  publicUrl: string;
}

// Default config from build-time env vars (used in dev / native builds)
const defaults: AppConfig = {
  apiUrl: process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8090",
  googleClientId: process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID ?? "",
  publicUrl: process.env.EXPO_PUBLIC_PUBLIC_URL ?? "",
};

let runtimeConfig: AppConfig | null = null;

/**
 * Storage key for the user-chosen server URL on native (bring-your-own-
 * server). Web never uses this — the web app is served BY a specific
 * server, so its API URL comes from that server's /config.json.
 */
const SERVER_URL_STORAGE_KEY = "toqui_server_url";

const changeListeners = new Set<() => void>();

/**
 * Subscribe to config changes (currently: the user switching servers on
 * native). Returns an unsubscribe function. The root layout uses this to
 * remount the provider tree so transports pick up the new API URL.
 */
export function onConfigChange(listener: () => void): () => void {
  changeListeners.add(listener);
  return () => changeListeners.delete(listener);
}

/**
 * Normalize a user-entered server URL: trim, default to https:// when no
 * scheme is given, and strip trailing slashes. Returns null when the
 * value can't be a valid http(s) URL.
 */
export function normalizeServerUrl(input: string): string | null {
  let value = input.trim();
  if (!value) return null;
  const hasScheme = /^[a-z][a-z0-9+.-]*:\/\//i.test(value);
  if (hasScheme) {
    if (!/^https?:\/\//i.test(value)) return null; // ftp://, ws://, etc.
  } else {
    value = `https://${value}`;
  }
  try {
    const url = new URL(value);
    if (url.protocol !== "http:" && url.protocol !== "https:") return null;
    if (!url.hostname) return null;
    return value.replace(/\/+$/, "");
  } catch {
    return null;
  }
}

/**
 * Load runtime config. Precedence:
 * - Web: /config.json (generated at container startup by the Docker
 *   entrypoint from env vars — nothing is baked into the static bundle),
 *   falling back to build-time defaults (dev server).
 * - Native: the user-chosen server URL from device storage
 *   (bring-your-own-server), falling back to the build-time
 *   EXPO_PUBLIC_API_URL default.
 */
export async function loadConfig(): Promise<AppConfig> {
  if (runtimeConfig) return runtimeConfig;

  if (Platform.OS === "web") {
    try {
      const res = await fetch("/config.json");
      if (res.ok) {
        const json = await res.json();
        runtimeConfig = {
          apiUrl: json.apiUrl || defaults.apiUrl,
          googleClientId: json.googleClientId || defaults.googleClientId,
          publicUrl: json.publicUrl || defaults.publicUrl,
        };
        return runtimeConfig;
      }
    } catch {
      // Fall through to defaults (dev server, no config.json)
    }
    runtimeConfig = defaults;
    return runtimeConfig;
  }

  try {
    const stored = await AsyncStorage.getItem(SERVER_URL_STORAGE_KEY);
    const normalized = stored ? normalizeServerUrl(stored) : null;
    runtimeConfig = normalized ? { ...defaults, apiUrl: normalized } : defaults;
  } catch {
    runtimeConfig = defaults;
  }
  return runtimeConfig;
}

/**
 * True when the native app has no server to talk to: nothing chosen by
 * the user yet and no EXPO_PUBLIC_API_URL baked in at build time (the
 * localhost default only makes sense for dev). Drives the first-run
 * server-setup screen for App Store / TestFlight builds.
 */
export function needsServerSetup(): boolean {
  if (Platform.OS === "web") return false;
  const cfg = getConfig();
  return cfg.apiUrl === "" || (cfg.apiUrl === "http://localhost:8090" && !process.env.EXPO_PUBLIC_API_URL);
}

/**
 * Persist the user-chosen server URL (native bring-your-own-server),
 * update the in-memory config, and notify subscribers so the provider
 * tree remounts with transports pointing at the new server.
 */
export async function setServerUrl(url: string): Promise<void> {
  const normalized = normalizeServerUrl(url);
  if (!normalized) {
    throw new Error(`invalid server URL: ${url}`);
  }
  await AsyncStorage.setItem(SERVER_URL_STORAGE_KEY, normalized);
  runtimeConfig = { ...(runtimeConfig ?? defaults), apiUrl: normalized };
  changeListeners.forEach((listener) => listener());
}

/** Synchronous access after loadConfig() has been called */
export function getConfig(): AppConfig {
  return runtimeConfig ?? defaults;
}

/**
 * Resolve an absolute URL for a path on this Toqui instance — used for
 * share links and OG meta tags. On web, defaults to `window.location.origin`
 * (anyone visiting the share URL will hit the same origin they're already
 * on). On native, falls back to the configured publicUrl. Returns just the
 * path if no origin can be resolved (so the user still gets *something*
 * usable to copy + paste).
 */
export function getPublicUrl(path: string): string {
  const cfg = getConfig();
  if (cfg.publicUrl) return cfg.publicUrl.replace(/\/$/, "") + path;
  if (Platform.OS === "web" && typeof window !== "undefined" && window.location?.origin) {
    return window.location.origin + path;
  }
  return path;
}
