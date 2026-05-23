package booking

import (
	"encoding/json"
	"testing"
)

func TestNormalizeBookingType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Existing types
		{"flight", "flight"},
		{"hotel", "hotel"},
		{"car_rental", "car_rental"},
		{"train", "train"},
		{"tour", "tour"},
		{"activity", "activity"},
		{"restaurant", "restaurant"},
		{"other", "other"},
		// New types
		{"ferry", "ferry"},
		{"bus", "bus"},
		{"cruise", "cruise"},
		{"transfer", "transfer"},
		{"vacation_rental", "vacation_rental"},
		{"VACATION_RENTAL", "vacation_rental"},
		{" vacation_rental ", "vacation_rental"},
		// Case normalization
		{"FERRY", "ferry"},
		{"Bus", "bus"},
		{"CRUISE", "cruise"},
		{"Transfer", "transfer"},
		{"Flight", "flight"},
		// Whitespace
		{" ferry ", "ferry"},
		{" bus\t", "bus"},
		// Unknown types map to "other"
		{"helicopter", "other"},
		{"taxi", "other"},
		{"", "other"},
		{"segway", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeBookingType(tt.input)
			if got != tt.want {
				t.Errorf("normalizeBookingType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripCodeFences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no fences",
			input: `{"type":"ferry"}`,
			want:  `{"type":"ferry"}`,
		},
		{
			name:  "json fences",
			input: "```json\n{\"type\":\"bus\"}\n```",
			want:  `{"type":"bus"}`,
		},
		{
			name:  "plain fences",
			input: "```\n{\"type\":\"cruise\"}\n```",
			want:  `{"type":"cruise"}`,
		},
		{
			name:  "with surrounding whitespace",
			input: "  ```json\n{\"type\":\"transfer\"}\n```  ",
			want:  `{"type":"transfer"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripCodeFences(tt.input)
			if got != tt.want {
				t.Errorf("stripCodeFences(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParsedBooking_PriceFields(t *testing.T) {
	input := `{
		"type": "ferry",
		"confirmation_code": "BCF-123456",
		"provider": "BC Ferries",
		"title": "Tsawwassen to Swartz Bay Ferry",
		"start_time": "2026-07-15T09:00:00-07:00",
		"end_time": "2026-07-15T10:35:00-07:00",
		"departure_location": "Tsawwassen",
		"arrival_location": "Swartz Bay",
		"num_guests": 2,
		"price_cents": 11750,
		"currency": "CAD",
		"timezone": "America/Vancouver",
		"details": {"operator":"BC Ferries","vessel_name":"Spirit of Vancouver Island","departure_port":"Tsawwassen","arrival_port":"Swartz Bay","num_passengers":2,"vehicle_included":true}
	}`

	var parsed ParsedBooking
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		t.Fatalf("unmarshal ParsedBooking: %v", err)
	}

	if parsed.Type != "ferry" {
		t.Errorf("Type: got %q, want %q", parsed.Type, "ferry")
	}
	if parsed.PriceCents != 11750 {
		t.Errorf("PriceCents: got %d, want 11750", parsed.PriceCents)
	}
	if parsed.Currency != "CAD" {
		t.Errorf("Currency: got %q, want %q", parsed.Currency, "CAD")
	}
	if parsed.Timezone != "America/Vancouver" {
		t.Errorf("Timezone: got %q, want %q", parsed.Timezone, "America/Vancouver")
	}
	if parsed.ConfirmationCode != "BCF-123456" {
		t.Errorf("ConfirmationCode: got %q, want %q", parsed.ConfirmationCode, "BCF-123456")
	}
	if parsed.NumGuests != 2 {
		t.Errorf("NumGuests: got %d, want 2", parsed.NumGuests)
	}
}

func TestParsedBooking_BusWithPrice(t *testing.T) {
	input := `{
		"type": "bus",
		"confirmation_code": "FLX-789012",
		"provider": "FlixBus",
		"title": "Barcelona to Madrid Bus",
		"start_time": "2026-08-01T07:30:00+02:00",
		"end_time": "2026-08-01T15:00:00+02:00",
		"departure_location": "Barcelona",
		"arrival_location": "Madrid",
		"num_guests": 1,
		"price_cents": 2990,
		"currency": "EUR",
		"timezone": "Europe/Madrid",
		"details": {"operator":"FlixBus","route_number":"N740","departure_station":"Barcelona Nord","arrival_station":"Madrid Estacion Sur","seat":"12A","class":"Standard"}
	}`

	var parsed ParsedBooking
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		t.Fatalf("unmarshal ParsedBooking: %v", err)
	}

	if parsed.Type != "bus" {
		t.Errorf("Type: got %q, want %q", parsed.Type, "bus")
	}
	if parsed.PriceCents != 2990 {
		t.Errorf("PriceCents: got %d, want 2990", parsed.PriceCents)
	}
	if parsed.Currency != "EUR" {
		t.Errorf("Currency: got %q, want %q", parsed.Currency, "EUR")
	}
	if parsed.Timezone != "Europe/Madrid" {
		t.Errorf("Timezone: got %q, want %q", parsed.Timezone, "Europe/Madrid")
	}
}

func TestParsedBooking_NoPriceFields(t *testing.T) {
	// Existing bookings without price/currency/timezone should still parse.
	input := `{
		"type": "flight",
		"confirmation_code": "ABC123",
		"provider": "Delta",
		"title": "JFK to BCN",
		"start_time": "2026-06-15T18:45:00",
		"departure_location": "JFK",
		"arrival_location": "BCN",
		"num_guests": 1,
		"details": {"airline":"Delta","flight_number":"DL472"}
	}`

	var parsed ParsedBooking
	if err := json.Unmarshal([]byte(input), &parsed); err != nil {
		t.Fatalf("unmarshal ParsedBooking: %v", err)
	}

	if parsed.PriceCents != 0 {
		t.Errorf("PriceCents: got %d, want 0", parsed.PriceCents)
	}
	if parsed.Currency != "" {
		t.Errorf("Currency: got %q, want empty", parsed.Currency)
	}
	if parsed.Timezone != "" {
		t.Errorf("Timezone: got %q, want empty", parsed.Timezone)
	}
}
