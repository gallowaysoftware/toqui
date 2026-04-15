package booking

import (
	"encoding/json"
	"testing"
)

func TestFlightDetails_JSONRoundTrip_WithLegs(t *testing.T) {
	original := FlightDetails{
		Airline:           "Delta",
		FlightNumber:      "DL472",
		DepartureAirport:  "JFK",
		ArrivalAirport:    "BCN",
		DepartureTerminal: "4",
		ArrivalTerminal:   "1",
		Seat:              "27F",
		CabinClass:        "Economy",
		Passengers:        []string{"MARTINEZ/ELENA"},
		Legs: []FlightLeg{
			{
				FlightNumber:     "DL472",
				Airline:          "Delta",
				DepartureAirport: "JFK",
				ArrivalAirport:   "BCN",
				DepartureTime:    "2026-06-15T18:45:00-04:00",
				ArrivalTime:      "2026-06-16T08:55:00+02:00",
				Cabin:            "Economy",
			},
			{
				FlightNumber:     "DL149",
				Airline:          "Delta",
				DepartureAirport: "BCN",
				ArrivalAirport:   "JFK",
				DepartureTime:    "2026-06-22T11:30:00+02:00",
				ArrivalTime:      "2026-06-22T14:50:00-04:00",
				Cabin:            "Economy",
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal FlightDetails: %v", err)
	}

	var decoded FlightDetails
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal FlightDetails: %v", err)
	}

	// Verify top-level fields
	if decoded.Airline != original.Airline {
		t.Errorf("Airline: got %q, want %q", decoded.Airline, original.Airline)
	}
	if decoded.FlightNumber != original.FlightNumber {
		t.Errorf("FlightNumber: got %q, want %q", decoded.FlightNumber, original.FlightNumber)
	}
	if decoded.DepartureAirport != original.DepartureAirport {
		t.Errorf("DepartureAirport: got %q, want %q", decoded.DepartureAirport, original.DepartureAirport)
	}
	if decoded.ArrivalAirport != original.ArrivalAirport {
		t.Errorf("ArrivalAirport: got %q, want %q", decoded.ArrivalAirport, original.ArrivalAirport)
	}
	if decoded.Seat != original.Seat {
		t.Errorf("Seat: got %q, want %q", decoded.Seat, original.Seat)
	}

	// Verify legs
	if len(decoded.Legs) != 2 {
		t.Fatalf("Legs count: got %d, want 2", len(decoded.Legs))
	}
	if decoded.Legs[0].FlightNumber != "DL472" {
		t.Errorf("Leg[0].FlightNumber: got %q, want %q", decoded.Legs[0].FlightNumber, "DL472")
	}
	if decoded.Legs[0].DepartureAirport != "JFK" {
		t.Errorf("Leg[0].DepartureAirport: got %q, want %q", decoded.Legs[0].DepartureAirport, "JFK")
	}
	if decoded.Legs[0].ArrivalAirport != "BCN" {
		t.Errorf("Leg[0].ArrivalAirport: got %q, want %q", decoded.Legs[0].ArrivalAirport, "BCN")
	}
	if decoded.Legs[1].FlightNumber != "DL149" {
		t.Errorf("Leg[1].FlightNumber: got %q, want %q", decoded.Legs[1].FlightNumber, "DL149")
	}
	if decoded.Legs[1].DepartureAirport != "BCN" {
		t.Errorf("Leg[1].DepartureAirport: got %q, want %q", decoded.Legs[1].DepartureAirport, "BCN")
	}
	if decoded.Legs[1].ArrivalAirport != "JFK" {
		t.Errorf("Leg[1].ArrivalAirport: got %q, want %q", decoded.Legs[1].ArrivalAirport, "JFK")
	}
}

func TestFlightDetails_JSONRoundTrip_NoLegs(t *testing.T) {
	// Backward compatibility: FlightDetails without legs should still work.
	original := FlightDetails{
		Airline:          "United",
		FlightNumber:     "UA100",
		DepartureAirport: "SFO",
		ArrivalAirport:   "LHR",
		CabinClass:       "Business",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal FlightDetails: %v", err)
	}

	// Verify legs key is omitted (omitempty)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw map: %v", err)
	}
	if _, ok := raw["legs"]; ok {
		t.Error("expected 'legs' key to be omitted when empty, but it was present")
	}

	var decoded FlightDetails
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal FlightDetails: %v", err)
	}

	if decoded.Airline != "United" {
		t.Errorf("Airline: got %q, want %q", decoded.Airline, "United")
	}
	if decoded.FlightNumber != "UA100" {
		t.Errorf("FlightNumber: got %q, want %q", decoded.FlightNumber, "UA100")
	}
	if decoded.Legs != nil {
		t.Errorf("Legs: got %v, want nil", decoded.Legs)
	}
}

