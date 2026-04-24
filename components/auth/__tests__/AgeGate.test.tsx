import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act, waitFor } from "@testing-library/react";
import { createElement } from "react";
import en from "@/messages/en.json";

// Mock react-native with web platform
vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Platform: { OS: "web" },
    useColorScheme: () => "light",
  };
});

// Mock auth so AgeGate doesn't require AuthProvider in tests.
// Tests can override the mock per-describe via `mockedUseAuth.mockReturnValue(...)`.
const mockedUseAuth = vi.fn(() => ({ accessToken: null, user: null } as {
  accessToken: string | null;
  user: { ageVerifiedAt: string | null } | null;
}));
vi.mock("@/lib/auth", () => ({
  useAuth: () => mockedUseAuth(),
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

import { AgeGate } from "@/components/auth/AgeGate";
import { ThemeProvider } from "@/lib/theme";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderAgeGate(children?: React.ReactNode) {
  return render(
    createElement(
      ThemeProvider,
      null,
      createElement(AgeGate, null, children ?? createElement("div", { "data-testid": "protected-content" }, "Protected")),
    ),
  );
}

/** Fill the date fields and submit */
function enterDOB(month: string, day: string, year: string) {
  const monthInput = screen.getByPlaceholderText("MM");
  const dayInput = screen.getByPlaceholderText("DD");
  const yearInput = screen.getByPlaceholderText("YYYY");

  fireEvent.change(monthInput, { target: { value: month } });
  fireEvent.change(dayInput, { target: { value: day } });
  fireEvent.change(yearInput, { target: { value: year } });

  const button = screen.getByText("Verify Age");
  fireEvent.click(button);
}

/** Build a date string for someone who turns `age` today */
function dobForAge(age: number): { year: string; month: string; day: string } {
  const today = new Date();
  return {
    year: String(today.getFullYear() - age),
    month: String(today.getMonth() + 1),
    day: String(today.getDate()),
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

beforeEach(() => {
  localStorage.clear();
  mockedUseAuth.mockReturnValue({ accessToken: null, user: null });
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("AgeGate", () => {
  // ------ Renders children when verified ------

  it("renders children immediately when already verified in storage", async () => {
    localStorage.setItem("toqui_age_verified", "true");

    await act(async () => {
      renderAgeGate();
    });

    expect(screen.getByTestId("protected-content")).toBeInTheDocument();
  });

  it("does NOT render children when storage value is not exactly 'true'", async () => {
    localStorage.setItem("toqui_age_verified", "yes");

    await act(async () => {
      renderAgeGate();
    });

    // Should show the age gate form, not the children
    expect(screen.queryByTestId("protected-content")).not.toBeInTheDocument();
    expect(screen.getByText("Age Verification")).toBeInTheDocument();
  });

  it("shows verification form when not yet verified", async () => {
    await act(async () => {
      renderAgeGate();
    });

    expect(screen.getByText("Age Verification")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("MM")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("DD")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("YYYY")).toBeInTheDocument();
    expect(screen.getByText("Verify Age")).toBeInTheDocument();
  });

  // ------ Successful verification ------

  it("renders children after valid 18+ DOB is submitted", async () => {
    await act(async () => {
      renderAgeGate();
    });

    const { month, day, year } = dobForAge(25);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(screen.getByTestId("protected-content")).toBeInTheDocument();
  });

  it("persists verification to localStorage after success", async () => {
    await act(async () => {
      renderAgeGate();
    });

    const { month, day, year } = dobForAge(30);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(localStorage.getItem("toqui_age_verified")).toBe("true");
  });

  // ------ Exact 18th birthday edge case ------

  it("allows entry on exact 18th birthday (today - 18 years exactly)", async () => {
    await act(async () => {
      renderAgeGate();
    });

    const { month, day, year } = dobForAge(18);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(screen.getByTestId("protected-content")).toBeInTheDocument();
  });

  it("denies entry one day before 18th birthday", async () => {
    await act(async () => {
      renderAgeGate();
    });

    // Person born tomorrow, 18 years ago -- they turn 18 tomorrow
    const today = new Date();
    const tomorrow = new Date(today);
    tomorrow.setDate(tomorrow.getDate() + 1);
    const year = String(tomorrow.getFullYear() - 18);
    const month = String(tomorrow.getMonth() + 1);
    const day = String(tomorrow.getDate());

    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(screen.getByText("Access Denied")).toBeInTheDocument();
    expect(screen.queryByTestId("protected-content")).not.toBeInTheDocument();
  });

  // ------ Underage denial ------

  it("shows denial screen for 17-year-old", async () => {
    await act(async () => {
      renderAgeGate();
    });

    const { month, day, year } = dobForAge(17);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(screen.getByText("Access Denied")).toBeInTheDocument();
    expect(screen.getByText(/must be at least 18 years old/)).toBeInTheDocument();
    expect(screen.queryByTestId("protected-content")).not.toBeInTheDocument();
  });

  it("shows denial screen for a 5-year-old", async () => {
    await act(async () => {
      renderAgeGate();
    });

    const { month, day, year } = dobForAge(5);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(screen.getByText("Access Denied")).toBeInTheDocument();
  });

  it("does NOT persist verification when underage", async () => {
    await act(async () => {
      renderAgeGate();
    });

    const { month, day, year } = dobForAge(15);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(localStorage.getItem("toqui_age_verified")).toBeNull();
  });

  // ------ Denial is permanent (no retry) ------

  it("denial screen has no verify button to retry", async () => {
    await act(async () => {
      renderAgeGate();
    });

    const { month, day, year } = dobForAge(10);
    await act(async () => {
      enterDOB(month, day, year);
    });

    expect(screen.getByText("Access Denied")).toBeInTheDocument();
    expect(screen.queryByText("Verify Age")).not.toBeInTheDocument();
  });

  // ------ Round-trip date validation ------

  it("rejects February 30 (invalid day for month)", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("2", "30", "1990");
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
    expect(screen.queryByTestId("protected-content")).not.toBeInTheDocument();
    expect(screen.queryByText("Access Denied")).not.toBeInTheDocument();
  });

  it("rejects February 29 in a non-leap year (e.g. 2001)", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("2", "29", "2001");
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
  });

  it("accepts February 29 in a leap year (e.g. 2000)", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("2", "29", "2000");
    });

    // Should NOT show error -- person is old enough
    expect(screen.queryByText("Please enter a valid date of birth.")).not.toBeInTheDocument();
  });

  it("rejects April 31 (April has 30 days)", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("4", "31", "1990");
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
  });

  it("rejects September 31", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("9", "31", "1985");
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
  });

  // ------ Boundary validation ------

  it("rejects month 0", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("0", "15", "1990");
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
  });

  it("rejects month 13", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("13", "15", "1990");
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
  });

  it("rejects day 0", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("6", "0", "1990");
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
  });

  it("rejects day 32", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("1", "32", "1990");
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
  });

  it("rejects year before 1900", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("6", "15", "1899");
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
  });

  it("rejects year in the future", async () => {
    await act(async () => {
      renderAgeGate();
    });

    const futureYear = String(new Date().getFullYear() + 1);
    await act(async () => {
      enterDOB("6", "15", futureYear);
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
  });

  // ------ Non-numeric / empty input ------

  it("rejects non-numeric input", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("ab", "cd", "efgh");
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
  });

  it("rejects empty fields", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("", "", "");
    });

    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();
  });

  // ------ Error is cleared on next valid attempt ------

  it("clears error message when submitting a valid date after an invalid one", async () => {
    await act(async () => {
      renderAgeGate();
    });

    // First: invalid date
    await act(async () => {
      enterDOB("2", "30", "1990");
    });
    expect(screen.getByText("Please enter a valid date of birth.")).toBeInTheDocument();

    // Second: valid date of an adult
    const { month, day, year } = dobForAge(25);
    await act(async () => {
      enterDOB(month, day, year);
    });

    // Error should be gone, children should render
    expect(screen.queryByText("Please enter a valid date of birth.")).not.toBeInTheDocument();
    expect(screen.getByTestId("protected-content")).toBeInTheDocument();
  });

  // ------ Server-side age_verified_at skips the gate ------

  describe("backend age_verified_at (issue #371)", () => {
    it("skips the gate when user.ageVerifiedAt is set, even with no localStorage", async () => {
      mockedUseAuth.mockReturnValue({
        accessToken: "token",
        user: {
          ageVerifiedAt: new Date("2026-04-20T12:00:00Z").toISOString(),
        },
      });

      await act(async () => {
        renderAgeGate();
      });

      expect(screen.getByTestId("protected-content")).toBeInTheDocument();
      expect(screen.queryByText("Age Verification")).not.toBeInTheDocument();
    });

    it("still shows the gate when user.ageVerifiedAt is null", async () => {
      mockedUseAuth.mockReturnValue({
        accessToken: "token",
        user: { ageVerifiedAt: null },
      });

      await act(async () => {
        renderAgeGate();
      });

      expect(screen.getByText("Age Verification")).toBeInTheDocument();
      expect(screen.queryByTestId("protected-content")).not.toBeInTheDocument();
    });

    it("falls back to localStorage when user.ageVerifiedAt is null but local flag set", async () => {
      localStorage.setItem("toqui_age_verified", "true");
      mockedUseAuth.mockReturnValue({
        accessToken: "token",
        user: { ageVerifiedAt: null },
      });

      await act(async () => {
        renderAgeGate();
      });

      expect(screen.getByTestId("protected-content")).toBeInTheDocument();
    });

    it("skips the gate when user.ageVerifiedAt is set and no accessToken (offline-first boot)", async () => {
      mockedUseAuth.mockReturnValue({
        accessToken: null,
        user: {
          ageVerifiedAt: new Date("2026-01-01T00:00:00Z").toISOString(),
        },
      });

      await act(async () => {
        renderAgeGate();
      });

      expect(screen.getByTestId("protected-content")).toBeInTheDocument();
    });
  });

  // ------ Very old people ------

  it("accepts year 1900 (126-year-old)", async () => {
    await act(async () => {
      renderAgeGate();
    });

    await act(async () => {
      enterDOB("1", "1", "1900");
    });

    // Should not show error (date is valid, person is definitely 18+)
    expect(screen.queryByText("Please enter a valid date of birth.")).not.toBeInTheDocument();
    expect(screen.getByTestId("protected-content")).toBeInTheDocument();
  });
});
