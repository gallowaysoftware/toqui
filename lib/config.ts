import { Platform } from "react-native";

interface AppConfig {
  apiUrl: string;
  googleClientId: string;
  googleIosClientId: string;
}

// Default config from build-time env vars (used in dev / native builds)
const defaults: AppConfig = {
  apiUrl: process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8090",
  googleClientId: process.env.EXPO_PUBLIC_GOOGLE_CLIENT_ID ?? "",
  googleIosClientId: process.env.EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID ?? "",
};

let runtimeConfig: AppConfig | null = null;

/**
 * Load runtime config from /config.json (generated at container startup).
 * Falls back to build-time defaults if unavailable (dev, native).
 *
 * On web in production, the Docker entrypoint generates config.json from
 * Cloud Run env vars (which source from Secret Manager). This means
 * secrets are never baked into the static JS bundle or Docker image.
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
          googleIosClientId: json.googleIosClientId || defaults.googleIosClientId,
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
