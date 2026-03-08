import { describe, it, expect, vi } from "vitest";

// Mock next/font/google (Inter is used in layout.tsx)
vi.mock("next/font/google", () => ({
  Inter: () => ({ className: "inter" }),
}));

// Mock server-only next-intl functions so the module can be imported
vi.mock("next-intl/server", () => ({
  getLocale: vi.fn(() => Promise.resolve("en")),
  getMessages: vi.fn(() => Promise.resolve({})),
}));

// Mock the providers and PWA component so the module can be imported
vi.mock("@/components/providers/Providers", () => ({
  Providers: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("@/components/pwa/ServiceWorkerRegistrar", () => ({
  ServiceWorkerRegistrar: () => null,
}));

describe("RootLayout metadata", () => {
  it("includes the PWA manifest link", async () => {
    const { metadata } = await import("./layout");

    expect(metadata.manifest).toBe("/manifest.json");
  });

  it("sets the theme color via viewport export", async () => {
    const { viewport } = await import("./layout");

    expect(viewport.themeColor).toBe("#E8654A");
  });

  it("configures Apple mobile web app settings", async () => {
    const { metadata } = await import("./layout");

    expect(metadata.appleWebApp).toEqual(
      expect.objectContaining({
        capable: true,
        statusBarStyle: "default",
        title: "Toqui",
      }),
    );
  });
});
