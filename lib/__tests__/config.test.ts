import { describe, it, expect, vi, beforeEach } from "vitest";

const storage = new Map<string, string>();
vi.mock("@react-native-async-storage/async-storage", () => ({
  default: {
    getItem: vi.fn((key: string) => Promise.resolve(storage.get(key) ?? null)),
    setItem: vi.fn((key: string, value: string) => {
      storage.set(key, value);
      return Promise.resolve();
    }),
    removeItem: vi.fn((key: string) => {
      storage.delete(key);
      return Promise.resolve();
    }),
  },
}));

import { normalizeServerUrl, setServerUrl, onConfigChange, getConfig } from "@/lib/config";

describe("normalizeServerUrl", () => {
  it.each([
    ["https://toqui.example.com", "https://toqui.example.com"],
    ["https://toqui.example.com/", "https://toqui.example.com"],
    ["https://toqui.example.com///", "https://toqui.example.com"],
    ["  https://toqui.example.com  ", "https://toqui.example.com"],
    // No scheme → assume https (self-hosters behind a reverse proxy).
    ["toqui.example.com", "https://toqui.example.com"],
    // Explicit http allowed (LAN deployments).
    ["http://192.168.1.50:8090", "http://192.168.1.50:8090"],
    ["https://toqui.example.com:8443", "https://toqui.example.com:8443"],
  ])("normalizes %s to %s", (input, want) => {
    expect(normalizeServerUrl(input)).toBe(want);
  });

  it.each([
    [""],
    ["   "],
    ["ftp://example.com"],
    ["not a url at all"],
    ["https://"],
  ])("rejects %s", (input) => {
    expect(normalizeServerUrl(input)).toBeNull();
  });
});

describe("setServerUrl", () => {
  beforeEach(() => {
    storage.clear();
  });

  it("persists the normalized URL, updates config, and notifies listeners", async () => {
    const listener = vi.fn();
    const unsubscribe = onConfigChange(listener);

    await setServerUrl("toqui.example.com/");

    expect(storage.get("toqui_server_url")).toBe("https://toqui.example.com");
    expect(getConfig().apiUrl).toBe("https://toqui.example.com");
    expect(listener).toHaveBeenCalledTimes(1);

    unsubscribe();
    await setServerUrl("https://other.example.com");
    expect(listener).toHaveBeenCalledTimes(1); // unsubscribed — no second call
  });

  it("rejects invalid URLs without persisting", async () => {
    await expect(setServerUrl("ftp://nope")).rejects.toThrow(/invalid server URL/);
    expect(storage.has("toqui_server_url")).toBe(false);
  });
});
