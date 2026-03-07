import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { RecommendationCard } from "./RecommendationCard";
import type { Recommendation } from "./RecommendationCard";

const skyscannerRec: Recommendation = {
  partner: "skyscanner",
  category: "flight",
  title: "London to Tokyo flights",
  description: "Find the best deals on flights from London to Tokyo",
  url: "https://skyscanner.net/transport/flights/lond/tyoa/?associateId=toqui",
};

const bookingRec: Recommendation = {
  partner: "booking.com",
  category: "hotel",
  title: "Hotels in Shibuya, Tokyo",
  description: "Browse top-rated hotels in the heart of Shibuya",
  url: "https://booking.com/searchresults.html?dest=Shibuya&aid=toqui",
  price: "From $89/night",
};

const activityRec: Recommendation = {
  partner: "getyourguide",
  category: "activity",
  title: "Mount Fuji Day Trip from Tokyo",
  description: "Full-day guided tour to Mount Fuji with lunch included",
  url: "https://getyourguide.com/tokyo/mt-fuji-tour?partner=toqui",
  price: "$120 per person",
};

describe("RecommendationCard", () => {
  it("renders all fields for a Skyscanner flight recommendation", () => {
    render(<RecommendationCard recommendation={skyscannerRec} />);

    expect(screen.getByText("London to Tokyo flights")).toBeInTheDocument();
    expect(screen.getByText("Find the best deals on flights from London to Tokyo")).toBeInTheDocument();
    expect(screen.getByText("Skyscanner")).toBeInTheDocument();
    expect(screen.getByText("Flight")).toBeInTheDocument();
  });

  it("renders Booking.com hotel recommendation with price", () => {
    render(<RecommendationCard recommendation={bookingRec} />);

    expect(screen.getByText("Hotels in Shibuya, Tokyo")).toBeInTheDocument();
    expect(screen.getByText("Browse top-rated hotels in the heart of Shibuya")).toBeInTheDocument();
    expect(screen.getByText("Booking.com")).toBeInTheDocument();
    expect(screen.getByText("Hotel")).toBeInTheDocument();
    expect(screen.getByText("From $89/night")).toBeInTheDocument();
  });

  it("renders GetYourGuide activity recommendation", () => {
    render(<RecommendationCard recommendation={activityRec} />);

    expect(screen.getByText("Mount Fuji Day Trip from Tokyo")).toBeInTheDocument();
    expect(screen.getByText("GetYourGuide")).toBeInTheDocument();
    expect(screen.getByText("Activity")).toBeInTheDocument();
    expect(screen.getByText("$120 per person")).toBeInTheDocument();
  });

  it("does not render price when not provided", () => {
    render(<RecommendationCard recommendation={skyscannerRec} />);

    // The price element should not exist
    const priceElements = screen.queryByText(/\$/);
    expect(priceElements).not.toBeInTheDocument();
  });

  it("renders CTA link with correct href and rel attributes", () => {
    render(<RecommendationCard recommendation={skyscannerRec} />);

    const link = screen.getByRole("link", { name: /View on Skyscanner/i });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute("href", skyscannerRec.url);
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer sponsored");
  });

  it("renders CTA with correct partner name for Booking.com", () => {
    render(<RecommendationCard recommendation={bookingRec} />);

    const link = screen.getByRole("link", { name: /View on Booking\.com/i });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute("href", bookingRec.url);
    expect(link).toHaveAttribute("rel", "noopener noreferrer sponsored");
  });

  it("renders CTA with correct partner name for GetYourGuide", () => {
    render(<RecommendationCard recommendation={activityRec} />);

    const link = screen.getByRole("link", { name: /View on GetYourGuide/i });
    expect(link).toBeInTheDocument();
    expect(link).toHaveAttribute("href", activityRec.url);
  });

  it("has an accessible article role with descriptive label", () => {
    render(<RecommendationCard recommendation={skyscannerRec} />);

    const article = screen.getByRole("article");
    expect(article).toHaveAttribute(
      "aria-label",
      "Skyscanner recommendation: London to Tokyo flights",
    );
  });

  it("renders affiliate disclosure below the card", () => {
    render(<RecommendationCard recommendation={skyscannerRec} />);

    expect(
      screen.getByText(
        "Toqui may earn a commission from bookings made through these links at no extra cost to you.",
      ),
    ).toBeInTheDocument();
  });

  it("handles an unknown partner gracefully", () => {
    const unknownRec: Recommendation = {
      partner: "newpartner",
      category: "hotel",
      title: "Test Hotel",
      description: "A test description",
      url: "https://example.com",
    };

    render(<RecommendationCard recommendation={unknownRec} />);

    expect(screen.getByText("newpartner")).toBeInTheDocument();
    expect(screen.getByText("Test Hotel")).toBeInTheDocument();
  });
});
