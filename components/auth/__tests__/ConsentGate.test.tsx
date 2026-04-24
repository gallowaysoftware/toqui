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

// React Query → capture invalidateQueries calls. The real QueryClient
// isn't needed here because ConsentGate just grabs the client via
// useQueryClient() and calls invalidateQueries() on it.
const mockInvalidateQueries = vi.fn();
vi.mock("@tanstack/react-query", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-query")>(
      "@tanstack/react-query",
    );
  return {
    ...actual,
    useQueryClient: () => ({
      invalidateQueries: mockInvalidateQueries,
    }),
  };
});

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
  mockInvalidateQueries.mockReset();
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

  it("clicking 'I agree' POSTs the batch consent body and acknowledges on success", async () => {
    mockConsentRequired = true;
    mockAuthFetch.mockResolvedValue({ ok: true, status: 201 });

    await act(async () => {
      renderGate();
    });

    const agree = screen.getByTestId("consent-gate-agree");
    await act(async () => {
      fireEvent.click(agree);
    });

    // One batch call to POST /auth/consent with both consents set.
    await waitFor(() => {
      expect(mockAuthFetch).toHaveBeenCalledTimes(1);
    });

    const [url, token, opts] = mockAuthFetch.mock.calls[0] as [
      string,
      string,
      { method: string; body: string },
    ];
    expect(url).toBe("http://api.test/auth/consent");
    expect(token).toBe("test-token");
    expect(opts.method).toBe("POST");

    // Body must match the `batchConsentRequest` shape defined in
    // toqui-backend/internal/handlers/consent.go. If this assertion
    // breaks, the backend's JSON decoder silently ignores extra fields
    // and the flag flip will look like it succeeded while the DB
    // rejects both consents — we MUST pin this shape.
    const body = JSON.parse(opts.body);
    expect(body).toEqual({
      terms_accepted: true,
      privacy_accepted: true,
    });

    // Analytics fired and signal cleared
    expect(mockTrack).toHaveBeenCalledWith("consent_recorded");
    expect(mockAcknowledge).toHaveBeenCalledTimes(1);
    // React Query caches invalidated so errored queries (the ones that
    // triggered `consent_required` in the first place) auto-refetch.
    expect(mockInvalidateQueries).toHaveBeenCalledTimes(1);
  });

  it("invalidates React Query caches BEFORE acknowledging the consent signal", async () => {
    // Ordering matters: if we ack first, the modal closes and any
    // screen listening for `consentRequired=false` may remount its
    // queries in a stale state. Invalidate first, THEN ack.
    mockConsentRequired = true;
    mockAuthFetch.mockResolvedValue({ ok: true, status: 201 });

    // Record the call order across the two mocks.
    const callOrder: string[] = [];
    mockInvalidateQueries.mockImplementation(() => {
      callOrder.push("invalidate");
    });
    mockAcknowledge.mockImplementation(() => {
      callOrder.push("acknowledge");
    });

    await act(async () => {
      renderGate();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("consent-gate-agree"));
    });

    await waitFor(() => {
      expect(mockAcknowledge).toHaveBeenCalledTimes(1);
    });
    expect(callOrder).toEqual(["invalidate", "acknowledge"]);
  });

  it("does NOT invalidate React Query caches when the backend rejects consent", async () => {
    // If the POST fails, the user is still gated — invalidating would
    // force a re-fetch storm against a still-gated session.
    mockConsentRequired = true;
    mockAuthFetch.mockResolvedValue({ ok: false, status: 500 });

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

    expect(mockInvalidateQueries).not.toHaveBeenCalled();

    errSpy.mockRestore();
  });

  it("leaves the gate up and shows an error when backend rejects the consent", async () => {
    mockConsentRequired = true;
    mockAuthFetch.mockResolvedValue({ ok: false, status: 500 });

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

    // First click kicks off the submit — one batch POST.
    await act(async () => {
      fireEvent.click(agree);
    });

    expect(mockAuthFetch).toHaveBeenCalledTimes(1);

    // Component now shows the spinner (ActivityIndicator) rather than
    // the "I agree" label — assert on that invariant so we know the
    // `submitting` state is active, which is what gates re-entry.
    expect(screen.queryByText("I agree")).not.toBeInTheDocument();

    // A second click after the submit starts should be a no-op because
    // the Pressable is rendered with `disabled={submitting}` AND
    // `handleAgree` early-returns when `submitting` is true.
    await act(async () => {
      fireEvent.click(agree);
    });
    expect(mockAuthFetch).toHaveBeenCalledTimes(1);

    // Unblock the in-flight submit.
    await act(async () => {
      resolveFetch({ ok: true, status: 200 });
    });
  });
});
