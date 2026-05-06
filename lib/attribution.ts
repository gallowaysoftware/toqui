// Attribution: read the UTM/ref payload captured by the marketing site
// (toqui-site/AttributionCapture.astro) on first visit, forward it on the
// signup gRPC call, and clear it afterward so a returning user's later
// sign-in doesn't get re-attributed.
//
// Storage:
//   - **Web**: a first-party cookie `toqui_attribution` on Domain=.toqui.travel,
//     written by the marketing site post-consent. We re-encode the parsed
//     value as base64-JSON when sending to the backend (the backend accepts
//     either standard or URL-safe base64).
//   - **Native**: there's no marketing-site → app cookie hop on iOS/Android.
//     Native users come straight to the app and may have an `app.toqui.travel`
//     deep-link from a Product Hunt comment etc., but for now the native
//     path is best-effort: we look for an `AsyncStorage` key (`toqui_attribution`)
//     that a future deep-link handler could populate, and ship empty otherwise.
//
// Privacy: the cookie is deleted after a successful login so we never re-send
// stale attribution. AsyncStorage value is removed alongside.
//
// See: audit issue #39 A-2.
import { Platform } from "react-native";

const STORAGE_KEY = "toqui_attribution";
const COOKIE_NAME = "toqui_attribution";

/**
 * Parse `document.cookie` for a single cookie value. Returns the URL-decoded
 * value or null if absent. Native: always returns null.
 */
function readCookie(name: string): string | null {
  if (Platform.OS !== "web" || typeof document === "undefined") return null;
  const cookies = document.cookie ? document.cookie.split("; ") : [];
  for (const c of cookies) {
    const eq = c.indexOf("=");
    if (eq < 0) continue;
    const k = c.substring(0, eq);
    if (k === name) {
      try {
        return decodeURIComponent(c.substring(eq + 1));
      } catch {
        return c.substring(eq + 1);
      }
    }
  }
  return null;
}

/**
 * Delete the cross-subdomain cookie. The marketing site sets it with
 * Domain=.toqui.travel; clearing it requires matching that exactly.
 * Best-effort: also clears a host-only variant in case a stray dev
 * environment wrote it without a domain.
 */
function clearCookie(): void {
  if (Platform.OS !== "web" || typeof document === "undefined") return;
  const past = "Thu, 01 Jan 1970 00:00:00 GMT";
  const isProd =
    typeof window !== "undefined" &&
    /(?:^|\.)toqui\.travel$/.test(window.location.hostname);
  const sec = isProd ? "; Secure" : "";
  document.cookie = `${COOKIE_NAME}=; Domain=.toqui.travel; Path=/; Expires=${past}; SameSite=Lax${sec}`;
  document.cookie = `${COOKIE_NAME}=; Path=/; Expires=${past}; SameSite=Lax${sec}`;
}

/**
 * Returns the captured attribution as a base64-encoded JSON string
 * suitable for the gRPC `attribution` field, or empty string when none
 * is available. Never throws — attribution is best-effort metadata.
 *
 * On web: reads the cookie. The marketing site stored it as base64(JSON);
 * we pass it through unchanged so the backend sees the exact bytes the
 * user's browser had stored (avoids a JSON parse → re-encode round-trip
 * that could mangle Unicode).
 *
 * On native: looks for an AsyncStorage key (placeholder for future deep-link
 * support; returns empty today).
 */
export async function readAttributionEncoded(): Promise<string> {
  try {
    if (Platform.OS === "web") {
      const cookie = readCookie(COOKIE_NAME);
      if (cookie) return cookie;
      // Fallback: localStorage (set pre-consent; useful in dev where
      // the cookie may not have been promoted, or on subdomains the
      // cookie didn't reach).
      if (typeof localStorage !== "undefined") {
        const ls = localStorage.getItem(STORAGE_KEY);
        if (ls) {
          // localStorage stores raw JSON, not base64. Re-encode for the
          // wire format the backend expects.
          try {
            return btoa(ls);
          } catch {
            return "";
          }
        }
      }
      return "";
    }
    // Native: look for a future deep-link-populated value.
    const { default: AsyncStorage } = await import(
      "@react-native-async-storage/async-storage"
    );
    const v = await AsyncStorage.getItem(STORAGE_KEY);
    return v ?? "";
  } catch {
    return "";
  }
}

/**
 * Delete attribution from cookie + AsyncStorage + localStorage. Call after
 * a successful signup so we don't re-attribute a later sign-in to the
 * same launch campaign.
 */
export async function clearAttribution(): Promise<void> {
  try {
    if (Platform.OS === "web") {
      clearCookie();
      if (typeof localStorage !== "undefined") {
        localStorage.removeItem(STORAGE_KEY);
      }
      return;
    }
    const { default: AsyncStorage } = await import(
      "@react-native-async-storage/async-storage"
    );
    await AsyncStorage.removeItem(STORAGE_KEY);
  } catch {
    // Swallow — clearing is best-effort. Worst case: we attribute
    // a returning user once more.
  }
}
