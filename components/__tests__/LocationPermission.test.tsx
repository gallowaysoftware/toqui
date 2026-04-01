import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { LocationPermission } from "../LocationPermission";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("lucide-react-native", () => ({
  MapPin: () => <span data-testid="mappin-icon" />,
  X: () => <span data-testid="x-icon" />,
}));

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      surface: "#fff",
      surfaceSecondary: "#f5f5f5",
      surfaceTertiary: "#f0f0f0",
      textPrimary: "#333",
      textSecondary: "#666",
      textTertiary: "#999",
      accent: "#BF4028",
      accentSoft: "#fef3f0",
      infoBg: "#e8f4fd",
      infoBorder: "#b3d9f2",
      success: "#16a34a",
      successBg: "#f0fdf4",
      warning: "#ca8a04",
      warningBg: "#fefce8",
      warningBorder: "#fef08a",
    },
  }),
}));

const mockGetItem = vi.fn();
const mockSetItem = vi.fn();

vi.mock("@react-native-async-storage/async-storage", () => ({
  default: {
    getItem: (...args: unknown[]) => mockGetItem(...args),
    setItem: (...args: unknown[]) => mockSetItem(...args),
  },
}));

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("LocationPermission", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetItem.mockResolvedValue(null);
    mockSetItem.mockResolvedValue(undefined);
  });

  it("renders the banner when not previously dismissed", async () => {
    const onEnable = vi.fn();
    render(<LocationPermission onEnable={onEnable} />);

    await waitFor(() => {
      expect(screen.getByText("Enable location")).toBeInTheDocument();
    });
    expect(
      screen.getByText("Share your location to get personalized nearby recommendations"),
    ).toBeInTheDocument();
  });

  it("calls onEnable when the Enable button is pressed", async () => {
    const onEnable = vi.fn();
    render(<LocationPermission onEnable={onEnable} />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Enable location" })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Enable location" }));
    expect(onEnable).toHaveBeenCalledTimes(1);
  });

  it("dismisses when the Not now button is pressed", async () => {
    const onEnable = vi.fn();
    render(<LocationPermission onEnable={onEnable} />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Not now" })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Not now" }));

    expect(mockSetItem).toHaveBeenCalledWith(
      "location_permission_dismissed_at",
      expect.any(String),
    );

    // Banner should disappear
    await waitFor(() => {
      expect(screen.queryByText("Enable location")).not.toBeInTheDocument();
    });
  });

  it("does not render if recently dismissed (within 24h)", async () => {
    // Dismissed 1 hour ago
    mockGetItem.mockResolvedValue(String(Date.now() - 60 * 60 * 1000));

    const onEnable = vi.fn();
    render(<LocationPermission onEnable={onEnable} />);

    // Give the async effect time to resolve
    await new Promise((r) => setTimeout(r, 50));

    expect(screen.queryByText("Enable location")).not.toBeInTheDocument();
  });

  it("renders if dismissal has expired (over 24h ago)", async () => {
    // Dismissed 25 hours ago
    mockGetItem.mockResolvedValue(String(Date.now() - 25 * 60 * 60 * 1000));

    const onEnable = vi.fn();
    render(<LocationPermission onEnable={onEnable} />);

    await waitFor(() => {
      expect(screen.getByText("Enable location")).toBeInTheDocument();
    });
  });
});
