import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import WaitlistPage from "./page";

// Track navigation calls
const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    replace: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    refresh: vi.fn(),
    prefetch: vi.fn(),
  }),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("next-intl", () => ({
  useTranslations: (namespace?: string) => {
    return (key: string) => (namespace ? `${namespace}.${key}` : key);
  },
}));

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

describe("WaitlistPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the email join form by default", () => {
    render(<WaitlistPage />, { wrapper: createWrapper() });

    expect(screen.getByText("waitlist.title")).toBeInTheDocument();
    expect(screen.getByText("waitlist.description")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("waitlist.emailPlaceholder")).toBeInTheDocument();
    expect(screen.getByText("waitlist.joinButton")).toBeInTheDocument();
  });

  it("renders the invite code link on the join form", () => {
    render(<WaitlistPage />, { wrapper: createWrapper() });

    expect(screen.getByText("waitlist.haveInvite")).toBeInTheDocument();
  });

  it("shows the invite code form when clicking 'have invite code'", () => {
    render(<WaitlistPage />, { wrapper: createWrapper() });

    fireEvent.click(screen.getByText("waitlist.haveInvite"));

    expect(screen.getByText("waitlist.inviteTitle")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("waitlist.inviteCodePlaceholder")).toBeInTheDocument();
    expect(screen.getByText("waitlist.redeemButton")).toBeInTheDocument();
  });

  it("shows position after successfully joining the waitlist", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ position: 42, invite_code: "TOQUI-TEST" }),
    });

    render(<WaitlistPage />, { wrapper: createWrapper() });

    const emailInput = screen.getByPlaceholderText("waitlist.emailPlaceholder");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });
    fireEvent.click(screen.getByText("waitlist.joinButton"));

    await waitFor(() => {
      expect(screen.getByText("waitlist.joinedTitle")).toBeInTheDocument();
    });

    expect(screen.getByTestId("waitlist-position")).toHaveTextContent("#42");
    expect(screen.getByText("waitlist.estimatedWait")).toBeInTheDocument();
  });

  it("shows error message when join request fails", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => "Internal server error",
    });

    render(<WaitlistPage />, { wrapper: createWrapper() });

    const emailInput = screen.getByPlaceholderText("waitlist.emailPlaceholder");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });
    fireEvent.click(screen.getByText("waitlist.joinButton"));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
  });

  it("shows loading state while joining", async () => {
    // Create a fetch that doesn't resolve immediately
    let resolveFetch: (value: unknown) => void;
    mockFetch.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveFetch = resolve;
      }),
    );

    render(<WaitlistPage />, { wrapper: createWrapper() });

    const emailInput = screen.getByPlaceholderText("waitlist.emailPlaceholder");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });
    fireEvent.click(screen.getByText("waitlist.joinButton"));

    await waitFor(() => {
      expect(screen.getByText("waitlist.joining")).toBeInTheDocument();
    });

    // Clean up the pending promise
    resolveFetch!({
      ok: true,
      json: async () => ({ position: 1, invite_code: "X" }),
    });
  });

  it("can navigate back from invite view to join view", () => {
    render(<WaitlistPage />, { wrapper: createWrapper() });

    // Go to invite view
    fireEvent.click(screen.getByText("waitlist.haveInvite"));
    expect(screen.getByText("waitlist.inviteTitle")).toBeInTheDocument();

    // Go back
    fireEvent.click(screen.getByText("common.back"));
    expect(screen.getByText("waitlist.title")).toBeInTheDocument();
  });

  it("redirects to login with invite code when submitting", () => {
    // Mock window.location.href setter
    const originalLocation = window.location;
    const mockLocation = { ...originalLocation, href: "" };
    Object.defineProperty(window, "location", {
      value: mockLocation,
      writable: true,
    });

    render(<WaitlistPage />, { wrapper: createWrapper() });

    // Go to invite view
    fireEvent.click(screen.getByText("waitlist.haveInvite"));

    // Fill in invite code
    const codeInput = screen.getByPlaceholderText("waitlist.inviteCodePlaceholder");
    fireEvent.change(codeInput, { target: { value: "TOQUI-ABCD" } });
    fireEvent.click(screen.getByText("waitlist.redeemButton"));

    expect(mockLocation.href).toContain("/auth/google/login?invite_code=TOQUI-ABCD");

    // Restore
    Object.defineProperty(window, "location", {
      value: originalLocation,
      writable: true,
    });
  });

  it("displays app branding", () => {
    render(<WaitlistPage />, { wrapper: createWrapper() });

    expect(screen.getByText("common.appName")).toBeInTheDocument();
    expect(screen.getByText("common.tagline")).toBeInTheDocument();
  });
});
