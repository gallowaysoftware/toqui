import { describe, it, expect, vi, beforeEach } from "vitest";

// needsServerSetup is native-only behavior — run this file as "ios".
vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>("react-native");
  return { ...actual, Platform: { OS: "ios" } };
});

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
    // Query strings / fragments have no meaning in a server base URL.
    ["https://host.example.com?x=1"],
    ["https://host.example.com/api#frag"],
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

describe("needsServerSetup", () => {
  // Fresh module instance per test — hasStoredServerUrl and runtimeConfig
  // are module state.
  async function freshConfig() {
    vi.resetModules();
    return await import("@/lib/config");
  }

  beforeEach(() => {
    storage.clear();
  });

  it("gates on native when nothing is configured (localhost default)", async () => {
    const cfg = await freshConfig();
    await cfg.loadConfig();
    expect(cfg.needsServerSetup()).toBe(true);
  });

  it("does not gate once a stored URL is loaded", async () => {
    storage.set("toqui_server_url", "https://toqui.example.com");
    const cfg = await freshConfig();
    await cfg.loadConfig();
    expect(cfg.needsServerSetup()).toBe(false);
    expect(cfg.getConfig().apiUrl).toBe("https://toqui.example.com");
  });

  it("does not gate after the user explicitly saves the localhost default (no redirect loop)", async () => {
    // Simulator testing of a store build: the user saves
    // http://localhost:8090 itself. String-matching the default would
    // bounce them back to setup forever.
    const cfg = await freshConfig();
    await cfg.loadConfig();
    expect(cfg.needsServerSetup()).toBe(true);
    await cfg.setServerUrl("http://localhost:8090");
    expect(cfg.needsServerSetup()).toBe(false);
  });
});
