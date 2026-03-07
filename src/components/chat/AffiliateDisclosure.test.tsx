import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { AffiliateDisclosure } from "./AffiliateDisclosure";

describe("AffiliateDisclosure", () => {
  it("renders the FTC disclosure text", () => {
    render(<AffiliateDisclosure />);

    expect(
      screen.getByText(
        "Toqui may earn a commission from bookings made through these links at no extra cost to you.",
      ),
    ).toBeInTheDocument();
  });

  it("has an accessible note role with label", () => {
    render(<AffiliateDisclosure />);

    const note = screen.getByRole("note");
    expect(note).toBeInTheDocument();
    expect(note).toHaveAttribute("aria-label", "Affiliate disclosure");
  });

  it("renders the disclosure text in a paragraph element", () => {
    render(<AffiliateDisclosure />);

    const text = screen.getByText(/Toqui may earn a commission/);
    expect(text.tagName).toBe("P");
  });
});
