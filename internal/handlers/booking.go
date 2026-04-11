package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/booking"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/requestid"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
)

type BookingHandler struct {
	bookingSvc *booking.Service
	queries    *dbgen.Queries
}

func NewBookingHandler(bookingSvc *booking.Service, queries *dbgen.Queries) *BookingHandler {
	return &BookingHandler{bookingSvc: bookingSvc, queries: queries}
}

func (h *BookingHandler) IngestBooking(ctx context.Context, req *connect.Request[toquiv1.IngestBookingRequest]) (*connect.Response[toquiv1.IngestBookingResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	typeHint := ""
	if req.Msg.Type != toquiv1.BookingType_BOOKING_TYPE_UNSPECIFIED {
		typeHint = req.Msg.Type.String()
	}
	b, err := h.bookingSvc.IngestText(ctx, userID, req.Msg.TripId, typeHint, req.Msg.RawText)
	if err != nil {
		return nil, aiAwareError(ctx, "booking ingest", err)
	}

	// Auto-link: create an itinerary item for the booking so it appears
	// in the day-by-day view. This is best-effort — failure does not
	// block the booking response.
	if h.queries != nil && b.TripID.Valid {
		h.autoLinkBookingToItinerary(ctx, uuid.UUID(b.TripID.Bytes), b)
	}

	return connect.NewResponse(&toquiv1.IngestBookingResponse{
		Booking: bookingToProto(b),
	}), nil
}

func (h *BookingHandler) IngestEmail(ctx context.Context, req *connect.Request[toquiv1.IngestEmailRequest]) (*connect.Response[toquiv1.IngestEmailResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("email ingestion not yet implemented"))
}

func (h *BookingHandler) ListBookings(ctx context.Context, req *connect.Request[toquiv1.ListBookingsRequest]) (*connect.Response[toquiv1.ListBookingsResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	tripID, err := uuid.Parse(req.Msg.TripId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	bookings, err := h.bookingSvc.ListByTrip(ctx, userID, tripID)
	if err != nil {
		return nil, internalError(ctx, "booking operation", err)
	}

	protoBookings := make([]*toquiv1.Booking, len(bookings))
	for i, b := range bookings {
		protoBookings[i] = bookingToProto(&b)
	}

	return connect.NewResponse(&toquiv1.ListBookingsResponse{
		Bookings: protoBookings,
	}), nil
}

func (h *BookingHandler) GetBooking(ctx context.Context, req *connect.Request[toquiv1.GetBookingRequest]) (*connect.Response[toquiv1.GetBookingResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	bookingID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	b, err := h.bookingSvc.GetByID(ctx, userID, bookingID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&toquiv1.GetBookingResponse{
		Booking: bookingToProto(b),
	}), nil
}

func (h *BookingHandler) LinkBookingToTrip(ctx context.Context, req *connect.Request[toquiv1.LinkBookingToTripRequest]) (*connect.Response[toquiv1.LinkBookingToTripResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	bookingID, err := uuid.Parse(req.Msg.BookingId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	tripID, err := uuid.Parse(req.Msg.TripId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	b, err := h.bookingSvc.LinkToTrip(ctx, userID, bookingID, tripID)
	if err != nil {
		return nil, internalError(ctx, "booking operation", err)
	}

	return connect.NewResponse(&toquiv1.LinkBookingToTripResponse{
		Booking: bookingToProto(b),
	}), nil
}

func (h *BookingHandler) DeleteBooking(ctx context.Context, req *connect.Request[toquiv1.DeleteBookingRequest]) (*connect.Response[toquiv1.DeleteBookingResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	bookingID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := h.bookingSvc.Delete(ctx, userID, bookingID); err != nil {
		return nil, internalError(ctx, "booking operation", err)
	}

	return connect.NewResponse(&toquiv1.DeleteBookingResponse{}), nil
}

func (h *BookingHandler) ExtractBookingField(ctx context.Context, req *connect.Request[toquiv1.ExtractBookingFieldRequest]) (*connect.Response[toquiv1.ExtractBookingFieldResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	bookingID, err := uuid.Parse(req.Msg.BookingId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	result, err := h.bookingSvc.ExtractField(ctx, userID, bookingID, req.Msg.Question)
	if err != nil {
		return nil, aiAwareError(ctx, "booking extract field", err)
	}

	// Convert map[string]any to map[string]string for proto compatibility
	fields := make(map[string]string, len(result.ExtractedFields))
	for k, v := range result.ExtractedFields {
		fields[k] = fmt.Sprintf("%v", v)
	}

	return connect.NewResponse(&toquiv1.ExtractBookingFieldResponse{
		Answer:          result.Answer,
		ExtractedFields: fields,
	}), nil
}

// aiAwareError checks if an error originated from an AI provider (e.g., rate
// limit) and returns an appropriate connect error code. Rate limit and provider
// errors use CodeUnavailable; other errors use CodeInternal. The original error
// is logged server-side; only a sanitized message reaches the client.
func aiAwareError(ctx context.Context, operation string, err error) *connect.Error {
	sanitized := ai.SanitizeProviderError(err)
	reqID := requestid.FromContext(ctx)

	// Log the original unsanitized error for debugging.
	slog.Error(operation, "error", ai.OriginalError(err), "request_id", reqID)

	if ai.IsProviderRateLimit(sanitized) {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("%s", sanitized.Error()))
	}

	// Check if it was any provider error (SanitizedError) vs a non-AI error.
	var se *ai.SanitizedError
	if errors.As(sanitized, &se) {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("%s", se.Error()))
	}

	// Non-AI error — use the standard internal error path.
	return connect.NewError(connect.CodeInternal, fmt.Errorf("an internal error occurred"))
}

var bookingTypeMap = map[string]toquiv1.BookingType{
	"flight":     toquiv1.BookingType_BOOKING_TYPE_FLIGHT,
	"hotel":      toquiv1.BookingType_BOOKING_TYPE_HOTEL,
	"car_rental": toquiv1.BookingType_BOOKING_TYPE_CAR_RENTAL,
	"train":      toquiv1.BookingType_BOOKING_TYPE_TRAIN,
	"activity":   toquiv1.BookingType_BOOKING_TYPE_ACTIVITY,
	"restaurant": toquiv1.BookingType_BOOKING_TYPE_RESTAURANT,
	"other":      toquiv1.BookingType_BOOKING_TYPE_OTHER,
	"tour":       toquiv1.BookingType_BOOKING_TYPE_TOUR,
}

var bookingSourceMap = map[string]toquiv1.BookingSource{
	"email":  toquiv1.BookingSource_BOOKING_SOURCE_EMAIL,
	"manual": toquiv1.BookingSource_BOOKING_SOURCE_MANUAL,
	"paste":  toquiv1.BookingSource_BOOKING_SOURCE_PASTE,
}

func bookingToProto(b *dbgen.Booking) *toquiv1.Booking {
	proto := &toquiv1.Booking{
		Id:        b.ID.String(),
		Title:     b.Title,
		Type:      bookingTypeMap[b.Type],
		Source:    bookingSourceMap[b.Source],
		CreatedAt: timestamppb.New(b.CreatedAt),
	}

	if b.TripID.Valid {
		tripUUID := uuid.UUID(b.TripID.Bytes)
		proto.TripId = tripUUID.String()
	}
	if b.ConfirmationCode.Valid {
		proto.ConfirmationCode = b.ConfirmationCode.String
	}
	if b.Provider.Valid {
		proto.Provider = b.Provider.String
	}
	if b.StartTime.Valid {
		proto.StartTime = timestamppb.New(b.StartTime.Time)
	}
	if b.EndTime.Valid {
		proto.EndTime = timestamppb.New(b.EndTime.Time)
	}
	if b.Address.Valid {
		proto.Address = b.Address.String
	}
	if b.DepartureLocation.Valid {
		proto.DepartureLocation = b.DepartureLocation.String
	}
	if b.ArrivalLocation.Valid {
		proto.ArrivalLocation = b.ArrivalLocation.String
	}
	if b.NumGuests.Valid {
		proto.NumGuests = b.NumGuests.Int32
	}
	if len(b.DetailsJson) > 0 {
		proto.DetailsJson = string(b.DetailsJson)
		setBookingDetailsOneof(proto, b.Type, b.DetailsJson)
	}
	if b.RawSource.Valid {
		proto.RawSource = b.RawSource.String
	}

	return proto
}

func setBookingDetailsOneof(proto *toquiv1.Booking, bookingType string, raw json.RawMessage) {
	switch bookingType {
	case "flight":
		var d booking.FlightDetails
		if json.Unmarshal(raw, &d) == nil {
			var protoLegs []*toquiv1.FlightLeg
			for _, leg := range d.Legs {
				protoLegs = append(protoLegs, &toquiv1.FlightLeg{
					FlightNumber:     leg.FlightNumber,
					Airline:          leg.Airline,
					DepartureAirport: leg.DepartureAirport,
					ArrivalAirport:   leg.ArrivalAirport,
					DepartureTime:    leg.DepartureTime,
					ArrivalTime:      leg.ArrivalTime,
					Cabin:            leg.Cabin,
				})
			}
			proto.BookingDetails = &toquiv1.Booking_FlightDetails{
				FlightDetails: &toquiv1.FlightDetails{
					Airline:           d.Airline,
					FlightNumber:      d.FlightNumber,
					DepartureAirport:  d.DepartureAirport,
					ArrivalAirport:    d.ArrivalAirport,
					DepartureTerminal: d.DepartureTerminal,
					ArrivalTerminal:   d.ArrivalTerminal,
					Seat:              d.Seat,
					CabinClass:        d.CabinClass,
					Passengers:        d.Passengers,
					Legs:              protoLegs,
				},
			}
		}
	case "hotel":
		var d booking.HotelDetails
		if json.Unmarshal(raw, &d) == nil {
			proto.BookingDetails = &toquiv1.Booking_HotelDetails{
				HotelDetails: &toquiv1.HotelDetails{
					HotelName:    d.HotelName,
					CheckInDate:  d.CheckInDate,
					CheckOutDate: d.CheckOutDate,
					RoomType:     d.RoomType,
					NumGuests:    int32(d.NumGuests),
					Address:      d.Address,
					Phone:        d.Phone,
				},
			}
		}
	case "car_rental":
		var d booking.CarRentalDetails
		if json.Unmarshal(raw, &d) == nil {
			proto.BookingDetails = &toquiv1.Booking_CarRentalDetails{
				CarRentalDetails: &toquiv1.CarRentalDetails{
					Company:         d.Company,
					PickupLocation:  d.PickupLocation,
					DropoffLocation: d.DropoffLocation,
					PickupTime:      d.PickupTime,
					DropoffTime:     d.DropoffTime,
					CarType:         d.CarType,
					DriverName:      d.DriverName,
				},
			}
		}
	case "train":
		var d booking.TrainDetails
		if json.Unmarshal(raw, &d) == nil {
			proto.BookingDetails = &toquiv1.Booking_TrainDetails{
				TrainDetails: &toquiv1.TrainDetails{
					Operator:         d.Operator,
					TrainNumber:      d.TrainNumber,
					DepartureStation: d.DepartureStation,
					ArrivalStation:   d.ArrivalStation,
					Seat:             d.Seat,
					CarNumber:        d.CarNumber,
					Class:            d.Class,
				},
			}
		}
	case "tour":
		var d booking.TourDetails
		if json.Unmarshal(raw, &d) == nil {
			stops := make([]*toquiv1.TourStop, len(d.Stops))
			for i, s := range d.Stops {
				stops[i] = &toquiv1.TourStop{
					Name:     s.Name,
					Location: s.Location,
					Duration: s.Duration,
					Notes:    s.Notes,
				}
			}
			proto.BookingDetails = &toquiv1.Booking_TourDetails{
				TourDetails: &toquiv1.TourDetails{
					TourOperator:    d.TourOperator,
					TourName:        d.TourName,
					NumParticipants: int32(d.NumParticipants),
					MeetingPoint:    d.MeetingPoint,
					Stops:           stops,
					Date:            d.Date,
					StartTime:       d.StartTime,
					Duration:        d.Duration,
					GuideName:       d.GuideName,
					Price:           d.Price,
				},
			}
		}
	case "activity":
		var d booking.ActivityDetails
		if json.Unmarshal(raw, &d) == nil {
			proto.BookingDetails = &toquiv1.Booking_ActivityDetails{
				ActivityDetails: &toquiv1.ActivityDetails{
					Operator:     d.Operator,
					ActivityName: d.ActivityName,
					Location:     d.Location,
					NumGuests:    int32(d.NumGuests),
					Notes:        d.Notes,
					Date:         d.Date,
					StartTime:    d.StartTime,
					Duration:     d.Duration,
					GuideName:    d.GuideName,
					Price:        d.Price,
				},
			}
		}
	case "restaurant":
		var d booking.RestaurantDetails
		if json.Unmarshal(raw, &d) == nil {
			proto.BookingDetails = &toquiv1.Booking_RestaurantDetails{
				RestaurantDetails: &toquiv1.RestaurantDetails{
					RestaurantName: d.RestaurantName,
					Cuisine:        d.Cuisine,
					PartySize:      int32(d.PartySize),
					Notes:          d.Notes,
				},
			}
		}
	}
}

// autoLinkBookingToItinerary creates an itinerary item linked to a booking
// so it appears in the day-by-day view. Skips if a linked item already exists.
func (h *BookingHandler) autoLinkBookingToItinerary(ctx context.Context, tripID uuid.UUID, b *dbgen.Booking) {
	// Check if an itinerary item is already linked to this booking.
	_, err := h.queries.GetItineraryItemByBooking(ctx, dbgen.GetItineraryItemByBookingParams{
		BookingID: pgtype.UUID{Bytes: b.ID, Valid: true},
		TripID:    tripID,
	})
	if err == nil {
		// Already linked, nothing to do.
		return
	}

	// Map booking type to itinerary item type.
	itemType := "booking"
	switch b.Type {
	case "BOOKING_TYPE_FLIGHT", "flight":
		itemType = "flight"
	case "BOOKING_TYPE_HOTEL", "hotel":
		itemType = "hotel"
	case "BOOKING_TYPE_CAR_RENTAL", "car_rental":
		itemType = "car_rental"
	case "BOOKING_TYPE_TOUR", "BOOKING_TYPE_ACTIVITY", "tour", "activity":
		itemType = "activity"
	case "BOOKING_TYPE_RESTAURANT", "restaurant":
		itemType = "restaurant"
	case "BOOKING_TYPE_TRAIN", "train":
		itemType = "train"
	}

	// Determine day number from start_time if available.
	var dayNumber int32
	// Default to day 1 if no start_time — the user can reorder later.
	if b.StartTime.Valid {
		dayNumber = 1 // Will be placed on day 1; proper date→day mapping requires trip start_date
	}

	_, createErr := h.queries.CreateItineraryItemFromBooking(ctx, dbgen.CreateItineraryItemFromBookingParams{
		TripID:     tripID,
		DayNumber:  pgtype.Int4{Int32: dayNumber, Valid: true},
		OrderInDay: pgtype.Int4{Int32: 0, Valid: true}, // top of the day
		Type:       pgtype.Text{String: itemType, Valid: true},
		Title:      pgtype.Text{String: b.Title, Valid: true},
		Description: pgtype.Text{
			String: fmt.Sprintf("Booking: %s (confirmation: %s)",
				b.Provider.String, b.ConfirmationCode.String),
			Valid: b.Provider.Valid || b.ConfirmationCode.Valid,
		},
		StartTime: b.StartTime,
		EndTime:   b.EndTime,
		BookingID: pgtype.UUID{Bytes: b.ID, Valid: true},
	})
	if createErr != nil {
		slog.Warn("auto-link booking to itinerary failed", "error", createErr, "booking_id", b.ID, "trip_id", tripID)
		return
	}
	slog.Info("auto-linked booking to itinerary", "booking_id", b.ID, "trip_id", tripID, "type", itemType)
}
