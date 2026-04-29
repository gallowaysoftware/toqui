import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act, waitFor } from "@testing-library/react";
import { createElement } from "react";
import en from "@/messages/en.json";

// react-native shim — same shape used by AgeGate.test.tsx so the modal
// component renders against the web platform path.
vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Platform: {
      OS: "web",
      select: (o: Record<string, unknown>) => o.web ?? o.default,
    },
    useColorScheme: () => "light",
  };
});

const mockedUseAuth = vi.fn(() => ({
  accessToken: null as string | null,
  user: null as { id: string } | null,
}));
vi.mock("@/lib/auth", () => ({
  useAuth: () => mockedUseAuth(),
}));

const mockedTrack = vi.fn();
vi.mock("@/lib/analytics", () => ({
  useAnalytics: () => ({
    track: mockedTrack,
    identify: vi.fn(),
    reset: vi.fn(),
    getFeatureFlag: vi.fn(),
  }),
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

import { AIDisclaimerGate } from "@/components/auth/AIDisclaimerGate";
import { ThemeProvider } from "@/lib/theme";

function renderGate(children?: React.ReactNode) {
  return render(
    createElement(
      ThemeProvider,
      null,
      createElement(
        AIDisclaimerGate,
        null,
        children ??
          createElement("div", { "data-testid": "protected" }, "Protected"),
      ),
    ),
  );
}

const STORAGE_KEY = (id: string) => `toqui_ai_disclaimer_acked_v1_${id}`;

beforeEach(() => {
  localStorage.clear();
  mockedUseAuth.mockReturnValue({ accessToken: null, user: null });
  mockedTrack.mockReset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("AIDisclaimerGate", () => {
  it("renders children and no modal when no user is signed in", async () => {
    await act(async () => {
      renderGate();
    });
    expect(screen.getByTestId("protected")).toBeInTheDocument();
    expect(screen.queryByTestId("ai-disclaimer-gate")).not.toBeInTheDocument();
  });

  it("shows the modal when an authenticated user has not acknowledged yet", async () => {
    mockedUseAuth.mockReturnValue({
      accessToken: "token",
      user: { id: "user-1" },
    });

    await act(async () => {
      renderGate();
    });

    await waitFor(() => {
      expect(screen.getByTestId("ai-disclaimer-gate")).toBeInTheDocument();
    });
    expect(screen.getByText(en.aiDisclaimer.title)).toBeInTheDocument();
    expect(screen.getByText(en.aiDisclaimer.acknowledge)).toBeInTheDocument();
    // children still render — the modal sits on top, doesn't hide them
    expect(screen.getByTestId("protected")).toBeInTheDocument();
  });

  it("does not show the modal when the user already acknowledged", async () => {
    localStorage.setItem(STORAGE_KEY("user-1"), "true");
    mockedUseAuth.mockReturnValue({
      accessToken: "token",
      user: { id: "user-1" },
    });

    await act(async () => {
      renderGate();
    });

    // Allow the storage read to settle
    await waitFor(() => {
      expect(screen.getByTestId("protected")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("ai-disclaimer-gate")).not.toBeInTheDocument();
  });

  it("acknowledges, persists per-user, and tracks the event", async () => {
    mockedUseAuth.mockReturnValue({
      accessToken: "token",
      user: { id: "user-1" },
    });

    await act(async () => {
      renderGate();
    });

    await waitFor(() => {
      expect(screen.getByTestId("ai-disclaimer-acknowledge")).toBeInTheDocument();
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("ai-disclaimer-acknowledge"));
    });

    // Behavioural contract: clicking acknowledge writes the per-user
    // storage flag and fires the analytics event. We deliberately don't
    // assert the modal unmounts in the DOM here because react-native-web
    // keeps Modal children in the tree with display:none when visible
    // flips, which makes a "not in the document" check brittle.
    await waitFor(() => {
      expect(localStorage.getItem(STORAGE_KEY("user-1"))).toBe("true");
    });
    expect(mockedTrack).toHaveBeenCalledWith("ai_disclaimer_acknowledged");
  });

  it("re-prompts a different user on the same device", async () => {
    // user-1 acknowledged earlier on this device
    localStorage.setItem(STORAGE_KEY("user-1"), "true");

    // user-2 signs in fresh — should still see the modal
    mockedUseAuth.mockReturnValue({
      accessToken: "token-2",
      user: { id: "user-2" },
    });

    await act(async () => {
      renderGate();
    });

    await waitFor(() => {
      expect(screen.getByTestId("ai-disclaimer-gate")).toBeInTheDocument();
    });
  });

  it("fails CLOSED when localStorage.getItem throws on the AI-disclaimer key (iOS private mode, etc.)", async () => {
    // Simulate iOS Safari private browsing or quota-exceeded — read
    // throws ONLY for the AI-disclaimer key. Mocking the whole
    // Storage.prototype.getItem leaks into ThemeProvider's persisted
    // theme read and causes spurious unhandled rejections, so we
    // scope the failure to the key under test.
    const realGetItem = Storage.prototype.getItem;
    const getItemSpy = vi
      .spyOn(Storage.prototype, "getItem")
      .mockImplementation(function (this: Storage, key: string) {
        if (key === STORAGE_KEY("user-1")) {
          throw new Error("QuotaExceededError: private mode");
        }
        return realGetItem.call(this, key);
      });

    mockedUseAuth.mockReturnValue({
      accessToken: "token",
      user: { id: "user-1" },
    });

    await act(async () => {
      renderGate();
    });

    // Pre-fix this would leave needsAck stuck at null, the modal would
    // never render, and the user would bypass the legal gate. The fix
    // (issue #198) catches the error in getStorageItem and returns null,
    // which `null !== "true"` evaluates to true, so the modal renders.
    await waitFor(() => {
      expect(screen.getByTestId("ai-disclaimer-gate")).toBeInTheDocument();
    });

    getItemSpy.mockRestore();
  });

  it("does NOT trap user on the modal when localStorage.setItem throws on acknowledge", async () => {
    // The legal audit trail is the PostHog event, not the local flag.
    // If the storage write fails we still want to: (1) fire the event
    // (so the trail captures the click), and (2) dismiss the modal so
    // the user isn't stuck. They'll re-see the modal on next session,
    // which is acceptable.
    mockedUseAuth.mockReturnValue({
      accessToken: "token",
      user: { id: "user-1" },
    });

    await act(async () => {
      renderGate();
    });

    await waitFor(() => {
      expect(screen.getByTestId("ai-disclaimer-acknowledge")).toBeInTheDocument();
    });

    const realSetItem = Storage.prototype.setItem;
    const setItemSpy = vi
      .spyOn(Storage.prototype, "setItem")
      .mockImplementation(function (this: Storage, key: string, value: string) {
        if (key === STORAGE_KEY("user-1")) {
          throw new Error("QuotaExceededError");
        }
        return realSetItem.call(this, key, value);
      });

    await act(async () => {
      fireEvent.click(screen.getByTestId("ai-disclaimer-acknowledge"));
    });

    // The PostHog event MUST fire even when storage fails — the
    // event is the actual audit trail.
    await waitFor(() => {
      expect(mockedTrack).toHaveBeenCalledWith("ai_disclaimer_acknowledged");
    });

    setItemSpy.mockRestore();
  });
});
