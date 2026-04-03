import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ShareNudgeBanner } from "../ShareNudgeBanner";

vi.mock("lucide-react-native", () => ({
  Share2: () => <span data-testid="share2-icon" />,
  X: () => <span data-testid="x-icon" />,
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const strings: Record<string, string> = {
        "share.nudgeBanner": "Share this trip with friends \u2192",
        "common.cancel": "Cancel",
      };
      return strings[key] ?? key;
    },
  }),
}));

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      surface: "#ffffff",
      surfaceSecondary: "#f9fafb",
      textPrimary: "#111827",
      textSecondary: "#4b5563",
      textTertiary: "#5f6673",
      accent: "#e8654a",
      accentSoft: "#fef2f0",
      border: "#e5e7eb",
    },
  }),
}));

describe("ShareNudgeBanner", () => {
  it("renders the nudge text", () => {
    render(<ShareNudgeBanner onShare={vi.fn()} onDismiss={vi.fn()} />);
    expect(screen.getByText("Share this trip with friends \u2192")).toBeTruthy();
  });

  it("calls onShare when the text is tapped", () => {
    const onShare = vi.fn();
    render(<ShareNudgeBanner onShare={onShare} onDismiss={vi.fn()} />);
    // The text is wrapped in a Pressable
    fireEvent.click(screen.getByText("Share this trip with friends \u2192"));
    expect(onShare).toHaveBeenCalledTimes(1);
  });

  it("calls onDismiss when the dismiss button is tapped", () => {
    const onDismiss = vi.fn();
    render(<ShareNudgeBanner onShare={vi.fn()} onDismiss={onDismiss} />);
    const dismissButton = screen.getByLabelText("Cancel");
    fireEvent.click(dismissButton);
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("has the correct test ID", () => {
    render(<ShareNudgeBanner onShare={vi.fn()} onDismiss={vi.fn()} />);
    expect(screen.getByTestId("share-nudge-banner")).toBeTruthy();
  });
});
