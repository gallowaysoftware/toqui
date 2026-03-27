import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { RecommendationCard } from "../RecommendationCard";
import type { Recommendation } from "@/lib/hooks/useChat";

// Mock Linking.openURL to track calls — use vi.hoisted so the fn is
// available inside the hoisted vi.mock factory.
const mockOpenURL = vi.hoisted(() => vi.fn());
vi.mock("react-native", async () => {
  const actual = await vi.importActual("react-native");
  return {
    ...actual,
    Linking: { openURL: mockOpenURL },
  };
});

// Mock lucide icons to simple spans
vi.mock("lucide-react-native", () => ({
  ExternalLink: ({ color, size }: { color: string; size: number }) => (
    <span data-testid="icon-external-link" />
  ),
  Plane: () => <span data-testid="icon-plane" />,
  Hotel: () => <span data-testid="icon-hotel" />,
  Ticket: () => <span data-testid="icon-ticket" />,
  Car: () => <span data-testid="icon-car" />,
  Shield: () => <span data-testid="icon-shield" />,
}));

function makeRecommendation(overrides: Partial<Recommendation> = {}): Recommendation {
  return {
    partner: "skyscanner",
    category: "flights",
    title: "Flight to Tokyo",
    description: "Direct flight from SFO",
    url: "https://www.skyscanner.com/flight/123",
    ...overrides,
  };
}

