import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act, waitFor } from "@testing-library/react";
import { createElement } from "react";
import en from "@/messages/en.json";
import { Code, ConnectError } from "@connectrpc/connect";

// ---------- mocks ----------

const mockReplace = vi.fn();
const mockRegisterWithEmail = vi.fn();

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
  useAuth: () => ({ registerWithEmail: mockRegisterWithEmail }),
}));

import EmailRegisterScreen from "@/app/auth/email-register";
import { ThemeProvider } from "@/lib/theme";

function renderScreen() {
  return render(createElement(ThemeProvider, null, createElement(EmailRegisterScreen)));
}

beforeEach(() => {
  mockReplace.mockClear();
  mockRegisterWithEmail.mockClear();
});

describe("EmailRegisterScreen", () => {
  it("renders the form", async () => {
    await act(async () => { renderScreen(); });
    expect(screen.getByTestId("email-register-title")).toBeInTheDocument();
    expect(screen.getByTestId("email-register-name")).toBeInTheDocument();
    expect(screen.getByTestId("email-register-email")).toBeInTheDocument();
    expect(screen.getByTestId("email-register-password")).toBeInTheDocument();
    expect(screen.getByTestId("email-register-submit")).toBeInTheDocument();
  });

  it("registers successfully and redirects to /(tabs)", async () => {
    mockRegisterWithEmail.mockResolvedValueOnce(undefined);

    await act(async () => { renderScreen(); });

    await act(async () => {
      fireEvent.change(screen.getByTestId("email-register-name"), {
        target: { value: "Alice" },
      });
      fireEvent.change(screen.getByTestId("email-register-email"), {
        target: { value: "alice@x.com" },
      });
      fireEvent.change(screen.getByTestId("email-register-password"), {
        target: { value: "supersecret123" },
      });
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("email-register-submit"));
    });

    await waitFor(() => {
      expect(mockRegisterWithEmail).toHaveBeenCalledWith(
        "alice@x.com",
        "supersecret123",
        "Alice",
      );
      expect(mockReplace).toHaveBeenCalledWith("/(tabs)");
    });
  });

  it("shows the duplicate-email message on AlreadyExists", async () => {
    mockRegisterWithEmail.mockRejectedValueOnce(
      new ConnectError("duplicate", Code.AlreadyExists),
    );

    await act(async () => { renderScreen(); });

    await act(async () => {
      fireEvent.change(screen.getByTestId("email-register-name"), {
        target: { value: "Alice" },
      });
      fireEvent.change(screen.getByTestId("email-register-email"), {
        target: { value: "taken@x.com" },
      });
      fireEvent.change(screen.getByTestId("email-register-password"), {
        target: { value: "supersecret123" },
      });
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("email-register-submit"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("email-register-error")).toHaveTextContent(
        en.auth.register.errors.alreadyExists,
      );
    });
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("shows the weak-password / invalid-argument message on InvalidArgument", async () => {
    mockRegisterWithEmail.mockRejectedValueOnce(
      new ConnectError("password too short", Code.InvalidArgument),
    );

    await act(async () => { renderScreen(); });

    await act(async () => {
      fireEvent.change(screen.getByTestId("email-register-name"), {
        target: { value: "Bob" },
      });
      fireEvent.change(screen.getByTestId("email-register-email"), {
        target: { value: "bob@x.com" },
      });
      fireEvent.change(screen.getByTestId("email-register-password"), {
        target: { value: "short" },
      });
    });

    await act(async () => {
      fireEvent.click(screen.getByTestId("email-register-submit"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("email-register-error")).toHaveTextContent(
        en.auth.register.errors.invalidArgument,
      );
    });
  });

  it("requires all three fields before calling the RPC", async () => {
    await act(async () => { renderScreen(); });

    await act(async () => {
      fireEvent.click(screen.getByTestId("email-register-submit"));
    });

    expect(mockRegisterWithEmail).not.toHaveBeenCalled();
    expect(screen.getByTestId("email-register-error")).toHaveTextContent(
      en.auth.register.errors.missingFields,
    );
  });
});
