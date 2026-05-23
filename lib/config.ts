import { Platform } from "react-native";

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
 * Load runtime config from /config.json (generated at container startup).
 * Falls back to build-time defaults if unavailable (dev, native).
 *
 * On web in production, the Docker entrypoint generates config.json from
 * container env vars. This means env-injected values are never baked into
 * the static JS bundle or Docker image.
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
  }

  runtimeConfig = defaults;
  return runtimeConfig;
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
