import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act, waitFor } from "@testing-library/react";
import { createElement } from "react";
import en from "@/messages/en.json";

const mockReplace = vi.fn();
const mockBack = vi.fn();

const mockSetServerUrl = vi.fn((_url: string) => Promise.resolve());
const mockNeedsServerSetup = vi.fn(() => true);
const mockLogout = vi.fn(() => Promise.resolve());
let mockAccessToken: string | null = null;

vi.mock("@/lib/config", async () => {
  const actual = await vi.importActual<typeof import("@/lib/config")>("@/lib/config");
  return {
    ...actual,
    setServerUrl: (...args: unknown[]) => mockSetServerUrl(...(args as [string])),
    needsServerSetup: () => mockNeedsServerSetup(),
    getConfig: () => ({ apiUrl: "https://current.example.com", googleClientId: "", publicUrl: "" }),
  };
});

vi.mock("@/lib/auth", () => ({
  useAuth: () => ({ accessToken: mockAccessToken, logout: mockLogout }),
}));

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

vi.mock("expo-router", () => ({
  useRouter: () => ({ replace: mockReplace, push: vi.fn(), back: mockBack }),
  useLocalSearchParams: () => ({}),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const parts = key.split(".");
      let val: unknown = en;
      for (const p of parts) {
        val = (val as Record<string, unknown>)?.[p];
      }
      return typeof val === "string" ? val : key;
    },
    i18n: { language: "en" },
  }),
}));

vi.mock("lucide-react-native", () => {
  const icon = (props: Record<string, unknown>) => createElement("svg", props);
  return { Server: icon };
});

import ServerSetupScreen from "@/app/server-setup";
import { ThemeProvider } from "@/lib/theme";

async function renderScreen() {
  await act(async () => {
    render(createElement(ThemeProvider, null, createElement(ServerSetupScreen)));
  });
}

const fetchMock = vi.fn();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  mockAccessToken = null;
  mockNeedsServerSetup.mockReturnValue(true);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

describe("ServerSetupScreen", () => {
  it("rejects an invalid URL without hitting the network", async () => {
    await renderScreen();
    fireEvent.change(screen.getByTestId("server-setup-input"), {
      target: { value: "ftp://nope" },
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("server-setup-connect"));
    });
    expect(screen.getByTestId("server-setup-error").textContent).toContain(
      "valid server address",
    );
    expect(fetchMock).not.toHaveBeenCalled();
    expect(mockSetServerUrl).not.toHaveBeenCalled();
  });

  it("verifies /healthz before saving and navigates home on success", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ status: "ok" }),
    });
    await renderScreen();
    fireEvent.change(screen.getByTestId("server-setup-input"), {
      target: { value: "toqui.example.com" },
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("server-setup-connect"));
    });
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "https://toqui.example.com/healthz",
        expect.anything(),
      );
      expect(mockSetServerUrl).toHaveBeenCalledWith("https://toqui.example.com");
      expect(mockReplace).toHaveBeenCalledWith("/");
    });
  });

  it("shows an error and does not save when the server is unreachable", async () => {
    fetchMock.mockRejectedValueOnce(new Error("network down"));
    await renderScreen();
    fireEvent.change(screen.getByTestId("server-setup-input"), {
      target: { value: "https://dead.example.com" },
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("server-setup-connect"));
    });
    await waitFor(() => {
      expect(screen.getByTestId("server-setup-error").textContent).toContain(
        "Couldn't reach a Toqui server",
      );
    });
    expect(mockSetServerUrl).not.toHaveBeenCalled();
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("rejects a healthz response that isn't a Toqui backend", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ hello: "world" }),
    });
    await renderScreen();
    fireEvent.change(screen.getByTestId("server-setup-input"), {
      target: { value: "https://not-toqui.example.com" },
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("server-setup-connect"));
    });
    await waitFor(() => {
      expect(screen.getByTestId("server-setup-error")).toBeTruthy();
    });
    expect(mockSetServerUrl).not.toHaveBeenCalled();
  });

  it("logs out before switching servers when signed in", async () => {
    mockNeedsServerSetup.mockReturnValue(false); // change-server mode
    mockAccessToken = "some-token";
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ status: "ok" }),
    });
    await renderScreen();
    fireEvent.change(screen.getByTestId("server-setup-input"), {
      target: { value: "https://new.example.com" },
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId("server-setup-connect"));
    });
    await waitFor(() => {
      expect(mockLogout).toHaveBeenCalled();
      expect(mockSetServerUrl).toHaveBeenCalledWith("https://new.example.com");
    });
  });

  it("offers a cancel button only when a server is already configured", async () => {
    mockNeedsServerSetup.mockReturnValue(false);
    await renderScreen();
    fireEvent.click(screen.getByTestId("server-setup-cancel"));
    expect(mockBack).toHaveBeenCalled();
  });
});
