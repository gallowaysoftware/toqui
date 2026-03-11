import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MessageLimitBanner } from "./MessageLimitBanner";
import type { UsageInfo } from "@/lib/hooks/useUsage";

function makeUsage(overrides: Partial<UsageInfo> = {}): UsageInfo {
  return {
    used: 10,
    limit: 30,
    remaining: 20,
    isAtLimit: false,
    isWarning: false,
    recordMessage: vi.fn(),
    markExhausted: vi.fn(),
    ...overrides,
  };
}

describe("MessageLimitBanner", () => {
  it("shows remaining message count in normal state", () => {
    render(<MessageLimitBanner usage={makeUsage({ remaining: 20, limit: 30 })} />);
    expect(screen.getByText("20 of 30 messages remaining today")).toBeInTheDocument();
  });

  it("has status role in normal state", () => {
    render(<MessageLimitBanner usage={makeUsage()} />);
    expect(screen.getByRole("status")).toBeInTheDocument();
  });

  it("shows warning styling when approaching limit", () => {
    const { container } = render(
      <MessageLimitBanner usage={makeUsage({ remaining: 3, isWarning: true })} />,
    );
    const statusEl = container.querySelector("[role='status']");
    expect(statusEl?.className).toContain("warning-text");
  });

  it("shows limit reached banner with coming soon button", () => {
    render(<MessageLimitBanner usage={makeUsage({ remaining: 0, isAtLimit: true })} />);
    expect(screen.getByText("Daily message limit reached")).toBeInTheDocument();
    expect(screen.getByText("Coming Soon")).toBeInTheDocument();
    expect(screen.getByText("Your message limit resets daily at midnight.")).toBeInTheDocument();
  });

  it("shows coming soon message when button is clicked", () => {
    render(<MessageLimitBanner usage={makeUsage({ remaining: 0, isAtLimit: true })} />);
    fireEvent.click(screen.getByText("Coming Soon"));
    expect(
      screen.getByText("Trip Pro is coming soon! Your message limit will reset tomorrow."),
    ).toBeInTheDocument();
  });

  it("has alert role when at limit", () => {
    render(<MessageLimitBanner usage={makeUsage({ remaining: 0, isAtLimit: true })} />);
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });

  it("does not show coming soon button when not at limit", () => {
    render(<MessageLimitBanner usage={makeUsage()} />);
    expect(screen.queryByText("Coming Soon")).not.toBeInTheDocument();
  });
});
