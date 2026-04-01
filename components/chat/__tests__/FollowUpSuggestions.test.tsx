import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import {
  generateFollowUps,
  generateCompanionFollowUps,
  FollowUpSuggestions,
} from "../FollowUpSuggestions";

vi.mock("@/lib/theme", () => ({
  useTheme: () => ({
    colors: {
      surface: "#fff",
      accent: "#BF4028",
    },
  }),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        "chat.followUp.localDishes": "Best local dishes to try",
        "chat.followUp.reservationTips": "Restaurant reservation tips",
        "chat.followUp.howLongToSpend": "How long should I spend there?",
        "chat.followUp.whatsNearby": "What's nearby?",
        "chat.followUp.addMoreActivities": "Add more activities",
        "chat.followUp.adjustPace": "Adjust the pace",
        "chat.followUp.alternativeOptions": "Alternative options",
        "chat.followUp.findFlights": "Help me find flights",
        "chat.followUp.recommendHotels": "Recommend hotels",
        "chat.followUp.createDayByDay": "Create a day-by-day plan",
        "chat.followUp.mustSees": "What are the must-sees?",
        "chat.followUp.whatElse": "What else should I know?",
        "chat.followUp.localTips": "Local tips",
        "chat.followUp.packingSuggestions": "Packing suggestions",
        "chat.followUp.findCoffeeShop": "Find a coffee shop",
        "chat.followUp.navigateHotel": "Navigate to my hotel",
        "chat.followUp.translateSomething": "Translate something for me",
      };
      return map[key] ?? key;
    },
  }),
}));

describe("generateFollowUps", () => {
  it("returns food-related suggestions when message mentions restaurants", () => {
    const keys = generateFollowUps("Here are some great restaurants in Tokyo");
    expect(keys).toContain("localDishes");
    expect(keys).toContain("reservationTips");
  });

  it("returns food-related suggestions when message mentions food", () => {
    const keys = generateFollowUps("The local food scene is amazing with street food everywhere");
    expect(keys).toContain("localDishes");
  });

  it("returns attraction-related suggestions when message mentions museums", () => {
    const keys = generateFollowUps("The Louvre Museum is a must-visit attraction");
    expect(keys).toContain("howLongToSpend");
    expect(keys).toContain("whatsNearby");
  });

  it("returns attraction-related suggestions for temples", () => {
    const keys = generateFollowUps("Visit the ancient temple in Kyoto");
    expect(keys).toContain("howLongToSpend");
  });

  it("returns itinerary-related suggestions when message mentions itinerary", () => {
    const keys = generateFollowUps("Here is your day 1 itinerary with morning activities");
    expect(keys).toContain("addMoreActivities");
    expect(keys).toContain("adjustPace");
  });

  it("returns itinerary suggestions for schedule keywords", () => {
    const keys = generateFollowUps("In the morning you can visit the park, afternoon at the beach");
    expect(keys).toContain("addMoreActivities");
  });

  it("returns booking suggestions when trip has no bookings", () => {
    const keys = generateFollowUps("Welcome! I can help with your trip.", {
      hasBookings: false,
    });
    expect(keys).toContain("findFlights");
    expect(keys).toContain("recommendHotels");
  });

  it("returns itinerary creation suggestions when trip has no itinerary", () => {
    const keys = generateFollowUps("Welcome to your trip planning!", {
      hasItinerary: false,
    });
    expect(keys).toContain("createDayByDay");
    expect(keys).toContain("mustSees");
  });

  it("returns fallback suggestions when no keywords match", () => {
    const keys = generateFollowUps("I hope that helps! Let me know if you have other questions.");
    expect(keys).toEqual(["whatElse", "localTips", "packingSuggestions"]);
  });

  it("returns at most 3 suggestions", () => {
    const keys = generateFollowUps(
      "Visit the museum and try the restaurant food near the temple",
    );
    expect(keys.length).toBeLessThanOrEqual(3);
  });

  it("does not return duplicates", () => {
    const keys = generateFollowUps("restaurant food dining cuisine dishes");
    const unique = new Set(keys);
    expect(unique.size).toBe(keys.length);
  });
});

describe("generateCompanionFollowUps", () => {
  it("returns location-aware suggestions when location is active", () => {
    const keys = generateCompanionFollowUps(true);
    expect(keys).toContain("whatsNearby");
    expect(keys).toContain("findCoffeeShop");
    expect(keys).toContain("translateSomething");
  });

  it("returns default companion suggestions when no location", () => {
    const keys = generateCompanionFollowUps(false);
    expect(keys).toContain("findCoffeeShop");
    expect(keys).toContain("navigateHotel");
    expect(keys).toContain("translateSomething");
  });
});

describe("FollowUpSuggestions component", () => {
  it("renders suggestion chips based on message content", () => {
    render(
      <FollowUpSuggestions
        lastAssistantMessage="Check out this great restaurant"
        onSelect={vi.fn()}
      />,
    );
    expect(screen.getByText("Best local dishes to try")).toBeInTheDocument();
    expect(screen.getByText("Restaurant reservation tips")).toBeInTheDocument();
  });

  it("calls onSelect with the suggestion label when tapped", () => {
    const onSelect = vi.fn();
    render(
      <FollowUpSuggestions
        lastAssistantMessage="Check out this great restaurant"
        onSelect={onSelect}
      />,
    );
    fireEvent.click(screen.getByText("Best local dishes to try"));
    expect(onSelect).toHaveBeenCalledWith("Best local dishes to try");
  });

  it("renders fallback suggestions for generic messages", () => {
    render(
      <FollowUpSuggestions
        lastAssistantMessage="Sure, I can help with that!"
        onSelect={vi.fn()}
      />,
    );
    expect(screen.getByText("What else should I know?")).toBeInTheDocument();
    expect(screen.getByText("Local tips")).toBeInTheDocument();
    expect(screen.getByText("Packing suggestions")).toBeInTheDocument();
  });

  it("renders companion mode suggestions with location", () => {
    render(
      <FollowUpSuggestions
        lastAssistantMessage="Here are some nearby places"
        onSelect={vi.fn()}
        mode="companion"
        hasLocation
      />,
    );
    expect(screen.getByText("What's nearby?")).toBeInTheDocument();
    expect(screen.getByText("Find a coffee shop")).toBeInTheDocument();
    expect(screen.getByText("Translate something for me")).toBeInTheDocument();
  });

  it("renders companion mode suggestions without location", () => {
    render(
      <FollowUpSuggestions
        lastAssistantMessage="I can help you navigate"
        onSelect={vi.fn()}
        mode="companion"
        hasLocation={false}
      />,
    );
    expect(screen.getByText("Find a coffee shop")).toBeInTheDocument();
    expect(screen.getByText("Navigate to my hotel")).toBeInTheDocument();
    expect(screen.getByText("Translate something for me")).toBeInTheDocument();
  });

  it("sets accessibility role on chips", () => {
    render(
      <FollowUpSuggestions
        lastAssistantMessage="Check out this restaurant"
        onSelect={vi.fn()}
      />,
    );
    const buttons = screen.getAllByRole("button");
    expect(buttons.length).toBeGreaterThanOrEqual(2);
  });
});
