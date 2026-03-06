import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { ThemeProvider } from "@/components/providers/ThemeProvider";
import { ThemeToggleButton, ThemeSelector } from "./ThemeToggle";

// Mock lucide-react icons
vi.mock("lucide-react", () => ({
  Sun: (props: Record<string, unknown>) => <svg data-testid="sun-icon" {...props} />,
  Moon: (props: Record<string, unknown>) => <svg data-testid="moon-icon" {...props} />,
  Monitor: (props: Record<string, unknown>) => <svg data-testid="monitor-icon" {...props} />,
}));

function createMatchMediaMock(prefersDark: boolean) {
  return vi.fn().mockImplementation((query: string) => ({
    matches: query === "(prefers-color-scheme: dark)" ? prefersDark : false,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }));
}

function renderWithTheme(ui: React.ReactElement) {
  return render(<ThemeProvider>{ui}</ThemeProvider>);
}

describe("ThemeToggleButton", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.classList.remove("dark");
    window.matchMedia = createMatchMediaMock(false);
  });

  it("renders a button with accessible label", () => {
    renderWithTheme(<ThemeToggleButton />);
    const button = screen.getByRole("button");
    expect(button).toBeInTheDocument();
    expect(button.getAttribute("aria-label")).toContain("Theme:");
  });

  it("cycles through themes on click: system -> light -> dark -> system", () => {
    renderWithTheme(<ThemeToggleButton />);
    const button = screen.getByRole("button");

    // Default is system
    expect(button.getAttribute("aria-label")).toContain("System");

    // Click: system -> light
    act(() => {
      fireEvent.click(button);
    });
    expect(button.getAttribute("aria-label")).toContain("Light");

    // Click: light -> dark
    act(() => {
      fireEvent.click(button);
    });
    expect(button.getAttribute("aria-label")).toContain("Dark");

    // Click: dark -> system
    act(() => {
      fireEvent.click(button);
    });
    expect(button.getAttribute("aria-label")).toContain("System");
  });
});

describe("ThemeSelector", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.classList.remove("dark");
    window.matchMedia = createMatchMediaMock(false);
  });

  it("renders three theme buttons", () => {
    renderWithTheme(<ThemeSelector />);
    const buttons = screen.getAllByRole("button");
    expect(buttons).toHaveLength(3);
  });

  it("shows all three options: Light, Dark, System", () => {
    renderWithTheme(<ThemeSelector />);
    expect(screen.getByText("Light")).toBeInTheDocument();
    expect(screen.getByText("Dark")).toBeInTheDocument();
    expect(screen.getByText("System")).toBeInTheDocument();
  });

  it("marks the system button as pressed by default", () => {
    renderWithTheme(<ThemeSelector />);
    const systemButton = screen.getByText("System").closest("button");
    expect(systemButton?.getAttribute("aria-pressed")).toBe("true");
  });

  it("selects dark theme when Dark button is clicked", () => {
    renderWithTheme(<ThemeSelector />);

    act(() => {
      fireEvent.click(screen.getByText("Dark"));
    });

    const darkButton = screen.getByText("Dark").closest("button");
    expect(darkButton?.getAttribute("aria-pressed")).toBe("true");
    expect(document.documentElement.classList.contains("dark")).toBe(true);
  });

  it("selects light theme when Light button is clicked", () => {
    renderWithTheme(<ThemeSelector />);

    act(() => {
      fireEvent.click(screen.getByText("Light"));
    });

    const lightButton = screen.getByText("Light").closest("button");
    expect(lightButton?.getAttribute("aria-pressed")).toBe("true");
    expect(document.documentElement.classList.contains("dark")).toBe(false);
  });
});
