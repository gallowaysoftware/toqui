import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act, waitFor } from "@testing-library/react";
import { createElement } from "react";
import en from "@/messages/en.json";
import { Code, ConnectError } from "@connectrpc/connect";

// ---------- mocks ----------

const mockReplace = vi.fn();
const mockLoginWithEmail = vi.fn();

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

vi.mock("expo-router", () => ({
  useRouter: () => ({ replace: mockReplace, push: vi.fn(), back: vi.fn() }),
  Link: ({ children, href, testID }: { children: React.ReactNode; href: string; testID?: string }) =>
    createElement("a", { href, "data-testid": testID }, children),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const parts = key.split(".");
      let val: unknown = en;
      for (const p of parts) val = (val as Record<string, unknown>)?.[p];
      return typeof val === "string" ? val : key;
    },
    i18n: { language: "en" },
  }),
}));

vi.mock("@/lib/auth", () => ({
  useAuth: () => ({ loginWithEmail: mockLoginWithEmail }),
}));

import EmailLoginScreen from "@/app/auth/email-login";
import { ThemeProvider } from "@/lib/theme";

function renderScreen() {
  return render(createElement(ThemeProvider, null, createElement(EmailLoginScreen)));
}

beforeEach(() => {
  mockReplace.mockClear();
  mockLoginWithEmail.mockClear();
});

describe("EmailLoginScreen", () => {
  it("renders the form", async () => {
    await act(async () => { renderScreen(); });
    expect(screen.getByTestId("email-login-title")).toBeInTheDocument();
    expect(screen.getByTestId("email-login-email")).toBeInTheDocument();
    expect(screen.getByTestId("email-login-password")).toBeInTheDocument();
    expect(screen.getByTestId("email-login-submit")).toBeInTheDocument();
  });

  it("logs in successfully and redirects to /(tabs)", async () => {
    mockLoginWithEmail.mockResolvedValueOnce(undefined);

    await act(async () => { renderScreen(); });

    await act(async () => {
      fireEvent.change(screen.getByTestId("email-login-email"), {
        target: { value: "user@x.com" },
      });
      fireEvent.change(screen.getByTestId("email-login-password"), {
        target: { value: "supersecret123" },
      });
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("email-login-submit"));
    });

    await waitFor(() => {
      expect(mockLoginWithEmail).toHaveBeenCalledWith("user@x.com", "supersecret123");
      expect(mockReplace).toHaveBeenCalledWith("/(tabs)");
    });
  });

  it("shows the invalid-credentials message on Unauthenticated", async () => {
    mockLoginWithEmail.mockRejectedValueOnce(
      new ConnectError("bad creds", Code.Unauthenticated),
    );

    await act(async () => { renderScreen(); });

    await act(async () => {
      fireEvent.change(screen.getByTestId("email-login-email"), {
        target: { value: "user@x.com" },
      });
      fireEvent.change(screen.getByTestId("email-login-password"), {
        target: { value: "wrong-password" },
      });
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("email-login-submit"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("email-login-error")).toHaveTextContent(
        en.auth.login.errors.invalidCredentials,
      );
    });
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("requires both email and password before calling the RPC", async () => {
    await act(async () => { renderScreen(); });

    await act(async () => {
      fireEvent.click(screen.getByTestId("email-login-submit"));
    });

    expect(mockLoginWithEmail).not.toHaveBeenCalled();
    expect(screen.getByTestId("email-login-error")).toHaveTextContent(
      en.auth.login.errors.missingFields,
    );
  });

  it("shows the generic error on unknown failures", async () => {
    mockLoginWithEmail.mockRejectedValueOnce(new Error("network exploded"));

    await act(async () => { renderScreen(); });

    await act(async () => {
      fireEvent.change(screen.getByTestId("email-login-email"), {
        target: { value: "u@x.com" },
      });
      fireEvent.change(screen.getByTestId("email-login-password"), {
        target: { value: "supersecret123" },
      });
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("email-login-submit"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("email-login-error")).toHaveTextContent(
        en.auth.login.errors.generic,
      );
    });
  });
});
