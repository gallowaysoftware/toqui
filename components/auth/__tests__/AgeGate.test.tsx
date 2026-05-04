import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { createElement } from "react";
import en from "@/messages/en.json";

// AgeGate redesign tests (toqui-backend#420 / age-gate-redesign-frontend).
//
// What changed from the old test suite:
//
//   - localStorage is no longer the source of truth. The new component
//     derives "verified" from `user.ageVerifiedAt` only. Tests no longer
//     setItem the old `toqui_age_verified` key.
//   - Logged-out users are not gated. The old behaviour was "always
//     show the form until localStorage cache exists"; the new behaviour
//     is "render children unchanged when there's no accessToken".
//   - Under-18 DOB is now a SERVER-side decision. The component sends
//     the DOB to /auth/verify-age and reacts to the 403 response,
//     instead of short-circuiting client-side. Tests mock authFetch
//     to drive the various response shapes.
//   - Successful verification no longer writes localStorage; the
//     auth context's `user.ageVerifiedAt` repopulates on next refresh
//     and the next render bypasses the gate.

vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Platform: { OS: "web" },
    useColorScheme: () => "light",
  };
});

// Mocked auth context. Each describe block sets `mockedUseAuth.mockReturnValue`
// to drive a specific (accessToken, user) state into the component.
const mockedLogout = vi.fn(async () => {});
const mockedUseAuth = vi.fn(() => ({
  accessToken: null as string | null,
  user: null as { ageVerifiedAt: string | null } | null,
  logout: mockedLogout,
}));
vi.mock("@/lib/auth", () => ({
  useAuth: () => mockedUseAuth(),
}));

// Mocked authFetch. Tests set the response shape per-case.
const mockedAuthFetch = vi.fn();
vi.mock("@/lib/authFetch", () => ({
  authFetch: (url: string, token: string, opts: RequestInit) =>
    mockedAuthFetch(url, token, opts),
}));

vi.mock("@/lib/config", () => ({
  getConfig: () => ({ apiUrl: "https://api.test" }),
}));

