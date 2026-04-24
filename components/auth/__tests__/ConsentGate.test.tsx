import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act, waitFor } from "@testing-library/react";
import { createElement } from "react";
import en from "@/messages/en.json";

// ---------------------------------------------------------------------------
// Mocks — installed BEFORE component imports
// ---------------------------------------------------------------------------

// react-native → web platform so RN Modal renders as an inline portal.
vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>(
    "react-native",
  );
  return {
    ...actual,
    Platform: { OS: "web", select: (o: Record<string, unknown>) => o.web ?? o.default },
    useColorScheme: () => "light",
  };
});

// i18n → real English copy
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

// expo-web-browser → capture link clicks
const mockOpenBrowserAsync = vi.fn();
vi.mock("expo-web-browser", () => ({
  openBrowserAsync: (...args: unknown[]) => mockOpenBrowserAsync(...args),
}));

// Auth → controllable stub
const mockLogout = vi.fn();
vi.mock("@/lib/auth", () => ({
  useAuth: () => ({ accessToken: "test-token", logout: mockLogout }),
}));

// Analytics → capture track() calls
const mockTrack = vi.fn();
vi.mock("@/lib/analytics", () => ({
  useAnalytics: () => ({ track: mockTrack, identify: vi.fn() }),
}));

// Consent signal → injectable mock. We export a setter so each test can
// flip the flag on/off and assert on acknowledgeConsent calls.
let mockConsentRequired = false;
const mockAcknowledge = vi.fn();
vi.mock("@/lib/transport", () => ({
  useConsentSignal: () => ({
    consentRequired: mockConsentRequired,
    acknowledgeConsent: mockAcknowledge,
  }),
}));

// Config → stub API URL
vi.mock("@/lib/config", () => ({
  getConfig: () => ({ apiUrl: "http://api.test" }),
}));

