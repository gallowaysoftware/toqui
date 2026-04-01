import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { createElement } from "react";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// Mock react-native with web platform
vi.mock("react-native", async () => {
  const actual =
    await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Platform: { OS: "web" },
    useColorScheme: () => "light",
  };
});

// Mock lucide icons
vi.mock("lucide-react-native", () => ({
  WifiOff: (props: Record<string, unknown>) =>
    createElement("span", { "data-testid": "wifi-off-icon", ...props }),
  X: (props: Record<string, unknown>) =>
    createElement("span", { "data-testid": "x-icon", ...props }),
}));

// Mutable network status for controlling the hook in tests
let mockIsConnected = true;

vi.mock("@/lib/hooks/useNetworkStatus", () => ({
  useNetworkStatus: () => ({
    isConnected: mockIsConnected,
    isInternetReachable: mockIsConnected,
  }),
}));

import { OfflineBanner } from "@/components/OfflineBanner";
import { ThemeProvider } from "@/lib/theme";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

async function renderBanner() {
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      createElement(ThemeProvider, null, createElement(OfflineBanner)),
    );
  });
  return result;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => {
  mockIsConnected = true;
  localStorage.clear();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("OfflineBanner", () => {
  it("renders the banner element", async () => {
    await renderBanner();
    expect(screen.getByTestId("offline-banner")).toBeInTheDocument();
  });

  it("contains the offline message text when disconnected", async () => {
    mockIsConnected = false;
    await renderBanner();
    expect(screen.getByText(/You're offline/)).toBeInTheDocument();
  });

  it("renders a WifiOff icon", async () => {
    mockIsConnected = false;
    await renderBanner();
    expect(screen.getByTestId("wifi-off-icon")).toBeInTheDocument();
  });

  it("has accessibility role alert", async () => {
    await renderBanner();
    const banner = screen.getByTestId("offline-banner");
    expect(banner).toHaveAttribute("role", "alert");
  });

  it("renders a dismiss button", async () => {
    mockIsConnected = false;
    await renderBanner();
    expect(screen.getByTestId("offline-banner-dismiss")).toBeInTheDocument();
  });

  it("has correct accessibility label on dismiss button", async () => {
    mockIsConnected = false;
    await renderBanner();
    const dismiss = screen.getByTestId("offline-banner-dismiss");
    expect(dismiss).toHaveAttribute("aria-label", "Dismiss offline banner");
  });

  it("banner is present in DOM regardless of connection state", async () => {
    mockIsConnected = false;
    await renderBanner();
    expect(screen.getByTestId("offline-banner")).toBeInTheDocument();
    expect(screen.getByText(/You're offline/)).toBeInTheDocument();
  });

  it("uses amber background color for light mode", async () => {
    mockIsConnected = false;
    await renderBanner();
    const banner = screen.getByTestId("offline-banner");
    expect(banner).toHaveStyle({ backgroundColor: "#fbbf24" });
  });
});
