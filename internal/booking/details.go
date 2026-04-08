package booking

import (
	"encoding/json"
	"fmt"
)

type FlightDetails struct {
	Airline           string   `json:"airline,omitempty"`
	FlightNumber      string   `json:"flight_number,omitempty"`
	DepartureAirport  string   `json:"departure_airport,omitempty"`
	ArrivalAirport    string   `json:"arrival_airport,omitempty"`
	DepartureTerminal string   `json:"departure_terminal,omitempty"`
	ArrivalTerminal   string   `json:"arrival_terminal,omitempty"`
	Seat              string   `json:"seat,omitempty"`
	CabinClass        string   `json:"cabin_class,omitempty"`
	Passengers        []string `json:"passengers,omitempty"`
}

type HotelDetails struct {
	HotelName    string `json:"hotel_name,omitempty"`
	CheckInDate  string `json:"check_in_date,omitempty"`
	CheckOutDate string `json:"check_out_date,omitempty"`
	RoomType     string `json:"room_type,omitempty"`
	NumGuests    int    `json:"num_guests,omitempty"`
	Address      string `json:"address,omitempty"`
	Phone        string `json:"phone,omitempty"`
}

type CarRentalDetails struct {
	Company         string `json:"company,omitempty"`
	PickupLocation  string `json:"pickup_location,omitempty"`
	DropoffLocation string `json:"dropoff_location,omitempty"`
	PickupTime      string `json:"pickup_time,omitempty"`
	DropoffTime     string `json:"dropoff_time,omitempty"`
	CarType         string `json:"car_type,omitempty"`
	DriverName      string `json:"driver_name,omitempty"`
}

type TrainDetails struct {
	Operator         string `json:"operator,omitempty"`
	TrainNumber      string `json:"train_number,omitempty"`
	DepartureStation string `json:"departure_station,omitempty"`
	ArrivalStation   string `json:"arrival_station,omitempty"`
	Seat             string `json:"seat,omitempty"`
	CarNumber        string `json:"car_number,omitempty"`
	Class            string `json:"class,omitempty"`
}

type TourStop struct {
	Name     string `json:"name,omitempty"`
	Location string `json:"location,omitempty"`
	Duration string `json:"duration,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

type TourDetails struct {
	TourOperator    string `json:"tour_operator,omitempty"`
	TourName        string `json:"tour_name,omitempty"`
	NumParticipants int    `json:"num_participants,omitempty"`
	MeetingPoint    string `json:"meeting_point,omitempty"`
	// Run 4 R-06/R-11 found the tour parser dropped date, time, price and
	// guide. These fields cover the most common questions users ask about
	// booked tours ("what time?", "what's my guide called?", "how much did
	// I pay?").
	Date      string     `json:"date,omitempty"`       // ISO YYYY-MM-DD
	StartTime string     `json:"start_time,omitempty"` // local HH:MM or HH:MM-HH:MM window
	Duration  string     `json:"duration,omitempty"`   // e.g. "3h 30m"
	GuideName string     `json:"guide_name,omitempty"`
	Price     string     `json:"price,omitempty"` // currency-prefixed string, e.g. "USD 85.00"
	Stops     []TourStop `json:"stops,omitempty"`
}

type ActivityDetails struct {
	Operator     string `json:"operator,omitempty"`
	ActivityName string `json:"activity_name,omitempty"`
	Location     string `json:"location,omitempty"`
	NumGuests    int    `json:"num_guests,omitempty"`
	Date         string `json:"date,omitempty"`       // ISO YYYY-MM-DD
	StartTime    string `json:"start_time,omitempty"` // local HH:MM
	Duration     string `json:"duration,omitempty"`
	GuideName    string `json:"guide_name,omitempty"`
	Price        string `json:"price,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

type RestaurantDetails struct {
	RestaurantName string `json:"restaurant_name,omitempty"`
	Cuisine        string `json:"cuisine,omitempty"`
	PartySize      int    `json:"party_size,omitempty"`
	Notes          string `json:"notes,omitempty"`
}

func UnmarshalDetails(bookingType string, raw json.RawMessage) (any, error) {
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return nil, nil
	}

	var target any
	switch bookingType {
	case "flight":
		target = &FlightDetails{}
	case "hotel":
		target = &HotelDetails{}
	case "car_rental":
		target = &CarRentalDetails{}
	case "train":
		target = &TrainDetails{}
	case "tour":
		target = &TourDetails{}
	case "activity":
		target = &ActivityDetails{}
	case "restaurant":
		target = &RestaurantDetails{}
	default:
		var generic map[string]any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return nil, fmt.Errorf("unmarshal generic details: %w", err)
		}
		return generic, nil
	}

	if err := json.Unmarshal(raw, target); err != nil {
		return nil, fmt.Errorf("unmarshal %s details: %w", bookingType, err)
	}
	return target, nil
}