// Mocked analytics — capture the events fired so we can assert on them.
const mockedTrack = vi.fn();
vi.mock("@/lib/analytics", () => ({
  useAnalytics: () => ({ track: mockedTrack }),
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

import { AgeGate } from "@/components/auth/AgeGate";
import { ThemeProvider } from "@/lib/theme";

function renderAgeGate(children?: React.ReactNode) {
  return render(
    createElement(
      ThemeProvider,
      null,
      createElement(
        AgeGate,
        null,
        children ?? createElement("div", { "data-testid": "protected-content" }, "Protected"),
      ),
    ),
  );
}

function enterDOB(month: string, day: string, year: string) {
  fireEvent.change(screen.getByPlaceholderText("MM"), { target: { value: month } });
  fireEvent.change(screen.getByPlaceholderText("DD"), { target: { value: day } });
  fireEvent.change(screen.getByPlaceholderText("YYYY"), { target: { value: year } });
  fireEvent.click(screen.getByText("Verify age"));
}

function dobForAge(age: number): { year: string; month: string; day: string } {
  const today = new Date();
  return {
    year: String(today.getFullYear() - age),
    month: String(today.getMonth() + 1),
    day: String(today.getDate()),
  };
}

beforeEach(() => {
  mockedUseAuth.mockReturnValue({ accessToken: null, user: null, logout: mockedLogout });
  mockedAuthFetch.mockReset();
  mockedTrack.mockReset();
  mockedLogout.mockClear();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("AgeGate — logged-out short-circuit", () => {
  it("renders children when no accessToken (pre-login screens are not gated)", async () => {
    // The structural change: a logged-out visitor sees the
    // marketing/sign-in screens behind us, never the DOB form.
    mockedUseAuth.mockReturnValue({ accessToken: null, user: null, logout: mockedLogout });
    await act(async () => {
      renderAgeGate();
    });
    expect(screen.getByTestId("protected-content")).toBeInTheDocument();
    expect(screen.queryByText("Verify age")).not.toBeInTheDocument();
  });
});

describe("AgeGate — already-verified pass-through", () => {
  it("renders children when user.ageVerifiedAt is set", async () => {
    mockedUseAuth.mockReturnValue({
      accessToken: "token",
      user: { ageVerifiedAt: "2026-01-01T00:00:00Z" },
      logout: mockedLogout,
    });
    await act(async () => {
      renderAgeGate();
    });
    expect(screen.getByTestId("protected-content")).toBeInTheDocument();
  });
});

describe("AgeGate — form submission", () => {
  beforeEach(() => {
    mockedUseAuth.mockReturnValue({
      accessToken: "token",
      user: { ageVerifiedAt: null },
      logout: mockedLogout,
    });
  });

  it("shows the form with the new contextual copy", async () => {
    await act(async () => {
      renderAgeGate();
    });
    expect(screen.getByText("Quick check before we plan your trip")).toBeInTheDocument();
    expect(
      screen.getByText("We don't store the date itself — only that you've verified."),
    ).toBeInTheDocument();
    expect(screen.getByText("Verify age")).toBeInTheDocument();
  });

  it("posts DOB to /auth/verify-age and tracks success on 200", async () => {
    mockedAuthFetch.mockResolvedValue({ ok: true, status: 200 });

    await act(async () => {
      renderAgeGate();
    });
    const { month, day, year } = dobForAge(25);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(mockedAuthFetch).toHaveBeenCalledWith(
      "https://api.test/auth/verify-age",
      "token",
      expect.objectContaining({ method: "POST" }),
    );
    const call = mockedAuthFetch.mock.calls[0];
    const body = JSON.parse((call[2] as RequestInit).body as string);
    expect(body.date_of_birth).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(mockedTrack).toHaveBeenCalledWith("age_gate_passed");
  });

  it("does NOT short-circuit under-18 client-side — sends DOB to backend", async () => {
    // The old component blocked under-18 in the client. The new design
    // is "the backend is the single enforcement point" — under-18 must
    // still hit /auth/verify-age so the deletion + block-list write
    // happens server-side. Pin: an under-18 DOB triggers a fetch.
    mockedAuthFetch.mockResolvedValue({
      ok: false,
      status: 403,
      json: async () => ({ error: "under_age", message: "deleted" }),
    });

    await act(async () => {
      renderAgeGate();
    });
    const { month, day, year } = dobForAge(15);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(mockedAuthFetch).toHaveBeenCalled();
  });

  it("shows the deletion-confirmation screen on 403 under_age + clears auth", async () => {
    mockedAuthFetch.mockResolvedValue({
      ok: false,
      status: 403,
      json: async () => ({ error: "under_age", message: "deleted" }),
    });

    await act(async () => {
      renderAgeGate();
    });
    const { month, day, year } = dobForAge(15);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(screen.getByText("Account deleted")).toBeInTheDocument();
    expect(mockedTrack).toHaveBeenCalledWith("age_gate_under_age_refused");
    expect(mockedLogout).toHaveBeenCalledTimes(1);
    // The form is gone — user can't keep poking at it.
    expect(screen.queryByText("Verify age")).not.toBeInTheDocument();
  });

  it("shows tryAgain on a non-403 backend error (no deletion claim)", async () => {
    mockedAuthFetch.mockResolvedValue({ ok: false, status: 500 });

    await act(async () => {
      renderAgeGate();
    });
    const { month, day, year } = dobForAge(25);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(screen.getByText("Something went wrong. Please try again.")).toBeInTheDocument();
    // CRITICAL: a 500 must NOT show the deletion screen, because the
    // backend may not have actually deleted anything.
    expect(screen.queryByText("Account deleted")).not.toBeInTheDocument();
    expect(mockedLogout).not.toHaveBeenCalled();
  });

  it("shows tryAgain on a 403 without an under_age error code", async () => {
    // A 403 from some OTHER error path (e.g., consent gate, age
    // interceptor on a non-verify endpoint) must not be misread as the
    // deletion confirmation. Pin the error-body match.
    mockedAuthFetch.mockResolvedValue({
      ok: false,
      status: 403,
      json: async () => ({ error: "consent_required" }),
    });

    await act(async () => {
      renderAgeGate();
    });
    const { month, day, year } = dobForAge(25);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(screen.getByText("Something went wrong. Please try again.")).toBeInTheDocument();
    expect(screen.queryByText("Account deleted")).not.toBeInTheDocument();
    expect(mockedLogout).not.toHaveBeenCalled();
  });

  it("rejects malformed dates client-side (no fetch fired)", async () => {
    await act(async () => {
      renderAgeGate();
    });
    await act(async () => {
      enterDOB("13", "32", "1990"); // month 13, day 32 — invalid
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
    expect(mockedAuthFetch).not.toHaveBeenCalled();
  });

  it("survives a fetch network error with tryAgain (not deletion)", async () => {
    mockedAuthFetch.mockRejectedValue(new Error("network down"));

    await act(async () => {
      renderAgeGate();
    });
    const { month, day, year } = dobForAge(25);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(screen.getByText("Something went wrong. Please try again.")).toBeInTheDocument();
    expect(screen.queryByText("Account deleted")).not.toBeInTheDocument();
    expect(mockedLogout).not.toHaveBeenCalled();
  });
});