func TestFlightDetails_UnmarshalFromJSON_WithLegs(t *testing.T) {
	// Simulate AI-generated JSON with legs populated.
	input := `{
		"airline": "Delta",
		"flight_number": "DL472",
		"departure_airport": "JFK",
		"arrival_airport": "BCN",
		"seat": "27F",
		"cabin_class": "Economy",
		"passengers": ["MARTINEZ/ELENA"],
		"legs": [
			{
				"flight_number": "DL472",
				"airline": "Delta",
				"departure_airport": "JFK",
				"arrival_airport": "BCN",
				"departure_time": "2026-06-15T18:45:00",
				"arrival_time": "2026-06-16T08:55:00",
				"cabin": "Economy"
			},
			{
				"flight_number": "DL149",
				"airline": "Delta",
				"departure_airport": "BCN",
				"arrival_airport": "JFK",
				"departure_time": "2026-06-22T11:30:00",
				"arrival_time": "2026-06-22T14:50:00",
				"cabin": "Economy"
			}
		]
	}`

	var details FlightDetails
	if err := json.Unmarshal([]byte(input), &details); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(details.Legs) != 2 {
		t.Fatalf("Legs count: got %d, want 2", len(details.Legs))
	}
	if details.Legs[0].DepartureTime != "2026-06-15T18:45:00" {
		t.Errorf("Leg[0].DepartureTime: got %q", details.Legs[0].DepartureTime)
	}
	if details.Legs[1].ArrivalTime != "2026-06-22T14:50:00" {
		t.Errorf("Leg[1].ArrivalTime: got %q", details.Legs[1].ArrivalTime)
	}
}

func TestFlightDetails_UnmarshalFromJSON_NoLegs(t *testing.T) {
	// Simulate legacy AI output without legs field.
	input := `{
		"airline": "United",
		"flight_number": "UA100",
		"departure_airport": "SFO",
		"arrival_airport": "LHR"
	}`

	var details FlightDetails
	if err := json.Unmarshal([]byte(input), &details); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if details.Airline != "United" {
		t.Errorf("Airline: got %q, want %q", details.Airline, "United")
	}
	if details.Legs != nil {
		t.Errorf("Legs should be nil for legacy input, got %v", details.Legs)
	}
}

func TestUnmarshalDetails_Flight_WithLegs(t *testing.T) {
	input := json.RawMessage(`{
		"airline": "Delta",
		"flight_number": "DL472",
		"departure_airport": "JFK",
		"arrival_airport": "BCN",
		"legs": [
			{"flight_number": "DL472", "departure_airport": "JFK", "arrival_airport": "BCN"},
			{"flight_number": "DL149", "departure_airport": "BCN", "arrival_airport": "JFK"}
		]
	}`)

	result, err := UnmarshalDetails("flight", input)
	if err != nil {
		t.Fatalf("UnmarshalDetails: %v", err)
	}

	fd, ok := result.(*FlightDetails)
	if !ok {
		t.Fatalf("expected *FlightDetails, got %T", result)
	}

	if fd.Airline != "Delta" {
		t.Errorf("Airline: got %q, want %q", fd.Airline, "Delta")
	}
	if len(fd.Legs) != 2 {
		t.Fatalf("Legs count: got %d, want 2", len(fd.Legs))
	}
	if fd.Legs[0].FlightNumber != "DL472" {
		t.Errorf("Leg[0].FlightNumber: got %q, want %q", fd.Legs[0].FlightNumber, "DL472")
	}
	if fd.Legs[1].FlightNumber != "DL149" {
		t.Errorf("Leg[1].FlightNumber: got %q, want %q", fd.Legs[1].FlightNumber, "DL149")
	}
}

func TestUnmarshalDetails_Flight_NoLegs(t *testing.T) {
	input := json.RawMessage(`{
		"airline": "United",
		"flight_number": "UA100",
		"departure_airport": "SFO",
		"arrival_airport": "LHR"
	}`)

	result, err := UnmarshalDetails("flight", input)
	if err != nil {
		t.Fatalf("UnmarshalDetails: %v", err)
	}

	fd, ok := result.(*FlightDetails)
	if !ok {
		t.Fatalf("expected *FlightDetails, got %T", result)
	}

	if fd.Airline != "United" {
		t.Errorf("Airline: got %q, want %q", fd.Airline, "United")
	}
	if fd.Legs != nil {
		t.Errorf("Legs should be nil, got %v", fd.Legs)
	}
}

