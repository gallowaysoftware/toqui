package chat

import "testing"

func TestImpliesItineraryCreation(t *testing.T) {
	positive := []string{
		// Original Run 4 fabrication phrases
		"All those activities have been added to your itinerary!",
		"I've added those to your itinerary for you.",
		"I've already added the items to your itinerary.",
		"Here are three essential stops added to your itinerary.",
		"Great choices! I've put together a 10-day itinerary, all saved to your itinerary.",
		"Those have been added to the itinerary.",
		"I've saved those to your itinerary.",
		"I've created your itinerary with 14 items.",
		"already been added to your itinerary",

		// Run 5 R-02 / R-11 phrases that slipped past the original list
		"Let's get these fantastic plans officially in your itinerary.",
		"These are now officially added to your trip plan.",
		"Now properly added to your trip plan.",
		"These items are now locked in for your trip.",
		"Everything is locked into your itinerary.",
		"Your 10-day plan is now locked into your trip.",
		"Your itinerary now has all the food stops we discussed.",
		"Your itinerary now includes the Pompeii day trip.",
		"I've built out your itinerary for you.",
		"I've updated your itinerary with the wine tasting.",
	}
	negative := []string{
		"Here are some items you could add to your itinerary.",
		"Let me build out a 10-day plan for you.",
		"I recommend visiting the Colosseum — shall I add it to your itinerary?",
		"Your itinerary is looking great.",
		"Would you like me to add these to your itinerary?",
		// False-positive guards — these mention "locked in" but not "to
		// your itinerary/trip", so fabrication detection must NOT fire.
		"I've locked in your dinner reservation at Osteria.",
		"Your flight is locked in, no changes needed.",
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
