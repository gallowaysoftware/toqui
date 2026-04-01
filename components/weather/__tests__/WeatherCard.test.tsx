import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, act, fireEvent } from "@testing-library/react";
import { createElement } from "react";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

vi.mock("react-native", async () => {
  const actual =
    await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Platform: { OS: "web" },
    useColorScheme: () => "light",
  };
});

vi.mock("lucide-react-native", () => ({
  ChevronDown: (props: Record<string, unknown>) =>
    createElement("span", { "data-testid": "chevron-down", ...props }),
  ChevronUp: (props: Record<string, unknown>) =>
    createElement("span", { "data-testid": "chevron-up", ...props }),
  Info: (props: Record<string, unknown>) =>
    createElement("span", { "data-testid": "info-icon", ...props }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        "weather.title": "Weather forecast",
        "weather.climateNote": "Based on historical climate averages",
      };
      return translations[key] ?? key;
    },
  }),
}));

const mockGetItem = vi.fn((_key: string) => Promise.resolve(null as string | null));
const mockSetItem = vi.fn((_key: string, _value: string) => Promise.resolve());

vi.mock("@react-native-async-storage/async-storage", () => ({
  default: {
    getItem: (key: string) => mockGetItem(key),
    setItem: (key: string, value: string) => mockSetItem(key, value),
  },
}));

import { WeatherCard } from "@/components/weather/WeatherCard";
import { ThemeProvider } from "@/lib/theme";
import type { WeatherDay } from "@/lib/hooks/useWeather";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const sampleWeather: WeatherDay[] = [
  { date: "2025-07-01", tempHigh: 28, tempLow: 18, precipitation: 0, weatherCode: 0, description: "Clear sky" },
  { date: "2025-07-02", tempHigh: 30, tempLow: 20, precipitation: 5, weatherCode: 61, description: "Slight rain" },
  { date: "2025-07-03", tempHigh: 25, tempLow: 15, precipitation: 0, weatherCode: 2, description: "Partly cloudy" },
];

async function renderCard(props?: { isClimate?: boolean }) {
  let result!: ReturnType<typeof render>;
  await act(async () => {
    result = render(
      createElement(
        ThemeProvider,
        null,
        createElement(WeatherCard, {
          weather: sampleWeather,
          isClimate: props?.isClimate ?? false,
        }),
      ),
    );
  });
  return result;
}

afterEach(() => {
  vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("WeatherCard", () => {
  it("renders the weather title", async () => {
    await renderCard();
    expect(screen.getByText("Weather forecast")).toBeInTheDocument();
  });

  it("renders day labels for each weather day", async () => {
    await renderCard();
    expect(screen.getByText("Tue")).toBeInTheDocument();
    expect(screen.getByText("Wed")).toBeInTheDocument();
    expect(screen.getByText("Thu")).toBeInTheDocument();
  });

  it("renders temperature values in Celsius by default", async () => {
    await renderCard();
    expect(screen.getByText("28\u00B0")).toBeInTheDocument();
    expect(screen.getByText("18\u00B0")).toBeInTheDocument();
  });

  it("renders weather emoji for each day", async () => {
    await renderCard();
    // Clear sky sun emoji
    expect(screen.getByText("\u2600\uFE0F")).toBeInTheDocument();
  });

  it("toggles temperature unit on button press", async () => {
    await renderCard();
    const toggle = screen.getByText("\u00B0C");
    await act(async () => {
      fireEvent.click(toggle);
    });
    // After toggle, should show F and convert 28C → 82F
    expect(screen.getByText("\u00B0F")).toBeInTheDocument();
    expect(screen.getByText("82\u00B0")).toBeInTheDocument();
  });

  it("shows info icon when isClimate is true", async () => {
    await renderCard({ isClimate: true });
    expect(screen.getByTestId("info-icon")).toBeInTheDocument();
  });

  it("does not show info icon when isClimate is false", async () => {
    await renderCard({ isClimate: false });
    expect(screen.queryByTestId("info-icon")).not.toBeInTheDocument();
  });

  it("shows collapse chevron", async () => {
    await renderCard();
    // Starts expanded, so should show ChevronUp
    expect(screen.getByTestId("chevron-up")).toBeInTheDocument();
  });
});