func TestUnmarshalDetails_EmptyAndNull(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
	}{
		{"empty", json.RawMessage(`{}`)},
		{"null", json.RawMessage(`null`)},
		{"empty bytes", json.RawMessage{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := UnmarshalDetails("flight", tt.raw)
			if err != nil {
				t.Fatalf("UnmarshalDetails(%q): %v", tt.name, err)
			}
			if result != nil {
				t.Errorf("expected nil for %q, got %v", tt.name, result)
			}
		})
	}
}

func TestFlightLeg_AllFields(t *testing.T) {
	leg := FlightLeg{
		FlightNumber:     "AA100",
		Airline:          "American Airlines",
		DepartureAirport: "DFW",
		ArrivalAirport:   "NRT",
		DepartureTime:    "2026-07-01T14:00:00",
		ArrivalTime:      "2026-07-02T17:30:00",
		Cabin:            "First",
	}

	data, err := json.Marshal(leg)
	if err != nil {
		t.Fatalf("marshal FlightLeg: %v", err)
	}

	var decoded FlightLeg
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal FlightLeg: %v", err)
	}

	if decoded != leg {
		t.Errorf("round-trip mismatch: got %+v, want %+v", decoded, leg)
	}
}

func TestFlightLeg_OmitEmpty(t *testing.T) {
	// FlightLeg with only some fields populated should omit empty fields.
	leg := FlightLeg{
		FlightNumber:     "UA200",
		DepartureAirport: "LAX",
		ArrivalAirport:   "SYD",
	}

	data, err := json.Marshal(leg)
	if err != nil {
		t.Fatalf("marshal FlightLeg: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	// Fields with omitempty should not be present when empty.
	for _, key := range []string{"airline", "departure_time", "arrival_time", "cabin"} {
		if _, ok := raw[key]; ok {
			t.Errorf("expected key %q to be omitted, but it was present", key)
		}
	}

	// Populated fields should be present.
	for _, key := range []string{"flight_number", "departure_airport", "arrival_airport"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected key %q to be present, but it was missing", key)
		}
	}
}

func TestFlightDetails_ConnectingFlight_ThreeLegs(t *testing.T) {
	// Test a multi-segment connecting flight (e.g., JFK -> FRA -> NRT).
	details := FlightDetails{
		Airline:          "Lufthansa",
		FlightNumber:     "LH401",
		DepartureAirport: "JFK",
		ArrivalAirport:   "NRT",
		CabinClass:       "Business",
		Legs: []FlightLeg{
			{
				FlightNumber:     "LH401",
				Airline:          "Lufthansa",
				DepartureAirport: "JFK",
				ArrivalAirport:   "FRA",
				DepartureTime:    "2026-08-01T19:00:00",
				ArrivalTime:      "2026-08-02T09:15:00",
				Cabin:            "Business",
			},
			{
				FlightNumber:     "LH710",
				Airline:          "Lufthansa",
				DepartureAirport: "FRA",
				ArrivalAirport:   "NRT",
				DepartureTime:    "2026-08-02T13:30:00",
				ArrivalTime:      "2026-08-03T08:45:00",
				Cabin:            "Business",
			},
			{
				FlightNumber:     "LH711",
				Airline:          "Lufthansa",
				DepartureAirport: "NRT",
				ArrivalAirport:   "FRA",
				DepartureTime:    "2026-08-15T10:00:00",
				ArrivalTime:      "2026-08-15T16:00:00",
				Cabin:            "Business",
			},
		},
	}

	data, err := json.Marshal(details)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded FlightDetails
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(decoded.Legs) != 3 {
		t.Fatalf("Legs count: got %d, want 3", len(decoded.Legs))
	}

	// Verify backward compat: top-level fields match first leg.
	if decoded.DepartureAirport != "JFK" {
		t.Errorf("top-level DepartureAirport: got %q, want %q", decoded.DepartureAirport, "JFK")
	}
	if decoded.ArrivalAirport != "NRT" {
		t.Errorf("top-level ArrivalAirport: got %q, want %q", decoded.ArrivalAirport, "NRT")
	}
	if decoded.FlightNumber != "LH401" {
		t.Errorf("top-level FlightNumber: got %q, want %q", decoded.FlightNumber, "LH401")
	}
}

func TestFerryDetails_JSONRoundTrip(t *testing.T) {
	original := FerryDetails{
		Operator:        "BC Ferries",
		VesselName:      "Spirit of Vancouver Island",
		DeparturePort:   "Tsawwassen",
		ArrivalPort:     "Swartz Bay",
		DepartureTime:   "2026-07-15T09:00:00",
		ArrivalTime:     "2026-07-15T10:35:00",
		CabinType:       "Passenger",
		Deck:            "5",
		NumPassengers:   2,
		VehicleIncluded: true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal FerryDetails: %v", err)
	}

	var decoded FerryDetails
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal FerryDetails: %v", err)
	}

	if decoded.Operator != "BC Ferries" {
		t.Errorf("Operator: got %q, want %q", decoded.Operator, "BC Ferries")
	}
	if decoded.DeparturePort != "Tsawwassen" {
		t.Errorf("DeparturePort: got %q, want %q", decoded.DeparturePort, "Tsawwassen")
	}
	if decoded.ArrivalPort != "Swartz Bay" {
		t.Errorf("ArrivalPort: got %q, want %q", decoded.ArrivalPort, "Swartz Bay")
	}
	if !decoded.VehicleIncluded {
		t.Error("VehicleIncluded: got false, want true")
	}
	if decoded.NumPassengers != 2 {
		t.Errorf("NumPassengers: got %d, want 2", decoded.NumPassengers)
	}
}

