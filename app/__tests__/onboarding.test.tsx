import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { createElement } from "react";
import en from "@/messages/en.json";

// --- Track router calls ---
const mockReplace = vi.fn();
const mockPush = vi.fn();

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
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("OnboardingScreen", () => {
  it("renders the first slide with headline and subtext", async () => {
    await act(async () => {
      renderOnboarding();
    });

    expect(screen.getByText("AI-powered trip planning")).toBeInTheDocument();
    expect(
      screen.getByText("Chat with expert personas who know your destination inside and out"),
    ).toBeInTheDocument();
  });

  it("renders all three slides", async () => {
    await act(async () => {
      renderOnboarding();
    });

    expect(screen.getByTestId("onboarding-slide-0")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-slide-1")).toBeInTheDocument();
    expect(screen.getByTestId("onboarding-slide-2")).toBeInTheDocument();
  });

  it("renders dot indicators", async () => {
    await act(async () => {
      renderOnboarding();
    });

    const dotsContainer = screen.getByLabelText("Page 1 of 3");
    expect(dotsContainer).toBeInTheDocument();
  });

  it("does not show CTA buttons on initial slide (not the last)", async () => {
    await act(async () => {
      renderOnboarding();
    });

    expect(screen.queryByTestId("onboarding-start-planning")).not.toBeInTheDocument();
    expect(screen.queryByTestId("onboarding-explore-first")).not.toBeInTheDocument();
  });

  it("'Start Planning' button sets completion flag and navigates to /trips/new", async () => {
    await act(async () => {
      renderOnboarding();
    });

    // In jsdom, scroll events on react-native-web ScrollView don't propagate
    // nativeEvent.contentOffset. Instead, simulate the DOM-level scroll event
    // which react-native-web translates.
    const scrollView = screen.getByTestId("onboarding-scroll");
    Object.defineProperty(scrollView, "scrollLeft", { value: 375 * 2, writable: true });
    Object.defineProperty(scrollView, "scrollWidth", { value: 375 * 3, writable: true });
    Object.defineProperty(scrollView, "clientWidth", { value: 375, writable: true });
    await act(async () => {
      fireEvent.scroll(scrollView);
    });

    const startButton = screen.getByTestId("onboarding-start-planning");
    expect(startButton).toBeInTheDocument();

    await act(async () => {
      fireEvent.click(startButton);
    });

    expect(sessionStorage.getItem("toqui_onboarding_complete")).toBe("true");
    expect(mockReplace).toHaveBeenCalledWith("/trips/new");
  });

  it("'Explore First' button sets completion flag and navigates to home", async () => {
    await act(async () => {
      renderOnboarding();
    });

    // Simulate scroll to last slide
    const scrollView = screen.getByTestId("onboarding-scroll");
    Object.defineProperty(scrollView, "scrollLeft", { value: 375 * 2, writable: true });
    Object.defineProperty(scrollView, "scrollWidth", { value: 375 * 3, writable: true });
    Object.defineProperty(scrollView, "clientWidth", { value: 375, writable: true });
    await act(async () => {
      fireEvent.scroll(scrollView);
    });

    const exploreButton = screen.getByTestId("onboarding-explore-first");

    await act(async () => {
      fireEvent.click(exploreButton);
    });

    expect(sessionStorage.getItem("toqui_onboarding_complete")).toBe("true");
    expect(mockReplace).toHaveBeenCalledWith("/(tabs)");
  });

  it("shows all three slide headlines across the swiper", async () => {
    await act(async () => {
      renderOnboarding();
    });

    expect(screen.getByText("AI-powered trip planning")).toBeInTheDocument();
    expect(screen.getByText("Everything in one place")).toBeInTheDocument();
    expect(screen.getByText("Your first trip awaits")).toBeInTheDocument();
  });
});
