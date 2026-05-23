package handlers

import "testing"

func TestHasExplicitModifyIntent(t *testing.T) {
	// These should all match (MODIFY).
	modify := []string{
		"add that to my itinerary",
		"Add this to my plan for tomorrow",
		"please add to my itinerary",
		"actually, add that temple visit to my itinerary for tomorrow morning",
		"put this on my schedule",
		"save this to my plan",
		"remove from my itinerary the museum visit",
		"delete from my plan the lunch",
		"book this for Tuesday",
		"Can you add to the itinerary?",
		"ADD TO MY PLAN please",
	}

	for _, msg := range modify {
		if !hasExplicitModifyIntent(msg) {
			t.Errorf("expected MODIFY for %q, got INFO", msg)
		}
	}

	// These should NOT match (INFO — should fall through to LLM).
	info := []string{
		"what's a good lunch spot?",
		"recommend a restaurant",
		"is the tram worth riding?",
		"what should I do tonight?",
		"suggest something fun",
		"how do I get to the museum?",
		"what's the tipping etiquette?",
		"tell me about the history of this temple",
		"where should I eat?",
		"what's nearby?",
		"what's the address of the temple?",
		"how much does it cost?",
		"",
	}

	for _, msg := range info {
		if hasExplicitModifyIntent(msg) {
			t.Errorf("expected INFO for %q, got MODIFY", msg)
		}
	}
}