func TestBusDetails_JSONRoundTrip(t *testing.T) {
	original := BusDetails{
		Operator:         "FlixBus",
		RouteNumber:      "N740",
		DepartureStation: "Barcelona Nord",
		ArrivalStation:   "Madrid Estacion Sur",
		DepartureTime:    "2026-08-01T07:30:00",
		ArrivalTime:      "2026-08-01T15:00:00",
		Seat:             "12A",
		Class:            "Standard",
		Platform:         "3",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal BusDetails: %v", err)
	}

	var decoded BusDetails
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal BusDetails: %v", err)
	}

	if decoded.Operator != "FlixBus" {
		t.Errorf("Operator: got %q, want %q", decoded.Operator, "FlixBus")
	}
	if decoded.RouteNumber != "N740" {
		t.Errorf("RouteNumber: got %q, want %q", decoded.RouteNumber, "N740")
	}
	if decoded.DepartureStation != "Barcelona Nord" {
		t.Errorf("DepartureStation: got %q, want %q", decoded.DepartureStation, "Barcelona Nord")
	}
	if decoded.Seat != "12A" {
		t.Errorf("Seat: got %q, want %q", decoded.Seat, "12A")
	}
}

func TestCruiseDetails_JSONRoundTrip(t *testing.T) {
	original := CruiseDetails{
		CruiseLine:    "Royal Caribbean",
		ShipName:      "Wonder of the Seas",
		DeparturePort: "Fort Lauderdale",
		ArrivalPort:   "Fort Lauderdale",
		CabinNumber:   "8234",
		CabinType:     "Balcony",
		Deck:          "8",
		NumPassengers: 4,
		PortsOfCall:   []string{"Cozumel", "Roatan", "Costa Maya"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal CruiseDetails: %v", err)
	}

	var decoded CruiseDetails
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal CruiseDetails: %v", err)
	}

	if decoded.CruiseLine != "Royal Caribbean" {
		t.Errorf("CruiseLine: got %q, want %q", decoded.CruiseLine, "Royal Caribbean")
	}
	if decoded.ShipName != "Wonder of the Seas" {
		t.Errorf("ShipName: got %q, want %q", decoded.ShipName, "Wonder of the Seas")
	}
	if len(decoded.PortsOfCall) != 3 {
		t.Fatalf("PortsOfCall count: got %d, want 3", len(decoded.PortsOfCall))
	}
	if decoded.PortsOfCall[0] != "Cozumel" {
		t.Errorf("PortsOfCall[0]: got %q, want %q", decoded.PortsOfCall[0], "Cozumel")
	}
	if decoded.NumPassengers != 4 {
		t.Errorf("NumPassengers: got %d, want 4", decoded.NumPassengers)
	}
}

