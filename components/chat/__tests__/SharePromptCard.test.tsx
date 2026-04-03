import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SharePromptCard } from "../SharePromptCard";

vi.mock("lucide-react-native", () => ({
  Share2: () => <span data-testid="share2-icon" />,
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const strings: Record<string, string> = {
        "share.promptTitle": "Your itinerary is ready!",
        "share.promptSubtitle": "Share it with your travel companions",
        "referral.share": "Share",
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

describe("SharePromptCard", () => {
  it("renders the prompt title and subtitle", () => {
    render(<SharePromptCard onShare={vi.fn()} />);
    expect(screen.getByText("Your itinerary is ready!")).toBeTruthy();
    expect(screen.getByText("Share it with your travel companions")).toBeTruthy();
  });

  it("renders the share button", () => {
    render(<SharePromptCard onShare={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Share" })).toBeTruthy();
  });

  it("calls onShare when the share button is pressed", () => {
    const onShare = vi.fn();
    render(<SharePromptCard onShare={onShare} />);
    fireEvent.click(screen.getByRole("button", { name: "Share" }));
    expect(onShare).toHaveBeenCalledTimes(1);
  });

  it("renders the share icon", () => {
    render(<SharePromptCard onShare={vi.fn()} />);
    expect(screen.getByTestId("share2-icon")).toBeTruthy();
  });

  it("disables the button when isSharing is true", () => {
    render(<SharePromptCard onShare={vi.fn()} isSharing />);
    const button = screen.getByRole("button", { name: "Share" });
    expect(button).toBeDisabled();
  });

  it("has the correct test ID", () => {
    render(<SharePromptCard onShare={vi.fn()} />);
    expect(screen.getByTestId("share-prompt-card")).toBeTruthy();
  });
});
