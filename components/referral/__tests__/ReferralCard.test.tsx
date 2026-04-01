import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import ReferralCard from "../ReferralCard";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("lucide-react-native", () => ({
  Gift: () => <span data-testid="gift-icon" />,
  Copy: () => <span data-testid="copy-icon" />,
  Share2: () => <span data-testid="share2-icon" />,
  Users: () => <span data-testid="users-icon" />,
}));

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      surface: "#ffffff",
      surfaceSecondary: "#f9fafb",
      textPrimary: "#111827",
      textSecondary: "#4b5563",
      textTertiary: "#5f6673",
      accent: "#e8654a",
      accentSoft: "#fef2f0",
      border: "#e5e7eb",
    },
  }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (opts?.count !== undefined) return `${key}:${opts.count}`;
      return key;
    },
  }),
}));

const mockSetStringAsync = vi.fn();
vi.mock("expo-clipboard", () => ({
  setStringAsync: (...args: unknown[]) => mockSetStringAsync(...args),
}));

const mockReferral = {
  code: "ABC123" as string | null,
  successfulReferrals: 5,
  rewardsEarned: 2,
  isLoading: false,
  error: null as string | null,
};

vi.mock("@/lib/hooks/useReferral", () => ({
  useReferral: () => mockReferral,
}));

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("ReferralCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockReferral.code = "ABC123";
    mockReferral.successfulReferrals = 5;
    mockReferral.rewardsEarned = 2;
    mockReferral.isLoading = false;
    mockReferral.error = null;
  });

  it("renders the referral code", () => {
    render(<ReferralCard />);
    expect(screen.getByText("ABC123")).toBeInTheDocument();
  });

  it("renders description and code label", () => {
    render(<ReferralCard />);
    expect(screen.getByText("referral.description")).toBeInTheDocument();
    expect(screen.getByText("referral.yourCode")).toBeInTheDocument();
  });

  it("renders copy and share buttons", () => {
    render(<ReferralCard />);
    expect(screen.getByText("referral.copyLink")).toBeInTheDocument();
    expect(screen.getByText("referral.share")).toBeInTheDocument();
  });

  it("renders stats with correct counts", () => {
    render(<ReferralCard />);
    expect(screen.getByText("referral.friendsInvited:5")).toBeInTheDocument();
    expect(screen.getByText("referral.rewardsEarned:2")).toBeInTheDocument();
  });

  it("copies share link to clipboard when copy button is pressed", async () => {
    mockSetStringAsync.mockResolvedValue(undefined);
    render(<ReferralCard />);
    const copyBtn = screen.getByText("referral.copyLink");
    fireEvent.click(copyBtn);
    expect(mockSetStringAsync).toHaveBeenCalledWith("https://toqui.travel?ref=ABC123");
  });

  it("shows loading indicator when isLoading is true", () => {
    mockReferral.isLoading = true;
    const { container } = render(<ReferralCard />);
    // Should not show the code when loading
    expect(screen.queryByText("ABC123")).toBeNull();
    // ActivityIndicator renders a div with role="progressbar" in react-native-web
    expect(container.querySelector('[role="progressbar"]')).not.toBeNull();
  });

  it("renders nothing when error is set", () => {
    mockReferral.error = "Failed to load";
    const { container } = render(<ReferralCard />);
    expect(container.innerHTML).toBe("");
  });

  it("renders nothing when code is null", () => {
    mockReferral.code = null;
    const { container } = render(<ReferralCard />);
    expect(container.innerHTML).toBe("");
  });
});
