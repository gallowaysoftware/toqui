import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { createElement } from "react";
import en from "@/messages/en.json";

vi.mock("react-native", async () => {
  const actual = await vi.importActual<typeof import("react-native")>("react-native");
  return {
    ...actual,
    Platform: { OS: "web", select: (o: Record<string, unknown>) => o.web ?? o.default },
    useColorScheme: () => "light",
  };
});

const mockUseOIDCAuth = vi.fn();
vi.mock("@/lib/oidc-auth", () => ({
  useOIDCAuth: () => mockUseOIDCAuth(),
}));

// t() with minimal {{var}} interpolation so we can assert the rendered label.
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, vars?: Record<string, string>) => {
      const parts = key.split(".");
      let val: unknown = en;
      for (const p of parts) val = (val as Record<string, unknown>)?.[p];
      let str = typeof val === "string" ? val : key;
      if (vars) for (const [k, v] of Object.entries(vars)) str = str.replace(`{{${k}}}`, v);
      return str;
    },
    i18n: { language: "en" },
  }),
}));

import { OIDCSignInButton } from "@/components/auth/OIDCSignInButton";
import { ThemeProvider } from "@/lib/theme";

async function renderButton() {
  await act(async () => {
    render(createElement(ThemeProvider, null, createElement(OIDCSignInButton, null)));
  });
}

describe("OIDCSignInButton", () => {
  beforeEach(() => vi.clearAllMocks());

  it("labels the button with the provider name from the server", async () => {
    const signIn = vi.fn();
    mockUseOIDCAuth.mockReturnValue({ signIn, isReady: true, name: "Authelia", enabled: true });

    await renderButton();

    expect(screen.getByText("Sign in with Authelia")).toBeTruthy();
    fireEvent.click(screen.getByTestId("signin-oidc"));
    expect(signIn).toHaveBeenCalledTimes(1);
  });

  it("disables the button until the auth request is ready", async () => {
    const signIn = vi.fn();
    mockUseOIDCAuth.mockReturnValue({ signIn, isReady: false, name: "SSO", enabled: true });

    await renderButton();

    // react-native-web maps a disabled Pressable to aria-disabled.
    expect(screen.getByTestId("signin-oidc").getAttribute("aria-disabled")).toBe("true");
  });
});
