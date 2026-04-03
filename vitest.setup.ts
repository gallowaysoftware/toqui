import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// Define __DEV__ global used by React Native / Expo
(globalThis as Record<string, unknown>).__DEV__ = true;

// Mock @sentry/react-native globally — the real module requires native code
// that isn't available in the jsdom test environment.
vi.mock("@sentry/react-native", () => ({
  init: vi.fn(),
  captureException: vi.fn(),
  withScope: vi.fn((cb: (scope: unknown) => void) => {
    cb({ setExtra: vi.fn() });
  }),
}));