func TestTransferDetails_JSONRoundTrip(t *testing.T) {
	original := TransferDetails{
		Operator:        "Welcome Pickups",
		VehicleType:     "Sedan",
		PickupLocation:  "Barcelona Airport (BCN)",
		DropoffLocation: "Hotel Arts Barcelona",
		PickupTime:      "2026-06-15T14:30:00",
		NumPassengers:   2,
		DriverName:      "Carlos",
		FlightNumber:    "DL472",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal TransferDetails: %v", err)
	}

	var decoded TransferDetails
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal TransferDetails: %v", err)
	}

	if decoded.Operator != "Welcome Pickups" {
		t.Errorf("Operator: got %q, want %q", decoded.Operator, "Welcome Pickups")
	}
	if decoded.FlightNumber != "DL472" {
		t.Errorf("FlightNumber: got %q, want %q", decoded.FlightNumber, "DL472")
	}
	if decoded.PickupLocation != "Barcelona Airport (BCN)" {
		t.Errorf("PickupLocation: got %q, want %q", decoded.PickupLocation, "Barcelona Airport (BCN)")
	}
}

func TestUnmarshalDetails_Ferry(t *testing.T) {
	input := json.RawMessage(`{
		"operator": "BC Ferries",
		"vessel_name": "Spirit of Vancouver Island",
		"departure_port": "Tsawwassen",
		"arrival_port": "Swartz Bay",
		"num_passengers": 2,
		"vehicle_included": true
	}`)

	result, err := UnmarshalDetails("ferry", input)
	if err != nil {
		t.Fatalf("UnmarshalDetails: %v", err)
	}

	fd, ok := result.(*FerryDetails)
	if !ok {
		t.Fatalf("expected *FerryDetails, got %T", result)
	}
	if fd.Operator != "BC Ferries" {
		t.Errorf("Operator: got %q, want %q", fd.Operator, "BC Ferries")
	}
	if !fd.VehicleIncluded {
		t.Error("VehicleIncluded: got false, want true")
	}
}

func TestUnmarshalDetails_Bus(t *testing.T) {
	input := json.RawMessage(`{
		"operator": "FlixBus",
		"route_number": "N740",
		"departure_station": "Barcelona Nord",
		"arrival_station": "Madrid Estacion Sur",
		"seat": "12A"
	}`)

	result, err := UnmarshalDetails("bus", input)
	if err != nil {
		t.Fatalf("UnmarshalDetails: %v", err)
	}

	bd, ok := result.(*BusDetails)
	if !ok {
		t.Fatalf("expected *BusDetails, got %T", result)
	}
	if bd.Operator != "FlixBus" {
		t.Errorf("Operator: got %q, want %q", bd.Operator, "FlixBus")
	}
	if bd.Seat != "12A" {
		t.Errorf("Seat: got %q, want %q", bd.Seat, "12A")
	}
}

func TestUnmarshalDetails_Cruise(t *testing.T) {
	input := json.RawMessage(`{
		"cruise_line": "Royal Caribbean",
		"ship_name": "Wonder of the Seas",
		"departure_port": "Fort Lauderdale",
		"cabin_type": "Balcony",
		"ports_of_call": ["Cozumel", "Roatan"]
	}`)

	result, err := UnmarshalDetails("cruise", input)
	if err != nil {
		t.Fatalf("UnmarshalDetails: %v", err)
	}

	cd, ok := result.(*CruiseDetails)
	if !ok {
		t.Fatalf("expected *CruiseDetails, got %T", result)
	}
	if cd.CruiseLine != "Royal Caribbean" {
		t.Errorf("CruiseLine: got %q, want %q", cd.CruiseLine, "Royal Caribbean")
	}
	if len(cd.PortsOfCall) != 2 {
		t.Fatalf("PortsOfCall count: got %d, want 2", len(cd.PortsOfCall))
	}
}

func TestUnmarshalDetails_Transfer(t *testing.T) {
	input := json.RawMessage(`{
		"operator": "Welcome Pickups",
		"vehicle_type": "Sedan",
		"pickup_location": "BCN Airport",
		"dropoff_location": "Hotel Arts",
		"flight_number": "DL472"
	}`)

	result, err := UnmarshalDetails("transfer", input)
	if err != nil {
		t.Fatalf("UnmarshalDetails: %v", err)
	}

	td, ok := result.(*TransferDetails)
	if !ok {
		t.Fatalf("expected *TransferDetails, got %T", result)
	}
	if td.FlightNumber != "DL472" {
		t.Errorf("FlightNumber: got %q, want %q", td.FlightNumber, "DL472")
	}
	if td.VehicleType != "Sedan" {
		t.Errorf("VehicleType: got %q, want %q", td.VehicleType, "Sedan")
	}
}
