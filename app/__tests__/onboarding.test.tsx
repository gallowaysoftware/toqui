import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { createElement } from "react";
import en from "@/messages/en.json";

// --- Track router calls ---
const mockReplace = vi.fn();
const mockPush = vi.fn();

// Mock createTrip mutation
const mockMutateAsync = vi.fn();
vi.mock("@/lib/hooks/useTrips", () => ({
  useCreateTrip: () => ({
    mutateAsync: mockMutateAsync,
    isPending: false,
    isError: false,
    error: null,
  }),
}));

// Mock expo-web-browser (in-app browser for Terms / Privacy links)
const mockOpenBrowserAsync = vi.fn();
vi.mock("expo-web-browser", () => ({
  openBrowserAsync: (...args: unknown[]) => mockOpenBrowserAsync(...args),
  maybeCompleteAuthSession: vi.fn(),
}));

// Mock react-native with web platform
vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Platform: { OS: "web" },
    useColorScheme: () => "light",
    Dimensions: {
      get: () => ({ width: 375, height: 812 }),
      addEventListener: () => ({ remove: vi.fn() }),
    },
  };
});

// Mock expo-router
vi.mock("expo-router", () => ({
  useRouter: () => ({
    replace: mockReplace,
    push: mockPush,
    back: vi.fn(),
  }),
  useLocalSearchParams: () => ({}),
}));

// Mock react-i18next to use actual English translations
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const parts = key.split(".");
      let val: unknown = en;
      for (const p of parts) {
        val = (val as Record<string, unknown>)?.[p];
      }
      if (typeof val !== "string") return key;
      if (params) {
        return Object.entries(params).reduce(
          (s, [k, v]) => s.replace(`{{${k}}}`, String(v)),
          val,
        );
      }
      return val;
    },
    i18n: { language: "en" },
  }),
}));

// Mock lucide-react-native
vi.mock("lucide-react-native", () => {
  const icon = (props: Record<string, unknown>) => createElement("svg", props);
  return {
    Compass: icon,
    Map: icon,
    Briefcase: icon,
    Calendar: icon,
    Globe: icon,
    MapPinned: icon,
    Plus: icon,
    MapPin: icon,
    ChevronRight: icon,
    Crown: icon,
    Plane: icon,
    AlertCircle: icon,
    RefreshCw: icon,
    Users: icon,
  };
});

import OnboardingScreen from "@/app/onboarding";
import { ThemeProvider } from "@/lib/theme";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderOnboarding() {
  return render(
    createElement(ThemeProvider, null, createElement(OnboardingScreen)),
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => {
  sessionStorage.clear();
  mockReplace.mockClear();
  mockPush.mockClear();
  mockMutateAsync.mockClear();
  mockOpenBrowserAsync.mockClear();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("OnboardingScreen", () => {
  it("renders the welcome headline", async () => {
    await act(async () => {
      renderOnboarding();
    });

    expect(screen.getByText("Welcome to Toqui")).toBeInTheDocument();
  });

  it("renders the value proposition", async () => {
    await act(async () => {
      renderOnboarding();
    });

    expect(
      screen.getByText("Chat with AI travel experts to plan your perfect trip in minutes."),
    ).toBeInTheDocument();
  });

  it("renders a destination input field", async () => {
    await act(async () => {
      renderOnboarding();
    });

    expect(screen.getByTestId("onboarding-destination-input")).toBeInTheDocument();
  });

  it("renders Start Planning and Browse trip ideas buttons", async () => {
    await act(async () => {
      renderOnboarding();
    });

    expect(screen.getByTestId("onboarding-start-planning")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-browse-ideas")).toBeInTheDocument();
  });

  it("renders the implicit-accept terms notice with inline links", async () => {
    await act(async () => {
      renderOnboarding();
    });

    expect(screen.getByTestId("onboarding-terms-notice")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-terms-link")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-privacy-link")).toBeInTheDocument();
    expect(screen.getByText("Terms of Service")).toBeInTheDocument();
    expect(screen.getByText("Privacy Policy")).toBeInTheDocument();
  });

  it("does NOT render a separate terms-acceptance checkbox", async () => {
    await act(async () => {
      renderOnboarding();
    });

    expect(screen.queryByTestId("onboarding-terms-checkbox")).not.toBeInTheDocument();
  });

  it("Start Planning is disabled when no destination is entered (no checkbox required)", async () => {
    await act(async () => {
      renderOnboarding();
    });

    const startButton = screen.getByTestId("onboarding-start-planning");
    expect(startButton).toBeDisabled();
  });

  it("Start Planning creates trip and navigates to chat (implicit terms acceptance)", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "trip-123" });

    await act(async () => {
      renderOnboarding();
    });

    const input = screen.getByTestId("onboarding-destination-input");
    await act(async () => {
      fireEvent.change(input, { target: { value: "Tokyo" } });
    });

    const startButton = screen.getByTestId("onboarding-start-planning");
    await act(async () => {
      fireEvent.click(startButton);
    });

    expect(sessionStorage.getItem("toqui_onboarding_complete")).toBe("true");
    expect(mockMutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({ title: "Tokyo" }),
    );
    expect(mockReplace).toHaveBeenCalledWith("/trips/trip-123/chat");
  });

  it("Browse trip ideas sets completion flag and navigates to home (implicit terms acceptance)", async () => {
    await act(async () => {
      renderOnboarding();
    });

    const browseButton = screen.getByTestId("onboarding-browse-ideas");
    await act(async () => {
      fireEvent.click(browseButton);
    });

    expect(sessionStorage.getItem("toqui_onboarding_complete")).toBe("true");
    expect(mockReplace).toHaveBeenCalledWith("/(tabs)");
  });

  it("Terms link opens the in-app browser (not Safari via Linking)", async () => {
    await act(async () => {
      renderOnboarding();
    });

    const termsLink = screen.getByTestId("onboarding-terms-link");
    await act(async () => {
      fireEvent.click(termsLink);
    });

    expect(mockOpenBrowserAsync).toHaveBeenCalledWith("https://toqui.travel/terms");
  });

  it("Privacy link opens the in-app browser (not Safari via Linking)", async () => {
    await act(async () => {
      renderOnboarding();
    });

    const privacyLink = screen.getByTestId("onboarding-privacy-link");
    await act(async () => {
      fireEvent.click(privacyLink);
    });

    expect(mockOpenBrowserAsync).toHaveBeenCalledWith("https://toqui.travel/privacy");
  });
});
