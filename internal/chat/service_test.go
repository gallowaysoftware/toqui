package chat

import (
	"testing"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
)

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

// TestUserRequestsItineraryCreation locks in the user-intent fabrication
// detector. The Run 6 retrospective showed that the response-text-based
// detector misses the broader "AI describes a plan and stops" failure
// mode (R-05, R-16, N-06, N-07, N-12, N-16). This test pins the user
// utterances we expect to fire the retry.
func TestUserRequestsItineraryCreation(t *testing.T) {
	positive := []string{
		// Direct tool name mentions (must be alongside an action verb)
		"Please call create_itinerary_items with the following items",
		"Use create_itinerary_items to add these",

		// Verb + structured noun
		"Build me a 10-day itinerary for Italy",
		"Build me an itinerary covering all the cities",
		"Build out my itinerary please",
		"Build out the itinerary for my Cusco days",
		"Build a day-by-day itinerary",
		"Create a 7-day itinerary for Iceland",
		"Create my itinerary now",
		"Give me a day-by-day itinerary for Peru",
		"Give me a complete itinerary",
		"Give me a detailed itinerary",
		"Plan me a 14-day trip",
		"Plan a 5-day Tokyo schedule",
		"Plan a day-by-day food trip",

		// Add patterns
		"Add to my itinerary the Pompeii day trip",
		"Add it to my itinerary please",
		"Add these to my itinerary",
		"Add to day 3 the Sacred Valley tour",
		"Add a day trip to Hakone",

		// Structured noun phrases
		"I want a day-by-day itinerary",
		"Give me the full itinerary",
		"What does my detailed itinerary look like",
	}
	negative := []string{
		// Questions about existing trips
		"What's the weather in Lisbon today?",
		"Where should I eat near Khao San Road?",
		"How do I get from Delhi to Agra?",
		"Tell me about visiting Knossos",

		// Discussion that could be part of planning but doesn't ask for items
		"I like the sound of Pompeii — what else is nearby?",
		"That sounds great",
		"Thanks for the recommendation",

		// Negation guards (review W1)
		"Don't add to my itinerary — just tell me about it",
		"Don't add it to my itinerary",
		"Do not build the itinerary yet",
		"Stop adding things to my itinerary",
		"Forget the create_itinerary_items call, just explain it to me",
		"Cancel the itinerary I was building",

		// Non-itinerary "build me" contexts (review W1)
		"Could you build me a packing list?",
		"Build me a budget breakdown for the trip",
		"Build me a list of vocab words to learn",

		// Error/discussion contexts
		"The create_itinerary_items error is weird, can you check it?",

		"",
	}

	for _, text := range positive {
		if !userRequestsItineraryCreation(text) {
			t.Errorf("expected userRequestsItineraryCreation(%q) = true", text)
		}
	}
	for _, text := range negative {
		if userRequestsItineraryCreation(text) {
			t.Errorf("expected userRequestsItineraryCreation(%q) = false", text)
		}
	}
}

// TestImpliesExpertHandoff documents the new suggest_expert fabrication
// detector added in Run 7. R-16 / N-07 / N-12 in Run 6 all hit the
// pattern where the AI says "let me bring in a specialist" without
// actually firing suggest_expert.
func TestImpliesExpertHandoff(t *testing.T) {
	positive := []string{
		"Let me bring in a specialist for this",
		"Let me bring in our local expert",
		"I'll bring in our specialist on craft beer",
		"I'll hand you off to our food specialist",
		"I'll hand this off to our specialist",
		"I'm going to bring in a specialist",
		"I'll connect you with our specialist",
		"Let me connect you with our expert",
		"The expert here is Maria — let me grab her",
		"Our specialist on Mexican cuisine can take this",
		"Handing you off to our expert",
	}
	negative := []string{
		// Self-identification, not a handoff
		"I'm an expert in this area myself",
		"You should consult a local expert when you arrive",
		// Innocuous "bring in" / "connect" usage (review W4)
		"Let me bring in some examples",
		"I'll bring in my suitcase",
		"I'll connect you with the restaurant directly",
		"I'll connect you with the hotel concierge",
		"That bridge was built in 1850",
		"",
	}
	for _, text := range positive {
		if !impliesExpertHandoff(text) {
			t.Errorf("expected impliesExpertHandoff(%q) = true", text)
		}
	}
	for _, text := range negative {
		if impliesExpertHandoff(text) {
			t.Errorf("expected impliesExpertHandoff(%q) = false", text)
		}
	}
}

// TestMostRecentUserContent verifies the helper skips system-injected
// nudges (which would otherwise hide the user's real intent across
// fabrication retries) and falls through to the genuine user message.
func TestMostRecentUserContent(t *testing.T) {
	cases := []struct {
		name     string
		messages []ai.Message
		want     string
	}{
		{
			name:     "empty",
			messages: nil,
			want:     "",
		},
		{
			name: "single user message",
			messages: []ai.Message{
				{Role: "user", Content: "build me an itinerary for Peru"},
			},
			want: "build me an itinerary for Peru",
		},
		{
			name: "skips system-note retry nudges",
			messages: []ai.Message{
				{Role: "user", Content: "build me an itinerary for Peru"},
				{Role: "assistant", Content: "Here's a plan..."},
				{Role: "user", Content: "(System note: your last turn produced no output. Please answer my previous message.)"},
			},
			want: "build me an itinerary for Peru",
		},
		{
			name: "skips tool-result-only user messages",
			messages: []ai.Message{
				{Role: "user", Content: "what should I add to day 3"},
				{Role: "assistant", Content: "Let me check"},
				{Role: "user", ToolResults: []ai.ToolResult{{ToolCallID: "x", Name: "web_search", Content: "{}"}}},
			},
			want: "what should I add to day 3",
		},
		{
			name: "returns most recent real user message",
			messages: []ai.Message{
				{Role: "user", Content: "first message"},
				{Role: "assistant", Content: "ok"},
				{Role: "user", Content: "second message"},
			},
			want: "second message",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mostRecentUserContent(c.messages); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestStripRetryArtifacts(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no artifacts — unchanged",
			input: "Here's your 10-day Italy itinerary! Day 1 starts in Rome.",
			want:  "Here's your 10-day Italy itinerary! Day 1 starts in Rome.",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "strips 'I actually did call' sentence",
			input: "I actually did call create_itinerary_items in my previous response. Here's your plan for Peru!",
			want:  "Here's your plan for Peru!",
		},
		{
			name:  "strips 'in my previous response' sentence",
			input: "Great itinerary! In my previous response, I already saved these items. Let me know if you want changes.",
			want:  "Great itinerary! Let me know if you want changes.",
		},
		{
			name:  "strips multiple artifact sentences",
			input: "I called create_itinerary_items with all the items. As mentioned earlier, everything is saved. Here are your highlights.",
			want:  "Here are your highlights.",
		},
		{
			name:  "preserves normal text with 'call' in it",
			input: "Call the restaurant to make a reservation. The number is on their website.",
			want:  "Call the restaurant to make a reservation. The number is on their website.",
		},
		{
			name:  "all-artifact text returns original",
			input: "I actually did call create_itinerary_items in my previous response.",
			want:  "I actually did call create_itinerary_items in my previous response.",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripRetryArtifacts(c.input)
			if got != c.want {
				t.Errorf("stripRetryArtifacts(%q)\n  got:  %q\n  want: %q", c.input, got, c.want)
			}
		})
	}
}