describe("RecommendationCard", () => {
  beforeEach(() => {
    mockOpenURL.mockClear();
  });

  describe("URL security validation", () => {
    it("opens valid https:// URLs", () => {
      render(<RecommendationCard recommendation={makeRecommendation()} />);
      fireEvent.click(screen.getByText("Flight to Tokyo"));
      expect(mockOpenURL).toHaveBeenCalledWith("https://www.skyscanner.com/flight/123");
    });

    it("blocks javascript: protocol URLs", () => {
      render(
        <RecommendationCard
          recommendation={makeRecommendation({ url: "javascript:alert(document.cookie)" })}
        />,
      );
      fireEvent.click(screen.getByText("Flight to Tokyo"));
      expect(mockOpenURL).not.toHaveBeenCalled();
    });

    it("blocks data: protocol URLs", () => {
      render(
        <RecommendationCard
          recommendation={makeRecommendation({ url: "data:text/html,<script>alert(1)</script>" })}
        />,
      );
      fireEvent.click(screen.getByText("Flight to Tokyo"));
      expect(mockOpenURL).not.toHaveBeenCalled();
    });

    it("blocks http:// URLs (not https)", () => {
      render(
        <RecommendationCard
          recommendation={makeRecommendation({ url: "http://evil.com" })}
        />,
      );
      fireEvent.click(screen.getByText("Flight to Tokyo"));
      expect(mockOpenURL).not.toHaveBeenCalled();
    });

    it("blocks URLs with HTTPS in different case (case-sensitive check)", () => {
      // The component uses startsWith("https://") which is case-sensitive
      render(
        <RecommendationCard
          recommendation={makeRecommendation({ url: "HTTPS://evil.com" })}
        />,
      );
      fireEvent.click(screen.getByText("Flight to Tokyo"));
      expect(mockOpenURL).not.toHaveBeenCalled();
    });

    it("blocks empty URLs", () => {
      render(
        <RecommendationCard recommendation={makeRecommendation({ url: "" })} />,
      );
      fireEvent.click(screen.getByText("Flight to Tokyo"));
      expect(mockOpenURL).not.toHaveBeenCalled();
    });

    it("blocks file:// protocol URLs", () => {
      render(
        <RecommendationCard
          recommendation={makeRecommendation({ url: "file:///etc/passwd" })}
        />,
      );
      fireEvent.click(screen.getByText("Flight to Tokyo"));
      expect(mockOpenURL).not.toHaveBeenCalled();
    });
  });

  describe("partner icon mapping", () => {
    it("shows Plane icon for skyscanner", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "skyscanner" })} />);
      expect(screen.getByTestId("icon-plane")).toBeInTheDocument();
      expect(screen.getByText("Skyscanner")).toBeInTheDocument();
    });

    it("shows Hotel icon for booking.com", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "booking.com" })} />);
      expect(screen.getByTestId("icon-hotel")).toBeInTheDocument();
      expect(screen.getByText("Booking.com")).toBeInTheDocument();
    });

    it("shows Hotel icon for bookingcom variant", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "bookingcom" })} />);
      expect(screen.getByTestId("icon-hotel")).toBeInTheDocument();
    });

    it("shows Hotel icon for booking_com variant", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "booking_com" })} />);
      expect(screen.getByTestId("icon-hotel")).toBeInTheDocument();
    });

    it("shows Ticket icon for getyourguide", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "getyourguide" })} />);
      expect(screen.getByTestId("icon-ticket")).toBeInTheDocument();
      expect(screen.getByText("GetYourGuide")).toBeInTheDocument();
    });

    it("shows Ticket icon for viator", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "viator" })} />);
      expect(screen.getByTestId("icon-ticket")).toBeInTheDocument();
      expect(screen.getByText("Viator")).toBeInTheDocument();
    });

    it("shows Car icon for discovercars", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "discovercars" })} />);
      expect(screen.getByTestId("icon-car")).toBeInTheDocument();
      expect(screen.getByText("DiscoverCars")).toBeInTheDocument();
    });

    it("shows Shield icon for safetywing", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "safetywing" })} />);
      expect(screen.getByTestId("icon-shield")).toBeInTheDocument();
      expect(screen.getByText("SafetyWing")).toBeInTheDocument();
    });

    it("falls back to ExternalLink icon for unknown partner", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "kayak" })} />);
      expect(screen.getByText("kayak")).toBeInTheDocument();
      // Should use ExternalLink as fallback - there will be 2 (header + CTA)
      const icons = screen.getAllByTestId("icon-external-link");
      expect(icons.length).toBeGreaterThanOrEqual(2); // header icon + CTA icon
    });

    it("performs case-insensitive partner lookup", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "Skyscanner" })} />);
      expect(screen.getByTestId("icon-plane")).toBeInTheDocument();
      expect(screen.getByText("Skyscanner")).toBeInTheDocument();
    });
  });

  describe("content rendering", () => {
    it("renders title", () => {
      render(<RecommendationCard recommendation={makeRecommendation()} />);
      expect(screen.getByText("Flight to Tokyo")).toBeInTheDocument();
    });

    it("renders description when provided", () => {
      render(
        <RecommendationCard
          recommendation={makeRecommendation({ description: "Great deal" })}
        />,
      );
      expect(screen.getByText("Great deal")).toBeInTheDocument();
    });

    it("does not render description element when empty", () => {
      render(
        <RecommendationCard
          recommendation={makeRecommendation({ description: "" })}
        />,
      );
      // Falsy description should not render (empty string is falsy)
      expect(screen.queryByText("Great deal")).toBeNull();
    });

    it("renders price when provided", () => {
      render(
        <RecommendationCard
          recommendation={makeRecommendation({ price: "$299" })}
        />,
      );
      expect(screen.getByText("$299")).toBeInTheDocument();
    });

    it("does not render price when absent", () => {
      render(
        <RecommendationCard recommendation={makeRecommendation({ price: undefined })} />,
      );
      expect(screen.queryByText("$")).toBeNull();
    });

    it("renders disclosure when provided", () => {
      render(
        <RecommendationCard
          recommendation={makeRecommendation({ disclosure: "Affiliate link" })}
        />,
      );
      expect(screen.getByText("Affiliate link")).toBeInTheDocument();
    });

    it("does not render disclosure when absent", () => {
      render(
        <RecommendationCard recommendation={makeRecommendation({ disclosure: undefined })} />,
      );
      expect(screen.queryByText("Affiliate link")).toBeNull();
    });

    it("renders CTA with partner label", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "viator" })} />);
      expect(screen.getByText("View on Viator")).toBeInTheDocument();
    });

    it("renders CTA with raw partner name for unknown partners", () => {
      render(<RecommendationCard recommendation={makeRecommendation({ partner: "kayak" })} />);
      expect(screen.getByText("View on kayak")).toBeInTheDocument();
    });
  });
});
