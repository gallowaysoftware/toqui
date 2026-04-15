import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { ProUpgrade } from "../ProUpgrade";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("lucide-react-native", () => ({
  CheckCircle: () => <span data-testid="check-circle-icon" />,
  Star: () => <span data-testid="star-icon" />,
  Mail: () => <span data-testid="mail-icon" />,
  BookOpen: () => <span data-testid="book-open-icon" />,
  ChevronRight: () => <span data-testid="chevron-right-icon" />,
  X: () => <span data-testid="x-icon" />,
  ArrowRight: () => <span data-testid="arrow-right-icon" />,
}));

vi.mock("expo-router", () => ({
  useRouter: () => ({
    push: vi.fn(),
    back: vi.fn(),
    replace: vi.fn(),
  }),
}));

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      surface: "#ffffff",
      border: "#e5e7eb",
      textPrimary: "#111827",
      textSecondary: "#4b5563",
      textTertiary: "#5f6673",
      accent: "#e8654a",
      error: "#dc2626",
      success: "#16a34a",
      successBg: "#f0fdf4",
    },
  }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const mockInitCheckout = vi.fn();
const mockCheckStatus = vi.fn();
const mockCheckout = {
  initCheckout: mockInitCheckout,
  checkStatus: mockCheckStatus,
  isLoading: false,
  error: null as string | null,
};

vi.mock("@/lib/hooks/useCheckout", () => ({
  useCheckout: () => mockCheckout,
}));

vi.mock("@/lib/hooks/useSubscription", () => ({
  useSubscription: () => ({
    subscription: null,
    isLoading: false,
    error: null,
    subscribe: vi.fn(),
    cancel: vi.fn(),
    manageSubscription: vi.fn(),
  }),
}));

const mockGetFeatureFlag = vi.fn();
vi.mock("@/lib/analytics", () => ({
  useAnalytics: () => ({
    track: vi.fn(),
    getFeatureFlag: mockGetFeatureFlag,
  }),
}));

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("ProUpgrade", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCheckout.isLoading = false;
    mockCheckout.error = null;
    // Default: status check resolves to not unlocked
    mockCheckStatus.mockResolvedValue({ unlocked: false });
    // Default: no feature flag set (falls back to $19)
    mockGetFeatureFlag.mockReturnValue(undefined);
  });

  it("shows loading indicator while checking status", () => {
    // Make checkStatus never resolve to keep it in checking state
    mockCheckStatus.mockReturnValue(new Promise(() => {}));
    const { container } = render(<ProUpgrade tripId="t1" />);
    expect(container.querySelector('[role="progressbar"]')).not.toBeNull();
  });

  it("renders upgrade UI when not unlocked", async () => {
    mockCheckStatus.mockResolvedValue({ unlocked: false });
    render(<ProUpgrade tripId="t1" />);

    await waitFor(() => {
      expect(screen.getByText("checkout.title")).toBeInTheDocument();
    });
    expect(screen.getByText("$19 CAD")).toBeInTheDocument();
    expect(screen.getByText("checkout.priceDescription")).toBeInTheDocument();
  });

  it("renders benefit list", async () => {
    mockCheckStatus.mockResolvedValue({ unlocked: false });
    render(<ProUpgrade tripId="t1" />);

    await waitFor(() => {
      expect(screen.getByText("checkout.benefits.experts")).toBeInTheDocument();
    });
    expect(screen.getByText("checkout.benefits.bookings")).toBeInTheDocument();
    expect(screen.getByText("checkout.benefits.email")).toBeInTheDocument();
  });

  it("shows success view when already unlocked", async () => {
    mockCheckStatus.mockResolvedValue({ unlocked: true });
    render(<ProUpgrade tripId="t1" />);

    await waitFor(() => {
      expect(screen.getByText("checkout.success")).toBeInTheDocument();
    });
    expect(screen.getByText("checkout.successDescription")).toBeInTheDocument();
  });

  it("shows error message when error is set", async () => {
    mockCheckout.error = "Payment failed";
    mockCheckStatus.mockResolvedValue({ unlocked: false });
    render(<ProUpgrade tripId="t1" />);

    await waitFor(() => {
      expect(screen.getByText("checkout.error")).toBeInTheDocument();
    });
  });

  it("shows unlock button", async () => {
    mockCheckStatus.mockResolvedValue({ unlocked: false });
    render(<ProUpgrade tripId="t1" />);

    await waitFor(() => {
      expect(screen.getByText("checkout.unlockButton")).toBeInTheDocument();
    });
  });

  it("displays price from PostHog feature flag variant", async () => {
    mockGetFeatureFlag.mockReturnValue("15");
    mockCheckStatus.mockResolvedValue({ unlocked: false });
    render(<ProUpgrade tripId="t1" />);

    await waitFor(() => {
      expect(screen.getByText("$15 CAD")).toBeInTheDocument();
    });
  });

  it("renders not-unlocked state when status check fails", async () => {
    mockCheckStatus.mockRejectedValue(new Error("Network error"));
    render(<ProUpgrade tripId="t1" />);

    await waitFor(() => {
      expect(screen.getByText("checkout.title")).toBeInTheDocument();
    });
  });

  it("renders compact inline banner when compact prop is set", async () => {
    mockCheckStatus.mockResolvedValue({ unlocked: false });
    const onDismiss = vi.fn();
    render(<ProUpgrade tripId="t1" compact onDismiss={onDismiss} />);

    await waitFor(() => {
      expect(screen.getByText("checkout.unlockInline")).toBeInTheDocument();
    });
    // Should not render the full card elements
    expect(screen.queryByText("checkout.title")).toBeNull();
    expect(screen.queryByText("$19 CAD")).toBeNull();
  });

  it("calls onDismiss when compact banner dismiss is clicked", async () => {
    mockCheckStatus.mockResolvedValue({ unlocked: false });
    const onDismiss = vi.fn();
    render(<ProUpgrade tripId="t1" compact onDismiss={onDismiss} />);

    await waitFor(() => {
      expect(screen.getByText("checkout.unlockInline")).toBeInTheDocument();
    });

    const dismissButton = screen.getByLabelText("common.dismiss");
    fireEvent.click(dismissButton);
    expect(onDismiss).toHaveBeenCalledOnce();
  });
});
