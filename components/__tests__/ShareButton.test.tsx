import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { ShareButton } from "../ShareButton";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("lucide-react-native", () => ({
  Share2: () => <span data-testid="share2-icon" />,
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

vi.mock("@/lib/auth", () => ({
  useAuth: () => ({ accessToken: "test-token" }),
}));

const mockAuthFetch = vi.fn();
vi.mock("@/lib/authFetch", () => ({
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

vi.mock("@/lib/config", () => ({
  getConfig: () => ({ apiUrl: "http://localhost:8090" }),
}));

// Mock react-native Share
const mockShare = vi.fn();
vi.mock("react-native", async () => {
  const actual = await vi.importActual("react-native");
  return {
    ...(actual as object),
    Share: { share: (...args: unknown[]) => mockShare(...args) },
    Alert: { alert: vi.fn() },
    Platform: { OS: "web" },
  };
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("ShareButton", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ share_token: "abc123" }),
    });
    mockShare.mockResolvedValue({ action: "sharedAction" });
  });

  it("renders with default label", () => {
    render(<ShareButton tripId="trip-1" tripTitle="Tokyo Adventure" />);
    expect(screen.getByText("Share")).toBeTruthy();
  });

  it("renders with custom label", () => {
    render(
      <ShareButton tripId="trip-1" tripTitle="Tokyo Adventure" label="Share Trip" />,
    );
    expect(screen.getByText("Share Trip")).toBeTruthy();
  });

  it("has correct accessibility label", () => {
    render(<ShareButton tripId="trip-1" tripTitle="Tokyo Adventure" />);
    expect(screen.getByLabelText("Share Tokyo Adventure")).toBeTruthy();
  });

  it("renders the share icon", () => {
    render(<ShareButton tripId="trip-1" tripTitle="Tokyo Adventure" />);
    expect(screen.getByTestId("share2-icon")).toBeTruthy();
  });

  it("calls authFetch to enable sharing on press", async () => {
    render(<ShareButton tripId="trip-1" tripTitle="Tokyo Adventure" />);

    const button = screen.getByRole("button", { name: "Share Tokyo Adventure" });
    fireEvent.click(button);

    await waitFor(() => {
      expect(mockAuthFetch).toHaveBeenCalledWith(
        "http://localhost:8090/api/trips/share",
        "test-token",
        { method: "POST", body: JSON.stringify({ trip_id: "trip-1" }) },
      );
    });
  });

  it("uses Share.share with the share URL", async () => {
    render(<ShareButton tripId="trip-1" tripTitle="Tokyo Adventure" />);

    const button = screen.getByRole("button", { name: "Share Tokyo Adventure" });
    fireEvent.click(button);

    await waitFor(() => {
      expect(mockShare).toHaveBeenCalledWith(
        expect.objectContaining({
          url: "https://app.toqui.travel/shared/abc123",
        }),
      );
    });
  });

  it("includes destination in share message when provided", async () => {
    render(
      <ShareButton
        tripId="trip-1"
        tripTitle="Tokyo Adventure"
        destination="Japan"
      />,
    );

    const button = screen.getByRole("button", { name: "Share Tokyo Adventure" });
    fireEvent.click(button);

    await waitFor(() => {
      expect(mockShare).toHaveBeenCalledWith(
        expect.objectContaining({
          message: expect.stringContaining("Japan"),
        }),
      );
    });
  });
});
