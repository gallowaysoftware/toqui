package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
)

func TestBookingTypeMap_AllTypes(t *testing.T) {
	expectedTypes := []string{
		"flight", "hotel", "car_rental", "train", "activity",
		"restaurant", "other", "tour", "ferry", "bus", "cruise", "transfer",
	}

	for _, bt := range expectedTypes {
		if _, ok := bookingTypeMap[bt]; !ok {
			t.Errorf("bookingTypeMap missing type %q", bt)
		}
	}
}

func TestBookingTypeToString(t *testing.T) {
	tests := []struct {
		input toquiv1.BookingType
		want  string
	}{
		{toquiv1.BookingType_BOOKING_TYPE_FLIGHT, "flight"},
		{toquiv1.BookingType_BOOKING_TYPE_HOTEL, "hotel"},
		{toquiv1.BookingType_BOOKING_TYPE_CAR_RENTAL, "car_rental"},
		{toquiv1.BookingType_BOOKING_TYPE_TRAIN, "train"},
		{toquiv1.BookingType_BOOKING_TYPE_ACTIVITY, "activity"},
		{toquiv1.BookingType_BOOKING_TYPE_RESTAURANT, "restaurant"},
		{toquiv1.BookingType_BOOKING_TYPE_OTHER, "other"},
		{toquiv1.BookingType_BOOKING_TYPE_TOUR, "tour"},
		{toquiv1.BookingType_BOOKING_TYPE_FERRY, "ferry"},
		{toquiv1.BookingType_BOOKING_TYPE_BUS, "bus"},
		{toquiv1.BookingType_BOOKING_TYPE_CRUISE, "cruise"},
		{toquiv1.BookingType_BOOKING_TYPE_TRANSFER, "transfer"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := bookingTypeToString(tt.input)
			if got != tt.want {
				t.Errorf("bookingTypeToString(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBookingTypeToString_Unspecified(t *testing.T) {
	got := bookingTypeToString(toquiv1.BookingType_BOOKING_TYPE_UNSPECIFIED)
	if got != "" {
		t.Errorf("bookingTypeToString(UNSPECIFIED) = %q, want empty string", got)
	}
}

func TestBookingToProto_PriceAndTimezone(t *testing.T) {
	b := &dbgen.Booking{
		Title:      "Ferry to Swartz Bay",
		Type:       "ferry",
		Source:     "paste",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		PriceCents: pgtype.Int8{Int64: 11750, Valid: true},
		Currency:   pgtype.Text{String: "CAD", Valid: true},
		Timezone:   pgtype.Text{String: "America/Vancouver", Valid: true},
	}

	proto := bookingToProto(b)

	if proto.PriceCents != 11750 {
		t.Errorf("PriceCents: got %d, want 11750", proto.PriceCents)
	}
	if proto.Currency != "CAD" {
		t.Errorf("Currency: got %q, want %q", proto.Currency, "CAD")
	}
	if proto.Timezone != "America/Vancouver" {
		t.Errorf("Timezone: got %q, want %q", proto.Timezone, "America/Vancouver")
	}
	if proto.Type != toquiv1.BookingType_BOOKING_TYPE_FERRY {
		t.Errorf("Type: got %v, want BOOKING_TYPE_FERRY", proto.Type)
	}
}

func TestBookingToProto_NoPriceOrTimezone(t *testing.T) {
	b := &dbgen.Booking{
		Title:     "Flight to Barcelona",
		Type:      "flight",
		Source:    "paste",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	proto := bookingToProto(b)

	if proto.PriceCents != 0 {
		t.Errorf("PriceCents: got %d, want 0", proto.PriceCents)
	}
	if proto.Currency != "" {
		t.Errorf("Currency: got %q, want empty", proto.Currency)
	}
	if proto.Timezone != "" {
		t.Errorf("Timezone: got %q, want empty", proto.Timezone)
	}
}

func TestSetBookingDetailsOneof_Ferry(t *testing.T) {
	raw := json.RawMessage(`{
		"operator": "BC Ferries",
		"vessel_name": "Spirit of Vancouver Island",
		"departure_port": "Tsawwassen",
		"arrival_port": "Swartz Bay",
		"num_passengers": 2,
		"vehicle_included": true
	}`)

	proto := &toquiv1.Booking{}
	setBookingDetailsOneof(proto, "ferry", raw)

	fd := proto.GetFerryDetails()
	if fd == nil {
		t.Fatal("expected FerryDetails oneof, got nil")
	}
	if fd.Operator != "BC Ferries" {
		t.Errorf("Operator: got %q, want %q", fd.Operator, "BC Ferries")
	}
	if fd.VesselName != "Spirit of Vancouver Island" {
		t.Errorf("VesselName: got %q, want %q", fd.VesselName, "Spirit of Vancouver Island")
	}
	if !fd.VehicleIncluded {
		t.Error("VehicleIncluded: got false, want true")
	}
}

func TestSetBookingDetailsOneof_Bus(t *testing.T) {
	raw := json.RawMessage(`{
		"operator": "FlixBus",
		"route_number": "N740",
		"departure_station": "Barcelona Nord",
		"arrival_station": "Madrid Estacion Sur",
		"seat": "12A",
		"class": "Standard",
		"platform": "3"
	}`)

	proto := &toquiv1.Booking{}
	setBookingDetailsOneof(proto, "bus", raw)

	bd := proto.GetBusDetails()
	if bd == nil {
		t.Fatal("expected BusDetails oneof, got nil")
	}
	if bd.Operator != "FlixBus" {
		t.Errorf("Operator: got %q, want %q", bd.Operator, "FlixBus")
	}
	if bd.RouteNumber != "N740" {
		t.Errorf("RouteNumber: got %q, want %q", bd.RouteNumber, "N740")
	}
	if bd.Seat != "12A" {
		t.Errorf("Seat: got %q, want %q", bd.Seat, "12A")
	}
	if bd.Platform != "3" {
		t.Errorf("Platform: got %q, want %q", bd.Platform, "3")
	}
}

func TestSetBookingDetailsOneof_Cruise(t *testing.T) {
	raw := json.RawMessage(`{
		"cruise_line": "Royal Caribbean",
		"ship_name": "Wonder of the Seas",
		"departure_port": "Fort Lauderdale",
		"cabin_type": "Balcony",
		"deck": "8",
		"num_passengers": 4,
		"ports_of_call": ["Cozumel", "Roatan", "Costa Maya"]
	}`)

	proto := &toquiv1.Booking{}
	setBookingDetailsOneof(proto, "cruise", raw)

	cd := proto.GetCruiseDetails()
	if cd == nil {
		t.Fatal("expected CruiseDetails oneof, got nil")
	}
	if cd.CruiseLine != "Royal Caribbean" {
		t.Errorf("CruiseLine: got %q, want %q", cd.CruiseLine, "Royal Caribbean")
	}
	if len(cd.PortsOfCall) != 3 {
		t.Fatalf("PortsOfCall count: got %d, want 3", len(cd.PortsOfCall))
	}
	if cd.NumPassengers != 4 {
		t.Errorf("NumPassengers: got %d, want 4", cd.NumPassengers)
	}
}

func TestSetBookingDetailsOneof_Transfer(t *testing.T) {
	raw := json.RawMessage(`{
		"operator": "Welcome Pickups",
		"vehicle_type": "Sedan",
		"pickup_location": "BCN Airport",
		"dropoff_location": "Hotel Arts",
		"pickup_time": "14:30",
		"num_passengers": 2,
		"driver_name": "Carlos",
		"flight_number": "DL472"
	}`)

	proto := &toquiv1.Booking{}
	setBookingDetailsOneof(proto, "transfer", raw)

	td := proto.GetTransferDetails()
	if td == nil {
		t.Fatal("expected TransferDetails oneof, got nil")
	}
	if td.Operator != "Welcome Pickups" {
		t.Errorf("Operator: got %q, want %q", td.Operator, "Welcome Pickups")
	}
	if td.FlightNumber != "DL472" {
		t.Errorf("FlightNumber: got %q, want %q", td.FlightNumber, "DL472")
	}
	if td.VehicleType != "Sedan" {
		t.Errorf("VehicleType: got %q, want %q", td.VehicleType, "Sedan")
	}
}
