package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/booking"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/requestid"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"

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
	result, err := h.bookingSvc.IngestText(ctx, userID, req.Msg.TripId, typeHint, req.Msg.RawText)
	if err != nil {
		// Authz failure on the trip_id → 403 (#361). Anything else
		// runs through the existing aiAwareError helper (Anthropic
		// 529 handling etc.).
		if errors.Is(err, trip.ErrNotOwnerOrEditor) {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("editor access required to add bookings to this trip"))
		}
		return nil, aiAwareError(ctx, "booking ingest", err)
	}

	b := result.Booking

	// Auto-link: create an itinerary item for the booking so it appears
	// in the day-by-day view. This is best-effort — failure does not
	// block the booking response. Skip on merges: the itinerary item
	// is already linked to the existing booking.
	// Pass the caller userID so the SQL-gated insert re-checks authz
	// (defence-in-depth; the service already verified it during
	// CreateBookingForOwnerOrEditor, but any future code path that reaches
	// autoLinkBookingToItinerary with a mismatched pair should still be safe).
	if h.queries != nil && b.TripID.Valid && !result.WasUpdated {
		h.autoLinkBookingToItinerary(ctx, userID, uuid.UUID(b.TripID.Bytes), b)
	}

	return connect.NewResponse(&toquiv1.IngestBookingResponse{
		Booking:           bookingToProto(b),
		WasUpdated:        result.WasUpdated,
		PreviousBookingId: result.PreviousID,
	}), nil
}

func (h *BookingHandler) IngestEmail(ctx context.Context, req *connect.Request[toquiv1.IngestEmailRequest]) (*connect.Response[toquiv1.IngestEmailResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("email ingestion not yet implemented"))
}

func (h *BookingHandler) UpdateBooking(ctx context.Context, req *connect.Request[toquiv1.UpdateBookingRequest]) (*connect.Response[toquiv1.UpdateBookingResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	bookingID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Build update params. COALESCE in the SQL handles partial updates:
	// empty strings and zero values are treated as "no change".
	bookingType := ""
	if req.Msg.Type != toquiv1.BookingType_BOOKING_TYPE_UNSPECIFIED {
		bookingType = bookingTypeToString(req.Msg.Type)
	}

	params := dbgen.UpdateBookingParams{
		Title:             req.Msg.Title,
		Type:              bookingType,
		ConfirmationCode:  req.Msg.ConfirmationCode,
		Provider:          req.Msg.Provider,
		Address:           req.Msg.Address,
		DepartureLocation: req.Msg.DepartureLocation,
		ArrivalLocation:   req.Msg.ArrivalLocation,
		Currency:          req.Msg.Currency,
		Timezone:          req.Msg.Timezone,
	}

	if req.Msg.StartTime != nil {
		params.StartTime = pgtype.Timestamptz{Time: req.Msg.StartTime.AsTime(), Valid: true}
	}
	if req.Msg.EndTime != nil {
		params.EndTime = pgtype.Timestamptz{Time: req.Msg.EndTime.AsTime(), Valid: true}
	}
	if req.Msg.NumGuests > 0 {
		params.NumGuests = pgtype.Int4{Int32: req.Msg.NumGuests, Valid: true}
	}
	if req.Msg.PriceCents > 0 {
		params.PriceCents = pgtype.Int8{Int64: req.Msg.PriceCents, Valid: true}
	}

	b, err := h.bookingSvc.Update(ctx, userID, bookingID, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("booking not found"))
		}
		return nil, internalError(ctx, "booking update", err)
	}

	return connect.NewResponse(&toquiv1.UpdateBookingResponse{
		Booking: bookingToProto(b),
	}), nil
}

