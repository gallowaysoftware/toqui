import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, act } from "@testing-library/react";
import { createElement } from "react";

vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Platform: { OS: "web", select: (o: Record<string, unknown>) => o.web ?? o.default },
    useColorScheme: () => "light",
  };
});

const mockReplace = vi.fn();
vi.mock("expo-router", () => ({
  useRouter: () => ({ replace: mockReplace }),
}));

vi.mock("expo-web-browser", () => ({
  maybeCompleteAuthSession: vi.fn(),
}));

// Mock oidc-auth so importing the callback doesn't pull in expo-auth-session;
// only the marker key constant is needed.
vi.mock("@/lib/oidc-auth", () => ({
  OIDC_PENDING_KEY: "toqui_oidc_pending",
}));

const mockLogin = vi.fn(() => Promise.resolve());
const mockLoginWithOIDC = vi.fn(() => Promise.resolve());
vi.mock("@/lib/auth", () => ({
  useAuth: () => ({ login: mockLogin, loginWithOIDC: mockLoginWithOIDC }),
}));

import AuthCallbackScreen from "@/app/auth/callback";
import { OIDC_PENDING_KEY } from "@/lib/oidc-auth";

async function renderCallback(url: string) {
  window.history.replaceState({}, "", url);
  await act(async () => {
    render(createElement(AuthCallbackScreen));
    await Promise.resolve();
  });
}

describe("auth/callback OIDC routing", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
  });

  it("routes a returning code to Google when no OIDC marker is set", async () => {
    await renderCallback("/auth/callback?code=google-code");

    expect(mockLogin).toHaveBeenCalledWith("google-code", `${window.location.origin}/auth/callback`);
    expect(mockLoginWithOIDC).not.toHaveBeenCalled();
  });

  it("routes a returning code to OIDC when the marker is set, then clears it", async () => {
    sessionStorage.setItem(OIDC_PENDING_KEY, "1");

    await renderCallback("/auth/callback?code=sso-code");

    expect(mockLoginWithOIDC).toHaveBeenCalledWith(
      "sso-code",
      `${window.location.origin}/auth/callback`,
      "",
    );
    expect(mockLogin).not.toHaveBeenCalled();
    expect(sessionStorage.getItem(OIDC_PENDING_KEY)).toBeNull();
  });

  it("clears a stale marker even when the IdP returned no code (abandon/error)", async () => {
    // Regression for the leaked-marker misroute: an abandoned OIDC attempt
    // must not leave a marker that hijacks the next Google login.
    sessionStorage.setItem(OIDC_PENDING_KEY, "1");

    await renderCallback("/auth/callback?error=access_denied");

    expect(mockLogin).not.toHaveBeenCalled();
    expect(mockLoginWithOIDC).not.toHaveBeenCalled();
    expect(sessionStorage.getItem(OIDC_PENDING_KEY)).toBeNull();
  });
});