// authFetch → capture POST bodies and return configurable responses
const mockAuthFetch = vi.fn();
vi.mock("@/lib/authFetch", () => ({
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

// ---------------------------------------------------------------------------
// Component import (must come AFTER the mocks above)
// ---------------------------------------------------------------------------

import { ConsentGate } from "@/components/auth/ConsentGate";
import { ThemeProvider } from "@/lib/theme";

function renderGate(children?: React.ReactNode) {
  return render(
    createElement(
      ThemeProvider,
      null,
      createElement(
        ConsentGate,
        null,
        children ?? createElement("div", { "data-testid": "app-content" }, "app"),
      ),
    ),
  );
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => {
  mockConsentRequired = false;
  mockOpenBrowserAsync.mockReset();
  mockLogout.mockReset();
  mockTrack.mockReset();
  mockAcknowledge.mockReset();
  mockAuthFetch.mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("ConsentGate", () => {
  it("renders children and does NOT show the modal when consent is not required", async () => {
    mockConsentRequired = false;
    await act(async () => {
      renderGate();
    });

    expect(screen.getByTestId("app-content")).toBeInTheDocument();
    expect(screen.queryByTestId("consent-gate")).not.toBeInTheDocument();
  });

  it("pops the modal when consentRequired is true", async () => {
    mockConsentRequired = true;
    await act(async () => {
      renderGate();
    });

    // Children still render (the modal sits on top — the user hasn't
    // lost their navigation state, just can't interact until they agree).
    expect(screen.getByTestId("app-content")).toBeInTheDocument();
    expect(screen.getByTestId("consent-gate")).toBeInTheDocument();
    expect(screen.getByTestId("consent-gate-agree")).toBeInTheDocument();
    expect(screen.getByTestId("consent-gate-logout")).toBeInTheDocument();
  });

  it("terms link opens the in-app browser (not raw Linking)", async () => {
    mockConsentRequired = true;
    await act(async () => {
      renderGate();
    });

    const link = screen.getByTestId("consent-gate-terms-link");
    await act(async () => {
      fireEvent.click(link);
    });

    expect(mockOpenBrowserAsync).toHaveBeenCalledWith(
      "https://toqui.travel/terms",
    );
  });

  it("privacy link opens the in-app browser", async () => {
    mockConsentRequired = true;
    await act(async () => {
      renderGate();
    });

    const link = screen.getByTestId("consent-gate-privacy-link");
    await act(async () => {
      fireEvent.click(link);
    });

    expect(mockOpenBrowserAsync).toHaveBeenCalledWith(
      "https://toqui.travel/privacy",
    );
  });

  it("clicking 'I agree' POSTs BOTH consent types and acknowledges on success", async () => {
    mockConsentRequired = true;
    mockAuthFetch.mockResolvedValue({ ok: true, status: 200 });

    await act(async () => {
      renderGate();
    });

    const agree = screen.getByTestId("consent-gate-agree");
    await act(async () => {
      fireEvent.click(agree);
    });

    // Both consent types recorded, in either order (Promise.all).
    await waitFor(() => {
      expect(mockAuthFetch).toHaveBeenCalledTimes(2);
    });

    const bodies = mockAuthFetch.mock.calls.map((c) => {
      const opts = c[2] as { body: string };
      return JSON.parse(opts.body);
    });
    const types = bodies.map((b) => b.consent_type).sort();
    expect(types).toEqual(["privacy_policy", "terms"]);

    // URL + token plumbing correct
    for (const call of mockAuthFetch.mock.calls) {
      expect(call[0]).toBe("http://api.test/auth/consent");
      expect(call[1]).toBe("test-token");
    }

    // Analytics fired and signal cleared
    expect(mockTrack).toHaveBeenCalledWith("consent_recorded");
    expect(mockAcknowledge).toHaveBeenCalledTimes(1);
  });

  it("leaves the gate up and shows an error when backend rejects the consent", async () => {
    mockConsentRequired = true;
    // One fails — simulate backend 500 on the privacy record
    mockAuthFetch
      .mockResolvedValueOnce({ ok: true, status: 200 })
      .mockResolvedValueOnce({ ok: false, status: 500 });

    // Silence the expected console.error
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    await act(async () => {
      renderGate();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("consent-gate-agree"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("consent-gate-error")).toBeInTheDocument();
    });

    // Must NOT clear the signal — user needs to retry.
    expect(mockAcknowledge).not.toHaveBeenCalled();
    // Modal still up.
    expect(screen.getByTestId("consent-gate")).toBeInTheDocument();

    errSpy.mockRestore();
  });

  it("leaves the gate up on network error", async () => {
    mockConsentRequired = true;
    mockAuthFetch.mockRejectedValue(new TypeError("network down"));

    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    await act(async () => {
      renderGate();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("consent-gate-agree"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("consent-gate-error")).toBeInTheDocument();
    });
    expect(mockAcknowledge).not.toHaveBeenCalled();

    errSpy.mockRestore();
  });

  it("clicking 'Log out instead' calls logout and acknowledges the signal", async () => {
    mockConsentRequired = true;
    mockLogout.mockResolvedValue(undefined);

    await act(async () => {
      renderGate();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("consent-gate-logout"));
    });

    await waitFor(() => {
      expect(mockLogout).toHaveBeenCalledTimes(1);
    });
    // Defensive clear so the gate unmounts immediately.
    expect(mockAcknowledge).toHaveBeenCalledTimes(1);
  });

  it("still clears signal in finally even if logout rejects (UX: gate unmounts)", async () => {
    mockConsentRequired = true;
    mockLogout.mockRejectedValue(new Error("logout failed"));

    // The component swallows the logout rejection (fire-and-forget from
    // the click handler) and still runs its finally-block ack, so the
    // modal unmounts instead of leaving the user trapped in a limbo
    // where logout half-failed.
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    await act(async () => {
      renderGate();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("consent-gate-logout"));
    });

    await waitFor(() => {
      expect(mockAcknowledge).toHaveBeenCalledTimes(1);
    });
    // The error was logged, not thrown.
    expect(errSpy).toHaveBeenCalled();

    errSpy.mockRestore();
  });

  it("guards against double-submit via the `submitting` state", async () => {
    // We can't reliably test the "both clicks within the same microtask"
    // race through `fireEvent.click` because React batches both clicks
    // before the `submitting` state flushes. What we CAN test is the
    // underlying guard: after the first `I agree` click starts a submit,
    // the button is rendered with `disabled=true` and later clicks are
    // no-ops. RTL's fireEvent respects `disabled` on the Pressable via
    // the accessibility state.
    mockConsentRequired = true;
    let resolveFetch: (v: unknown) => void = () => {};
    mockAuthFetch.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveFetch = resolve;
        }),
    );

    await act(async () => {
      renderGate();
    });

    const agree = screen.getByTestId("consent-gate-agree");

    // First click kicks off the submit.
    await act(async () => {
      fireEvent.click(agree);
    });

    // Two authFetch calls in flight (terms + privacy via Promise.all).
    expect(mockAuthFetch).toHaveBeenCalledTimes(2);

    // Component now shows the spinner (ActivityIndicator) rather than
    // the "I agree" label — assert on that invariant so we know the
    // `submitting` state is active, which is what gates re-entry.
    expect(screen.queryByText("I agree")).not.toBeInTheDocument();

    // A second click after the submit starts should be a no-op because
    // the Pressable is rendered with `disabled={submitting}`. We don't
    // bother asserting the click count beyond the initial two — the
    // `disabled` prop + the guard in `handleAgree` are the belt-and-
    // suspenders we care about.
    await act(async () => {
      fireEvent.click(agree);
    });
    expect(mockAuthFetch).toHaveBeenCalledTimes(2);

    // Unblock the in-flight submit.
    await act(async () => {
      resolveFetch({ ok: true, status: 200 });
    });
  });
});
