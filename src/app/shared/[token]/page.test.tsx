import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import SharedTripPage from "./page";
import type { SharedTripResponse } from "@/lib/shared-trip-types";

// Mock next/navigation
const mockToken = "abc123";
vi.mock("next/navigation", () => ({
  useParams: () => ({ token: mockToken }),
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    refresh: vi.fn(),
    prefetch: vi.fn(),
  }),
  usePathname: () => `/shared/${mockToken}`,
  useSearchParams: () => new URLSearchParams(),
}));

// Mock next/link
vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...props
  }: {
    children: React.ReactNode;
    href: string;
    [key: string]: unknown;
  }) => (
    <a href={href} {...(props as React.AnchorHTMLAttributes<HTMLAnchorElement>)}>
      {children}
    </a>
  ),
}));

// Mock lucide-react icons
vi.mock("lucide-react", () => ({
  Calendar: (props: Record<string, unknown>) => <svg data-testid="calendar-icon" {...props} />,
  MapPin: (props: Record<string, unknown>) => <svg data-testid="mappin-icon" {...props} />,
  Utensils: (props: Record<string, unknown>) => <svg data-testid="utensils-icon" {...props} />,
  Landmark: (props: Record<string, unknown>) => <svg data-testid="landmark-icon" {...props} />,
  ShoppingBag: (props: Record<string, unknown>) => <svg data-testid="shopping-icon" {...props} />,
  Compass: (props: Record<string, unknown>) => <svg data-testid="compass-icon" {...props} />,
  Bed: (props: Record<string, unknown>) => <svg data-testid="bed-icon" {...props} />,
  Plane: (props: Record<string, unknown>) => <svg data-testid="plane-icon" {...props} />,
  Clock: (props: Record<string, unknown>) => <svg data-testid="clock-icon" {...props} />,
}));

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

const sampleResponse: SharedTripResponse = {
  trip: {
    title: "Tokyo Adventure",
    description: "Two weeks exploring Japan",
    destination_country: "JP",
    status: "active",
    start_date: "2026-04-01",
    end_date: "2026-04-14",
  },
  itinerary: [
    {
      day_number: 1,
      items: [
        {
          title: "Arrive at Narita Airport",
          type: "transport",
          description: "International flight",
        },
        { title: "Check into hotel", type: "accommodation" },
      ],
    },
    {
      day_number: 2,
      items: [
        {
          title: "Visit Senso-ji Temple",
          type: "sightseeing",
          description: "Oldest temple in Tokyo",
        },
        { title: "Ramen for lunch", type: "food" },
      ],
    },
  ],
};

describe("SharedTripPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders trip info after successful fetch", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => sampleResponse,
    });

    render(<SharedTripPage />);

    await waitFor(() => {
      expect(screen.getByText("Tokyo Adventure")).toBeInTheDocument();
    });

    expect(screen.getByText("Two weeks exploring Japan")).toBeInTheDocument();
    expect(screen.getByText("JP")).toBeInTheDocument();
  });

  it("renders itinerary days and items", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => sampleResponse,
    });

    render(<SharedTripPage />);

    await waitFor(() => {
      expect(screen.getByText("Day 1")).toBeInTheDocument();
    });

    expect(screen.getByText("Day 2")).toBeInTheDocument();
    expect(screen.getByText("Arrive at Narita Airport")).toBeInTheDocument();
    expect(screen.getByText("Visit Senso-ji Temple")).toBeInTheDocument();
    expect(screen.getByText("Ramen for lunch")).toBeInTheDocument();
  });

  it("shows 404 page when trip is not found", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
    });

    render(<SharedTripPage />);

    await waitFor(() => {
      expect(screen.getByText("Trip not found")).toBeInTheDocument();
    });

    expect(screen.getByText(/This shared trip link may have expired/)).toBeInTheDocument();
  });

  it("shows error page on server error", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
    });

    render(<SharedTripPage />);

    await waitFor(() => {
      expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    });
  });

  it("shows loading state initially", () => {
    // Don't resolve the fetch
    mockFetch.mockReturnValueOnce(new Promise(() => {}));

    render(<SharedTripPage />);

    expect(screen.getByRole("status")).toBeInTheDocument();
  });

  it("shows CTA to plan your own trip", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => sampleResponse,
    });

    render(<SharedTripPage />);

    await waitFor(() => {
      expect(screen.getByText("Plan your own trip with Toqui")).toBeInTheDocument();
    });
  });

  it("shows Toqui branding in header", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => sampleResponse,
    });

    render(<SharedTripPage />);

    await waitFor(() => {
      expect(screen.getByText("Toqui")).toBeInTheDocument();
    });

    expect(screen.getByText("Shared Trip")).toBeInTheDocument();
  });

  it("renders empty itinerary message when no days", async () => {
    const emptyResponse: SharedTripResponse = {
      trip: {
        title: "Empty Trip",
        status: "planning",
      },
      itinerary: [],
    };

    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => emptyResponse,
    });

    render(<SharedTripPage />);

    await waitFor(() => {
      expect(screen.getByText("Empty Trip")).toBeInTheDocument();
    });

    expect(screen.getByText("No itinerary has been added to this trip yet.")).toBeInTheDocument();
  });

  it("fetches from the correct API endpoint", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => sampleResponse,
    });

    render(<SharedTripPage />);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(expect.stringContaining(`/shared/${mockToken}`));
    });
  });
});
