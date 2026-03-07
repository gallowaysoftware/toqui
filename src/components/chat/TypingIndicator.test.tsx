import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { TypingIndicator } from "./TypingIndicator";

describe("TypingIndicator", () => {
  it("renders without crashing", () => {
    const { container } = render(<TypingIndicator />);
    expect(container).toBeTruthy();
  });

  it("renders three animated dots", () => {
    const { container } = render(<TypingIndicator />);
    const dots = container.querySelectorAll(".animate-bounce");
    expect(dots).toHaveLength(3);
  });

  it("has staggered animation delays", () => {
    const { container } = render(<TypingIndicator />);
    const dots = container.querySelectorAll(".animate-bounce");
    expect(dots[0].style.animationDelay).toBe("0ms");
    expect(dots[1].style.animationDelay).toBe("150ms");
    expect(dots[2].style.animationDelay).toBe("300ms");
  });
});
