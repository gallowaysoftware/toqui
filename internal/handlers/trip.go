package handlers

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/dbgen"
	"github.com/gallowaysoftware/toqui-backend/internal/lifecycle"
	"github.com/gallowaysoftware/toqui-backend/internal/theme"
	"github.com/gallowaysoftware/toqui-backend/internal/trip"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
)

type TripHandler struct {
	tripSvc      *trip.Service
	lifecycleSvc *lifecycle.Service
	themeSvc     *theme.Service
}

func NewTripHandler(tripSvc *trip.Service, lifecycleSvc *lifecycle.Service, themeSvc *theme.Service) *TripHandler {
	return &TripHandler{tripSvc: tripSvc, lifecycleSvc: lifecycleSvc, themeSvc: themeSvc}
}

func (h *TripHandler) CreateTrip(ctx context.Context, req *connect.Request[toquiv1.CreateTripRequest]) (*connect.Response[toquiv1.CreateTripResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	var startDate, endDate *time.Time
	if req.Msg.StartDate != "" {
		t, err := time.Parse("2006-01-02", req.Msg.StartDate)
		if err == nil {
			startDate = &t
		}
	}
	if req.Msg.EndDate != "" {
		t, err := time.Parse("2006-01-02", req.Msg.EndDate)
		if err == nil {
			endDate = &t
		}
	}

	t, err := h.tripSvc.Create(ctx, userID, req.Msg.Title, req.Msg.Description, startDate, endDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Fire-and-forget: tag trip themes via AI
	if h.themeSvc != nil {
		h.themeSvc.TagTripAsync(t.ID, t.Title, t.Description.String)
	}

	return connect.NewResponse(&toquiv1.CreateTripResponse{Trip: tripToProto(t)}), nil
}

func (h *TripHandler) GetTrip(ctx context.Context, req *connect.Request[toquiv1.GetTripRequest]) (*connect.Response[toquiv1.GetTripResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	tripID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	t, err := h.tripSvc.GetByID(ctx, userID, tripID)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	themes, _ := h.themeSvc.GetTripThemes(ctx, tripID)
	return connect.NewResponse(&toquiv1.GetTripResponse{Trip: tripToProtoWithThemes(t, themes)}), nil
}

func (h *TripHandler) ListTrips(ctx context.Context, req *connect.Request[toquiv1.ListTripsRequest]) (*connect.Response[toquiv1.ListTripsResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	var status string
	if req.Msg.Status != toquiv1.TripStatus_TRIP_STATUS_UNSPECIFIED {
		status = tripStatusToString(req.Msg.Status)
	}

	limit := int32(20)
	offset := int32(0)
	if req.Msg.Pagination != nil {
		if req.Msg.Pagination.PageSize > 0 {
			limit = req.Msg.Pagination.PageSize
		}
	}

	trips, count, err := h.tripSvc.ListByUser(ctx, userID, status, limit, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	protoTrips := make([]*toquiv1.Trip, len(trips))
	for i, t := range trips {
		protoTrips[i] = tripToProto(&t)
	}

	return connect.NewResponse(&toquiv1.ListTripsResponse{
		Trips: protoTrips,
		Pagination: &toquiv1.PaginationResponse{
			TotalCount: int32(count),
		},
	}), nil
}

func (h *TripHandler) UpdateTrip(ctx context.Context, req *connect.Request[toquiv1.UpdateTripRequest]) (*connect.Response[toquiv1.UpdateTripResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	tripID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	var startDate, endDate *time.Time
	if req.Msg.StartDate != "" {
		t, err := time.Parse("2006-01-02", req.Msg.StartDate)
		if err == nil {
			startDate = &t
		}
	}
	if req.Msg.EndDate != "" {
		t, err := time.Parse("2006-01-02", req.Msg.EndDate)
		if err == nil {
			endDate = &t
		}
	}

	status := tripStatusToString(req.Msg.Status)

	t, err := h.tripSvc.Update(ctx, userID, tripID, req.Msg.Title, req.Msg.Description, status, startDate, endDate)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// When trip is completed, stamp TTL on chat data (90-day retention)
	if status == "completed" {
		h.lifecycleSvc.SetChatTTLAsync(userID, tripID, 90)
	}

	return connect.NewResponse(&toquiv1.UpdateTripResponse{Trip: tripToProto(t)}), nil
}

func (h *TripHandler) DeleteTrip(ctx context.Context, req *connect.Request[toquiv1.DeleteTripRequest]) (*connect.Response[toquiv1.DeleteTripResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	tripID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Use lifecycle service to purge Firestore chat data + Postgres
	if err := h.lifecycleSvc.DeleteTrip(ctx, userID, tripID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&toquiv1.DeleteTripResponse{}), nil
}

func (h *TripHandler) GetItinerary(ctx context.Context, req *connect.Request[toquiv1.GetItineraryRequest]) (*connect.Response[toquiv1.GetItineraryResponse], error) {
	tripID, err := uuid.Parse(req.Msg.TripId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	items, err := h.tripSvc.GetItinerary(ctx, tripID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&toquiv1.GetItineraryResponse{
		Itinerary: itineraryToProto(req.Msg.TripId, items),
	}), nil
}

func (h *TripHandler) UpdateItinerary(ctx context.Context, req *connect.Request[toquiv1.UpdateItineraryRequest]) (*connect.Response[toquiv1.UpdateItineraryResponse], error) {
	// TODO: implement itinerary update
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func tripToProto(t *dbgen.Trip) *toquiv1.Trip {
	proto := &toquiv1.Trip{
		Id:        t.ID.String(),
		UserId:    t.UserID.String(),
		Title:     t.Title,
		Status:    stringToTripStatus(t.Status),
		CreatedAt: timestamppb.New(t.CreatedAt),
		UpdatedAt: timestamppb.New(t.UpdatedAt),
	}
	if t.Description.Valid {
		proto.Description = t.Description.String
	}
	if t.StartDate.Valid {
		proto.StartDate = t.StartDate.Time.Format("2006-01-02")
	}
	if t.EndDate.Valid {
		proto.EndDate = t.EndDate.Time.Format("2006-01-02")
	}
	if t.DestinationCountry.Valid {
		proto.DestinationCountry = t.DestinationCountry.String
	}
	return proto
}

func tripToProtoWithThemes(t *dbgen.Trip, themes []string) *toquiv1.Trip {
	proto := tripToProto(t)
	proto.Themes = themes
	return proto
}

func tripStatusToString(s toquiv1.TripStatus) string {
	switch s {
	case toquiv1.TripStatus_TRIP_STATUS_PLANNING:
		return "planning"
	case toquiv1.TripStatus_TRIP_STATUS_ACTIVE:
		return "active"
	case toquiv1.TripStatus_TRIP_STATUS_COMPLETED:
		return "completed"
	default:
		return "" // UNSPECIFIED — let COALESCE preserve existing value
	}
}

func stringToTripStatus(s string) toquiv1.TripStatus {
	switch s {
	case "planning":
		return toquiv1.TripStatus_TRIP_STATUS_PLANNING
	case "active":
		return toquiv1.TripStatus_TRIP_STATUS_ACTIVE
	case "completed":
		return toquiv1.TripStatus_TRIP_STATUS_COMPLETED
	default:
		return toquiv1.TripStatus_TRIP_STATUS_UNSPECIFIED
	}
}

func itineraryToProto(tripID string, items []dbgen.ItineraryItem) *toquiv1.Itinerary {
	dayMap := make(map[int32]*toquiv1.ItineraryDay)

	for _, item := range items {
		dayNum := int32(0)
		if item.DayNumber.Valid {
			dayNum = item.DayNumber.Int32
		}

		day, ok := dayMap[dayNum]
		if !ok {
			day = &toquiv1.ItineraryDay{
				Id:        uuid.New().String(),
				DayNumber: dayNum,
			}
			dayMap[dayNum] = day
		}

		protoItem := &toquiv1.ItineraryItem{
			Id: item.ID.String(),
		}
		if item.Title.Valid {
			protoItem.Title = item.Title.String
		}
		if item.OrderInDay.Valid {
			protoItem.OrderInDay = item.OrderInDay.Int32
		}
		if item.Type.Valid {
			protoItem.Type = item.Type.String
		}
		if item.Description.Valid {
			protoItem.Description = item.Description.String
		}
		if item.StartTime.Valid {
			protoItem.StartTime = timestamppb.New(item.StartTime.Time)
		}
		if item.EndTime.Valid {
			protoItem.EndTime = timestamppb.New(item.EndTime.Time)
		}

		day.Items = append(day.Items, protoItem)
	}

	days := make([]*toquiv1.ItineraryDay, 0, len(dayMap))
	for _, day := range dayMap {
		days = append(days, day)
	}

	return &toquiv1.Itinerary{
		TripId: tripID,
		Days:   days,
	}
}
