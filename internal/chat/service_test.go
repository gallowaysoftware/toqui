package chat

import "testing"

func TestImpliesItineraryCreation(t *testing.T) {
	positive := []string{
		"All those activities have been added to your itinerary!",
		"I've added those to your itinerary for you.",
		"I've already added the items to your itinerary.",
		"Here are three essential stops added to your itinerary.",
		"Great choices! I've put together a 10-day itinerary, all saved to your itinerary.",
		"Those have been added to the itinerary.",
		"I've saved those to your itinerary.",
		"I've created your itinerary with 14 items.",
		"already been added to your itinerary",
	}
	negative := []string{
		"Here are some items you could add to your itinerary.",
		"Let me build out a 10-day plan for you.",
		"I recommend visiting the Colosseum — shall I add it to your itinerary?",
		"Your itinerary is looking great.",
		"",
	}

	for _, text := range positive {
		if !impliesItineraryCreation(text) {
			t.Errorf("expected true for: %q", text)
		}
	}
	for _, text := range negative {
		if impliesItineraryCreation(text) {
			t.Errorf("expected false for: %q", text)
		}
	}
}
