"use client";

import { useState, useSyncExternalStore, useCallback, type FormEvent } from "react";
import { usePathname } from "next/navigation";

const STORAGE_KEY = "toqui_age_verified";
const EXEMPT_PATHS = ["/privacy", "/terms", "/waitlist"];
const EXEMPT_PREFIXES = ["/auth"];

// Subscribe to localStorage changes (no-op for this use case since we only write locally)
function subscribeToStorage(callback: () => void) {
  window.addEventListener("storage", callback);
  return () => window.removeEventListener("storage", callback);
}

function getVerifiedSnapshot(): boolean {
  return localStorage.getItem(STORAGE_KEY) === "true";
}

function getVerifiedServerSnapshot(): boolean {
  return false; // Always false on server to avoid hydration mismatch
}

function isExemptPath(pathname: string): boolean {
  return EXEMPT_PATHS.includes(pathname) || EXEMPT_PREFIXES.some((p) => pathname.startsWith(p));
}

function calculateAge(dob: Date): number {
  const today = new Date();
  let age = today.getFullYear() - dob.getFullYear();
  const monthDiff = today.getMonth() - dob.getMonth();
  if (monthDiff < 0 || (monthDiff === 0 && today.getDate() < dob.getDate())) {
    age--;
  }
  return age;
}

export function AgeGate({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  // useSyncExternalStore reads localStorage without useEffect/setState,
  // avoiding hydration mismatch (server snapshot returns false).
  const storedVerified = useSyncExternalStore(
    subscribeToStorage,
    getVerifiedSnapshot,
    getVerifiedServerSnapshot,
  );
  const [verified, setVerified] = useState(false);
  const [denied, setDenied] = useState(false);
  const [error, setError] = useState("");
  const [month, setMonth] = useState("");
  const [day, setDay] = useState("");
  const [year, setYear] = useState("");

  // Merge stored verification state with local state (set after form submit)
  const isUserVerified = storedVerified || verified;

  const handleSubmit = useCallback(
    (e: FormEvent<HTMLFormElement>) => {
      e.preventDefault();
      setError("");

      if (!year || !month || !day) {
        setError("Please enter your complete date of birth.");
        return;
      }

      const y = parseInt(year, 10);
      const m = parseInt(month, 10) - 1;
      const d = parseInt(day, 10);
      const dob = new Date(y, m, d);

      // Reject invalid dates — Date constructor silently coerces (e.g., Feb 30 → Mar 2)
      if (
        isNaN(dob.getTime()) ||
        dob.getFullYear() !== y ||
        dob.getMonth() !== m ||
        dob.getDate() !== d
      ) {
        setError("Please enter a valid date.");
        return;
      }

      const age = calculateAge(dob);
      if (age > 120 || age < 0) {
        setError("Please enter a valid date of birth.");
        return;
      }

      if (age < 18) {
        setDenied(true);
        return;
      }

      localStorage.setItem(STORAGE_KEY, "true");
      // Dispatch synthetic storage event so useSyncExternalStore picks up the
      // change in the same tab (the native "storage" event only fires cross-tab).
      window.dispatchEvent(new StorageEvent("storage", { key: STORAGE_KEY, newValue: "true" }));
      setVerified(true);
    },
    [year, month, day],
  );

  // Always render children for exempt paths (legal pages, auth flow, waitlist)
  if (isExemptPath(pathname)) {
    return <>{children}</>;
  }

  if (isUserVerified) {
    return <>{children}</>;
  }

  if (denied) {
    return (
      <div className="flex min-h-screen items-center justify-center p-8 bg-[var(--color-surface-secondary)]">
        <div className="max-w-md text-center">
          <h1 className="text-2xl font-bold text-[var(--color-text-primary)] mb-4">
            Age Requirement Not Met
          </h1>
          <p className="text-[var(--color-text-secondary)]">
            Toqui is only available to users who are 18 years of age or older. Thank you for your
            interest.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-8 bg-[var(--color-surface-secondary)]">
      <div className="max-w-md w-full">
        <div className="text-center mb-8">
          <h1 className="text-3xl font-bold tracking-tight text-[var(--color-text-primary)] mb-2">
            Welcome to Toqui
          </h1>
          <p className="text-[var(--color-text-secondary)]">
            Please confirm your date of birth to continue. You must be at least 18 years old to use
            this service.
          </p>
        </div>

        <form
          onSubmit={handleSubmit}
          className="bg-[var(--color-surface)] rounded-2xl p-8 shadow-sm border border-[var(--color-border)]"
        >
          <label className="block text-sm font-medium text-[var(--color-text-secondary)] mb-4">
            Date of Birth
          </label>

          <div className="grid grid-cols-3 gap-3 mb-6">
            <div>
              <label htmlFor="age-month" className="sr-only">
                Month
              </label>
              <input
                id="age-month"
                type="number"
                min="1"
                max="12"
                placeholder="MM"
                aria-label="Month"
                value={month}
                onChange={(e) => setMonth(e.target.value)}
                className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-secondary)] px-3 py-2.5 text-center text-[var(--color-text-primary)] placeholder:text-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]"
              />
            </div>

            <div>
              <label htmlFor="age-day" className="sr-only">
                Day
              </label>
              <input
                id="age-day"
                type="number"
                min="1"
                max="31"
                placeholder="DD"
                aria-label="Day"
                value={day}
                onChange={(e) => setDay(e.target.value)}
                className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-secondary)] px-3 py-2.5 text-center text-[var(--color-text-primary)] placeholder:text-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]"
              />
            </div>

            <div>
              <label htmlFor="age-year" className="sr-only">
                Year
              </label>
              <input
                id="age-year"
                type="number"
                min="1900"
                max={new Date().getFullYear()}
                placeholder="YYYY"
                aria-label="Year"
                value={year}
                onChange={(e) => setYear(e.target.value)}
                className="w-full rounded-lg border border-[var(--color-border)] bg-[var(--color-surface-secondary)] px-3 py-2.5 text-center text-[var(--color-text-primary)] placeholder:text-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]"
              />
            </div>
          </div>

          {error && (
            <p role="alert" className="text-sm text-[var(--color-error)] mb-4">
              {error}
            </p>
          )}

          <button
            type="submit"
            className="w-full rounded-full bg-[var(--color-accent)] px-6 py-3 text-white font-medium hover:bg-[var(--color-accent-hover)] transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)] focus-visible:ring-offset-2"
          >
            Continue
          </button>

          <p className="mt-4 text-xs text-center text-[var(--color-text-tertiary)]">
            By continuing, you agree to our{" "}
            <a href="/terms" className="text-[var(--color-accent)] hover:underline">
              Terms of Service
            </a>{" "}
            and{" "}
            <a href="/privacy" className="text-[var(--color-accent)] hover:underline">
              Privacy Policy
            </a>
            .
          </p>
        </form>
      </div>
    </div>
  );
}
