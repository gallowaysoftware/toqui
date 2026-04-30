import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";

// useOnboarding gates the onboarding modal on first launch. A bug
// here either:
//   - re-prompts every session (UX annoyance, defeats the
//     "complete-once" contract)
//   - silently treats new users as already-onboarded (hides the
//     onboarding flow entirely → AB test data corruption)
//
// Storage backend is sessionStorage on web, expo-secure-store on
// native. Pin both paths.

vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Platform: { OS: "web", select: (o: Record<string, unknown>) => o.web ?? o.default },
  };
});

import { useOnboarding } from "../useOnboarding";

const STORAGE_KEY = "toqui_onboarding_complete";

beforeEach(() => {
  sessionStorage.clear();
  vi.clearAllMocks();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("useOnboarding (web)", () => {
  it("starts in loading state (isComplete=null)", async () => {
    const { result } = renderHook(() => useOnboarding());
    // Initial render: no value yet from the async storage probe.
    expect(result.current.isOnboardingComplete).toBeNull();
    expect(result.current.isLoading).toBe(true);
    // Allow the effect to settle for cleanup.
    await waitFor(() => expect(result.current.isLoading).toBe(false));
  });

  it("returns true when sessionStorage already has the flag", async () => {
    sessionStorage.setItem(STORAGE_KEY, "true");
    const { result } = renderHook(() => useOnboarding());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isOnboardingComplete).toBe(true);
  });

  it("returns false when sessionStorage is empty", async () => {
    const { result } = renderHook(() => useOnboarding());

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isOnboardingComplete).toBe(false);
  });

  it("treats sessionStorage values other than 'true' as not complete", async () => {
    // Defensive: if some legacy value sneaks in (e.g. "1", "yes",
    // "complete"), we MUST treat it as not-complete and re-prompt.
    // The storage compares strictly to the literal "true" string;
    // pin that to avoid silent reads for malformed values.
    for (const stored of ["1", "yes", "complete", "TRUE", "false"]) {
      sessionStorage.setItem(STORAGE_KEY, stored);
      const { result } = renderHook(() => useOnboarding());
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(
        result.current.isOnboardingComplete,
        `stored=${stored}`,
      ).toBe(false);
      sessionStorage.clear();
    }
  });

  it("completeOnboarding writes the flag and flips state to true", async () => {
    const { result } = renderHook(() => useOnboarding());
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.isOnboardingComplete).toBe(false);

    await act(async () => {
      await result.current.completeOnboarding();
    });

    expect(result.current.isOnboardingComplete).toBe(true);
    expect(sessionStorage.getItem(STORAGE_KEY)).toBe("true");
  });

  it("isLoading flips false after completeOnboarding even if it was never true initially", async () => {
    // Edge: useOnboarding completed loading (false), user hits the
    // CTA, completeOnboarding awaits + flips. Pin that isLoading
    // stays false (was already false; should not re-toggle to true
    // during the write).
    const { result } = renderHook(() => useOnboarding());
    await waitFor(() => expect(result.current.isLoading).toBe(false));

    await act(async () => {
      await result.current.completeOnboarding();
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isOnboardingComplete).toBe(true);
  });

  it("persists across mounts (the whole point of the storage)", async () => {
    // Simulate first session: complete onboarding.
    {
      const { result } = renderHook(() => useOnboarding());
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      await act(async () => {
        await result.current.completeOnboarding();
      });
    }
    // Simulate second session (new render): should already be complete.
    {
      const { result } = renderHook(() => useOnboarding());
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.isOnboardingComplete).toBe(true);
    }
  });
});
