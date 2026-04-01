import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import ForwardingCard from "../ForwardingCard";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("lucide-react-native", () => ({
  Mail: () => <span data-testid="mail-icon" />,
  Copy: () => <span data-testid="copy-icon" />,
  Check: () => <span data-testid="check-icon" />,
  ChevronDown: () => <span data-testid="chevron-down-icon" />,
  ChevronUp: () => <span data-testid="chevron-up-icon" />,
  X: () => <span data-testid="x-icon" />,
}));

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      surface: "#ffffff",
      surfaceSecondary: "#f9fafb",
      textPrimary: "#111827",
      textSecondary: "#4b5563",
      textTertiary: "#5f6673",
      accent: "#BF4028",
      accentSoft: "#fef2f0",
      border: "#e5e7eb",
      info: "#1e40af",
      infoBg: "#eff6ff",
      infoBorder: "#bfdbfe",
      success: "#16a34a",
    },
  }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const mockSetStringAsync = vi.fn();
vi.mock("expo-clipboard", () => ({
  setStringAsync: (...args: unknown[]) => mockSetStringAsync(...args),
}));

// Mock navigator.clipboard for web platform (tests run in jsdom which is web)
const mockWriteText = vi.fn().mockResolvedValue(undefined);
Object.defineProperty(globalThis, "navigator", {
  value: {
    ...globalThis.navigator,
    clipboard: { writeText: mockWriteText },
  },
  writable: true,
  configurable: true,
});

// Mock localStorage for dismissed state
const localStorageMock: Record<string, string> = {};
Object.defineProperty(globalThis, "localStorage", {
  value: {
    getItem: (key: string) => localStorageMock[key] ?? null,
    setItem: (key: string, value: string) => { localStorageMock[key] = value; },
    removeItem: (key: string) => { delete localStorageMock[key]; },
  },
  writable: true,
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("ForwardingCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Clear dismissed state
    for (const key of Object.keys(localStorageMock)) {
      delete localStorageMock[key];
    }
  });

  it("renders the forwarding email address", async () => {
    render(<ForwardingCard tripId="trip-123" />);
    await waitFor(() => {
      expect(screen.getByText("bookings@mail.toqui.travel")).toBeInTheDocument();
    });
  });

  it("renders the title and description", async () => {
    render(<ForwardingCard tripId="trip-123" />);
    await waitFor(() => {
      expect(screen.getByText("bookings.forwarding.title")).toBeInTheDocument();
      expect(screen.getByText("bookings.forwarding.description")).toBeInTheDocument();
    });
  });

  it("copies email to clipboard when copy button is pressed", async () => {
    render(<ForwardingCard tripId="trip-123" />);
    await waitFor(() => {
      expect(screen.getByText("bookings.forwarding.copy")).toBeInTheDocument();
    });
    const copyBtn = screen.getByText("bookings.forwarding.copy");
    fireEvent.click(copyBtn);
    // Web platform uses navigator.clipboard.writeText
    await waitFor(() => {
      expect(mockWriteText).toHaveBeenCalledWith("bookings@mail.toqui.travel");
    });
  });

  it("shows 'Copied!' feedback after copy", async () => {
    render(<ForwardingCard tripId="trip-123" />);
    await waitFor(() => {
      expect(screen.getByText("bookings.forwarding.copy")).toBeInTheDocument();
    });
    const copyBtn = screen.getByText("bookings.forwarding.copy");
    fireEvent.click(copyBtn);
    await waitFor(() => {
      expect(screen.getByText("referral.copied")).toBeInTheDocument();
    });
  });

  it("shows how it works steps when toggle is pressed", async () => {
    render(<ForwardingCard tripId="trip-123" />);
    await waitFor(() => {
      expect(screen.getByText("bookings.forwarding.howItWorks")).toBeInTheDocument();
    });
    // Steps should not be visible initially
    expect(screen.queryByText("bookings.forwarding.step1")).toBeNull();

    fireEvent.click(screen.getByText("bookings.forwarding.howItWorks"));
    expect(screen.getByText("bookings.forwarding.step1")).toBeInTheDocument();
    expect(screen.getByText("bookings.forwarding.step2")).toBeInTheDocument();
    expect(screen.getByText("bookings.forwarding.step3")).toBeInTheDocument();
  });

  it("hides the card when dismiss button is pressed", async () => {
    render(<ForwardingCard tripId="trip-456" />);
    await waitFor(() => {
      expect(screen.getByText("bookings@mail.toqui.travel")).toBeInTheDocument();
    });

    // Find the dismiss button (the X icon's parent pressable)
    const dismissBtn = screen.getByLabelText("common.cancel");
    fireEvent.click(dismissBtn);

    expect(screen.queryByText("bookings@mail.toqui.travel")).toBeNull();
  });

  it("persists dismissed state to localStorage", async () => {
    render(<ForwardingCard tripId="trip-789" />);
    await waitFor(() => {
      expect(screen.getByText("bookings@mail.toqui.travel")).toBeInTheDocument();
    });

    const dismissBtn = screen.getByLabelText("common.cancel");
    fireEvent.click(dismissBtn);

    expect(localStorageMock["toqui_forwarding_dismissed_trip-789"]).toBe("1");
  });

  it("does not render when previously dismissed", async () => {
    localStorageMock["toqui_forwarding_dismissed_trip-abc"] = "1";
    const { container } = render(<ForwardingCard tripId="trip-abc" />);
    // Wait a tick for the async loadDismissed to resolve
    await waitFor(() => {
      expect(container.textContent).toBe("");
    });
  });
});
