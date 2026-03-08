import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { ThemeProvider, useTheme } from "./ThemeProvider";

// Helper component that exposes theme context values for testing
function ThemeConsumer() {
  const { theme, resolvedTheme, setTheme } = useTheme();
  return (
    <div>
      <span data-testid="theme">{theme}</span>
      <span data-testid="resolved">{resolvedTheme}</span>
      <button data-testid="set-dark" onClick={() => setTheme("dark")}>
        Dark
      </button>
      <button data-testid="set-light" onClick={() => setTheme("light")}>
        Light
      </button>
      <button data-testid="set-system" onClick={() => setTheme("system")}>
        System
      </button>
    </div>
  );
}

// Mock matchMedia
function createMatchMediaMock(prefersDark: boolean) {
  const listeners: Array<(e: MediaQueryListEvent) => void> = [];
  return vi.fn().mockImplementation((query: string) => ({
    matches: query === "(prefers-color-scheme: dark)" ? prefersDark : false,
    media: query,
    addEventListener: (_: string, cb: (e: MediaQueryListEvent) => void) => {
      listeners.push(cb);
    },
    removeEventListener: (_: string, cb: (e: MediaQueryListEvent) => void) => {
      const idx = listeners.indexOf(cb);
      if (idx >= 0) listeners.splice(idx, 1);
    },
    // Expose listeners for test control
    _listeners: listeners,
  }));
}

describe("ThemeProvider", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.classList.remove("dark");
    window.matchMedia = createMatchMediaMock(false);
  });

  it("defaults to system theme", () => {
    render(
      <ThemeProvider>
        <ThemeConsumer />
      </ThemeProvider>,
    );
    expect(screen.getByTestId("theme").textContent).toBe("system");
  });

  it("resolves system theme to light when prefers-color-scheme is light", () => {
    window.matchMedia = createMatchMediaMock(false);
    render(
      <ThemeProvider>
        <ThemeConsumer />
      </ThemeProvider>,
    );
    expect(screen.getByTestId("resolved").textContent).toBe("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });

  it("resolves system theme to dark when prefers-color-scheme is dark", () => {
    window.matchMedia = createMatchMediaMock(true);
    render(
      <ThemeProvider>
        <ThemeConsumer />
      </ThemeProvider>,
    );
    expect(screen.getByTestId("resolved").textContent).toBe("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("sets theme to dark and adds dark class to document", () => {
    render(
      <ThemeProvider>
        <ThemeConsumer />
      </ThemeProvider>,
    );

    act(() => {
      fireEvent.click(screen.getByTestId("set-dark"));
    });

    expect(screen.getByTestId("theme").textContent).toBe("dark");
    expect(screen.getByTestId("resolved").textContent).toBe("dark");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("sets theme to light and removes dark class from document", () => {
    document.documentElement.classList.add("dark");
    render(
      <ThemeProvider>
        <ThemeConsumer />
      </ThemeProvider>,
    );

    act(() => {
      fireEvent.click(screen.getByTestId("set-light"));
    });

    expect(screen.getByTestId("theme").textContent).toBe("light");
    expect(screen.getByTestId("resolved").textContent).toBe("light");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });

  it("persists theme preference to localStorage", () => {
    render(
      <ThemeProvider>
        <ThemeConsumer />
      </ThemeProvider>,
    );

    act(() => {
      fireEvent.click(screen.getByTestId("set-dark"));
    });

    expect(localStorage.getItem("toqui_theme")).toBe("dark");
  });

  it("reads theme preference from localStorage on mount", () => {
    localStorage.setItem("toqui_theme", "dark");

    render(
      <ThemeProvider>
        <ThemeConsumer />
      </ThemeProvider>,
    );

    expect(screen.getByTestId("theme").textContent).toBe("dark");
    expect(screen.getByTestId("resolved").textContent).toBe("dark");
  });

  it("ignores invalid localStorage values", () => {
    localStorage.setItem("toqui_theme", "invalid-value");

    render(
      <ThemeProvider>
        <ThemeConsumer />
      </ThemeProvider>,
    );

    expect(screen.getByTestId("theme").textContent).toBe("system");
  });

  it("throws when useTheme is used outside ThemeProvider", () => {
    // Suppress the expected error from console
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(() => render(<ThemeConsumer />)).toThrow("useTheme must be used within a ThemeProvider");

    spy.mockRestore();
  });
});
