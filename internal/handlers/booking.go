package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
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
}

func NewBookingHandler(bookingSvc *booking.Service) *BookingHandler {
	return &BookingHandler{bookingSvc: bookingSvc}
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

	return connect.NewResponse(&toquiv1.ExtractBookingFieldResponse{
		Answer:          result.Answer,
		ExtractedFields: result.ExtractedFields,
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