func (h *BookingHandler) GetTripCostSummary(ctx context.Context, req *connect.Request[toquiv1.GetTripCostSummaryRequest]) (*connect.Response[toquiv1.GetTripCostSummaryResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	tripID, err := uuid.Parse(req.Msg.TripId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	summaries, err := h.bookingSvc.GetTripCostSummary(ctx, userID, tripID)
	if err != nil {
		return nil, internalError(ctx, "trip cost summary", err)
	}

	var totalBookings int32
	totals := make([]*toquiv1.CurrencyTotal, len(summaries))
	for i, s := range summaries {
		totals[i] = &toquiv1.CurrencyTotal{
			Currency:     s.Currency,
			TotalCents:   s.TotalCents,
			BookingCount: int32(s.BookingCount),
		}
		totalBookings += int32(s.BookingCount)
	}

	return connect.NewResponse(&toquiv1.GetTripCostSummaryResponse{
		Totals:       totals,
		BookingCount: totalBookings,
	}), nil
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
		return nil, mapTripErr(ctx, "booking link", err)
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

	deleted, err := h.bookingSvc.Delete(ctx, userID, bookingID)
	if err != nil {
		return nil, internalError(ctx, "booking operation", err)
	}
	if !deleted {
		// Idempotent DELETE — the booking either did not exist or was owned by
		// another user. Log at debug for audit but return success to the client.
		slog.DebugContext(ctx, "DeleteBooking no-op",
			"user_id", userID,
			"booking_id", bookingID,
		)
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
	"flight":          toquiv1.BookingType_BOOKING_TYPE_FLIGHT,
	"hotel":           toquiv1.BookingType_BOOKING_TYPE_HOTEL,
	"car_rental":      toquiv1.BookingType_BOOKING_TYPE_CAR_RENTAL,
	"train":           toquiv1.BookingType_BOOKING_TYPE_TRAIN,
	"activity":        toquiv1.BookingType_BOOKING_TYPE_ACTIVITY,
	"restaurant":      toquiv1.BookingType_BOOKING_TYPE_RESTAURANT,
	"other":           toquiv1.BookingType_BOOKING_TYPE_OTHER,
	"tour":            toquiv1.BookingType_BOOKING_TYPE_TOUR,
	"ferry":           toquiv1.BookingType_BOOKING_TYPE_FERRY,
	"bus":             toquiv1.BookingType_BOOKING_TYPE_BUS,
	"cruise":          toquiv1.BookingType_BOOKING_TYPE_CRUISE,
	"transfer":        toquiv1.BookingType_BOOKING_TYPE_TRANSFER,
	"vacation_rental": toquiv1.BookingType_BOOKING_TYPE_VACATION_RENTAL,
}

// bookingTypeToString maps proto BookingType enum to the DB string representation.
func bookingTypeToString(bt toquiv1.BookingType) string {
	for k, v := range bookingTypeMap {
		if v == bt {
			return k
		}
	}
	return ""
}

// itineraryItemTypeForBooking maps a booking's stored type string (DB form or
// proto enum name) to the itinerary item type used when auto-linking a booking
// into a trip's day-by-day plan. Unknown types fall back to "booking".
//
// The double-form switch (DB string + enum name) is defence-in-depth: the DB
// stores the short form ("hotel") but tests and older paths occasionally pass
// the enum name ("BOOKING_TYPE_HOTEL").
func itineraryItemTypeForBooking(bookingType string) string {
	switch bookingType {
	case "BOOKING_TYPE_FLIGHT", "flight":
		return "flight"
	case "BOOKING_TYPE_HOTEL", "hotel":
		return "hotel"
	case "BOOKING_TYPE_VACATION_RENTAL", "vacation_rental":
		return "vacation_rental"
	case "BOOKING_TYPE_CAR_RENTAL", "car_rental":
		return "car_rental"
	case "BOOKING_TYPE_TOUR", "BOOKING_TYPE_ACTIVITY", "tour", "activity":
		return "activity"
	case "BOOKING_TYPE_RESTAURANT", "restaurant":
		return "restaurant"
	case "BOOKING_TYPE_TRAIN", "train":
		return "train"
	case "BOOKING_TYPE_FERRY", "ferry":
		return "ferry"
	case "BOOKING_TYPE_BUS", "bus":
		return "bus"
	case "BOOKING_TYPE_CRUISE", "cruise":
		return "cruise"
	case "BOOKING_TYPE_TRANSFER", "transfer":
		return "transfer"
	}
	return "booking"
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
	if b.PriceCents.Valid {
		proto.PriceCents = b.PriceCents.Int64
	}
	if b.Currency.Valid {
		proto.Currency = b.Currency.String
	}
	if b.Timezone.Valid {
		proto.Timezone = b.Timezone.String
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
	case "hotel", "vacation_rental":
		// Vacation rentals reuse the HotelDetails oneof — the schema maps
		// cleanly (hotel_name → listing title, room_type → unit type,
		// check_in/check_out dates, address, phone). A dedicated
		// VacationRentalDetails message can be added later if needed; for now
		// this avoids a breaking split while distinguishing the two via the
		// top-level BookingType enum.
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
	case "ferry":
		var d booking.FerryDetails
		if json.Unmarshal(raw, &d) == nil {
			proto.BookingDetails = &toquiv1.Booking_FerryDetails{
				FerryDetails: &toquiv1.FerryDetails{
					Operator:        d.Operator,
					VesselName:      d.VesselName,
					DeparturePort:   d.DeparturePort,
					ArrivalPort:     d.ArrivalPort,
					DepartureTime:   d.DepartureTime,
					ArrivalTime:     d.ArrivalTime,
					CabinType:       d.CabinType,
					Deck:            d.Deck,
					NumPassengers:   int32(d.NumPassengers),
					VehicleIncluded: d.VehicleIncluded,
				},
			}
		}
	case "bus":
		var d booking.BusDetails
		if json.Unmarshal(raw, &d) == nil {
			proto.BookingDetails = &toquiv1.Booking_BusDetails{
				BusDetails: &toquiv1.BusDetails{
					Operator:         d.Operator,
					RouteNumber:      d.RouteNumber,
					DepartureStation: d.DepartureStation,
					ArrivalStation:   d.ArrivalStation,
					DepartureTime:    d.DepartureTime,
					ArrivalTime:      d.ArrivalTime,
					Seat:             d.Seat,
					Class:            d.Class,
					Platform:         d.Platform,
				},
			}
		}
	case "cruise":
		var d booking.CruiseDetails
		if json.Unmarshal(raw, &d) == nil {
			proto.BookingDetails = &toquiv1.Booking_CruiseDetails{
				CruiseDetails: &toquiv1.CruiseDetails{
					CruiseLine:    d.CruiseLine,
					ShipName:      d.ShipName,
					DeparturePort: d.DeparturePort,
					ArrivalPort:   d.ArrivalPort,
					CabinNumber:   d.CabinNumber,
					CabinType:     d.CabinType,
					Deck:          d.Deck,
					NumPassengers: int32(d.NumPassengers),
					PortsOfCall:   d.PortsOfCall,
				},
			}
		}
	case "transfer":
		var d booking.TransferDetails
		if json.Unmarshal(raw, &d) == nil {
			proto.BookingDetails = &toquiv1.Booking_TransferDetails{
				TransferDetails: &toquiv1.TransferDetails{
					Operator:        d.Operator,
					VehicleType:     d.VehicleType,
					PickupLocation:  d.PickupLocation,
					DropoffLocation: d.DropoffLocation,
					PickupTime:      d.PickupTime,
					NumPassengers:   int32(d.NumPassengers),
					DriverName:      d.DriverName,
					FlightNumber:    d.FlightNumber,
				},
			}
		}
	}
}

// autoLinkBookingToItinerary creates an itinerary item linked to a booking
// so it appears in the day-by-day view. Skips if a linked item already
// exists. Uses the SQL-gated CreateItineraryItemFromBookingForOwnerOrEditor
// query so a mismatched (callerID, tripID) pair can't plant items into a
// foreign trip (#361 defence-in-depth — the service-layer
// CreateBookingForOwnerOrEditor already verifies this, but we want the
// insert to be safe regardless of how it's reached).
func (h *BookingHandler) autoLinkBookingToItinerary(ctx context.Context, callerID, tripID uuid.UUID, b *dbgen.Booking) {
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
	itemType := itineraryItemTypeForBooking(b.Type)

	// Determine day number from start_time if available.
	var dayNumber int32
	// Default to day 1 if no start_time — the user can reorder later.
	if b.StartTime.Valid {
		dayNumber = 1 // Will be placed on day 1; proper date→day mapping requires trip start_date
	}

	_, createErr := h.queries.CreateItineraryItemFromBookingForOwnerOrEditor(ctx, dbgen.CreateItineraryItemFromBookingForOwnerOrEditorParams{
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
		UserID:    callerID,
	})
	if createErr != nil {
		slog.Warn("auto-link booking to itinerary failed", "error", createErr, "booking_id", b.ID, "trip_id", tripID)
		return
	}
	slog.Info("auto-linked booking to itinerary", "booking_id", b.ID, "trip_id", tripID, "type", itemType)
}
