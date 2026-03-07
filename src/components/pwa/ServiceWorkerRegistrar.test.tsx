import { describe, it, expect, vi, beforeEach } from "vitest";
import { render } from "@testing-library/react";
import { ServiceWorkerRegistrar } from "./ServiceWorkerRegistrar";

describe("ServiceWorkerRegistrar", () => {
  const mockRegister = vi.fn(() => Promise.resolve({} as ServiceWorkerRegistration));

  beforeEach(() => {
    vi.clearAllMocks();
    // Reset to undefined (jsdom default — serviceWorker key exists but is undefined)
    Object.defineProperty(navigator, "serviceWorker", {
      value: undefined,
      configurable: true,
      writable: true,
    });
  });

  it("registers the service worker when supported", () => {
    Object.defineProperty(navigator, "serviceWorker", {
      value: { register: mockRegister },
      configurable: true,
      writable: true,
    });

    render(<ServiceWorkerRegistrar />);

    expect(mockRegister).toHaveBeenCalledWith("/sw.js");
    expect(mockRegister).toHaveBeenCalledTimes(1);
  });

  it("does not call register when serviceWorker is unavailable", () => {
    // serviceWorker is undefined (set in beforeEach), so register should not be called
    render(<ServiceWorkerRegistrar />);

    expect(mockRegister).not.toHaveBeenCalled();
  });

  it("renders nothing (returns null)", () => {
    // serviceWorker is undefined (set in beforeEach), no side-effect issues
    const { container } = render(<ServiceWorkerRegistrar />);

    expect(container.innerHTML).toBe("");
  });

  it("logs error when registration fails", async () => {
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    const registrationError = new Error("Registration failed");
    const failingRegister = vi.fn(() => Promise.reject(registrationError));

    Object.defineProperty(navigator, "serviceWorker", {
      value: { register: failingRegister },
      configurable: true,
      writable: true,
    });

    render(<ServiceWorkerRegistrar />);

    // Wait for the promise rejection to be handled
    await vi.waitFor(() => {
      expect(consoleSpy).toHaveBeenCalledWith(
        "Service worker registration failed:",
        registrationError,
      );
    });

    consoleSpy.mockRestore();
  });
});
