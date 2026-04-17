package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	queries      *dbgen.Queries
}

func NewTripHandler(tripSvc *trip.Service, lifecycleSvc *lifecycle.Service, themeSvc *theme.Service, queries *dbgen.Queries) *TripHandler {
	return &TripHandler{tripSvc: tripSvc, lifecycleSvc: lifecycleSvc, themeSvc: themeSvc, queries: queries}
}

func (h *TripHandler) CreateTrip(ctx context.Context, req *connect.Request[toquiv1.CreateTripRequest]) (*connect.Response[toquiv1.CreateTripResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	var startDate, endDate *time.Time
	if req.Msg.StartDate != "" {
		t, err := time.Parse("2006-01-02", req.Msg.StartDate)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid start_date format, expected YYYY-MM-DD: %w", err))
		}
		startDate = &t
	}
	if req.Msg.EndDate != "" {
		t, err := time.Parse("2006-01-02", req.Msg.EndDate)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid end_date format, expected YYYY-MM-DD: %w", err))
		}
		endDate = &t
	}

	// Honour the optional initial status (Run 4 N-01 P1). When unset or
	// TRIP_STATUS_PLANNING the default database value is used. Only PLANNING
	// and ACTIVE are accepted as initial values — COMPLETED is rejected at
	// the service layer because a trip cannot start in the terminal state.
	initialStatus := ""
	if req.Msg.Status != toquiv1.TripStatus_TRIP_STATUS_UNSPECIFIED {
		initialStatus = tripStatusToString(req.Msg.Status)
	}
	t, err := h.tripSvc.CreateWithStatus(ctx, userID, req.Msg.Title, req.Msg.Description, startDate, endDate, initialStatus)
	if err != nil {
		if errors.Is(err, trip.ErrInvalidInitialStatus) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, internalError(ctx, "trip operation", err)
	}

	// If the client provided budget fields at creation, apply them via Update
	// (the CreateTrip SQL query doesn't include budget columns).
	if req.Msg.BudgetCents != nil || req.Msg.Currency != "" {
		updated, err := h.tripSvc.Update(ctx, userID, t.ID, "", "", "", nil, nil, req.Msg.BudgetCents, req.Msg.Currency, "", "", "")
		if err != nil {
			slog.Warn("failed to set budget on newly created trip", "trip_id", t.ID, "error", err)
		} else {
			t = updated
		}
	}

	// Fire-and-forget: tag trip themes via AI
	if h.themeSvc != nil {
		h.themeSvc.TagTripAsync(userID, t.ID, t.Title, t.Description.String)
	}

	// Auto-grant 3-day Pro trial on first trip creation
	if h.queries != nil {
		if count, err := h.queries.CountTripsByUser(ctx, userID); err == nil && count == 1 {
			if err := h.queries.StartTripTrial(ctx, t.ID); err != nil {
				slog.Warn("failed to start trip trial", "error", err, "trip_id", t.ID)
			} else {
				slog.Info("first-trip trial started", "user_id", userID, "trip_id", t.ID)
			}
		}
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

	// Try owner access first, then collaborator access.
	t, err := h.tripSvc.GetByIDOrCollaborator(ctx, userID, tripID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("trip not found"))
		}
		return nil, internalError(ctx, "get trip", err)
	}

	var themes []string
	if h.themeSvc != nil {
		themes, _ = h.themeSvc.GetTripThemes(ctx, tripID)
	}
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

	limit := clampPageSize(req.Msg.GetPagination().GetPageSize(), 20, 100)

	query := strings.TrimSpace(req.Msg.GetQuery())

	// When a search query is provided, use full-text search instead of the
	// normal list path. Search results are ranked by relevance and do not
	// include shared trips (the user is searching their own trips).
	if query != "" {
		results, err := h.tripSvc.SearchByUser(ctx, userID, query, limit)
		if err != nil {
			return nil, internalError(ctx, "search trips", err)
		}
		protoTrips := make([]*toquiv1.Trip, len(results))
		for i, t := range results {
			protoTrips[i] = tripToProto(&t)
		}
		return connect.NewResponse(&toquiv1.ListTripsResponse{
			Trips: protoTrips,
			Pagination: &toquiv1.PaginationResponse{
				TotalCount: int32(len(results)),
			},
		}), nil
	}

	offset := int32(0)

	trips, count, err := h.tripSvc.ListByUser(ctx, userID, status, limit, offset)
	if err != nil {
		return nil, internalError(ctx, "trip operation", err)
	}

	protoTrips := make([]*toquiv1.Trip, len(trips))
	for i, t := range trips {
		protoTrips[i] = tripToProto(&t)
	}

	// Also include trips shared with this user (collaborator trips).
	// Only include when not filtering by status (to keep the filtered view clean).
	if status == "" {
		sharedTrips, err := h.tripSvc.ListSharedTrips(ctx, userID)
		if err != nil {
			slog.Warn("list shared trips failed", "error", err, "user_id", userID)
		} else {
			// Deduplicate: shared trips should not overlap with owned trips,
			// but guard against it anyway.
			seen := make(map[string]bool, len(protoTrips))
			for _, pt := range protoTrips {
				seen[pt.Id] = true
			}
			for _, st := range sharedTrips {
				if !seen[st.ID.String()] {
					protoTrips = append(protoTrips, tripToProto(&st))
					count++
				}
			}
		}
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
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid start_date format, expected YYYY-MM-DD: %w", err))
		}
		startDate = &t
	}
	if req.Msg.EndDate != "" {
		t, err := time.Parse("2006-01-02", req.Msg.EndDate)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid end_date format, expected YYYY-MM-DD: %w", err))
		}
		endDate = &t
	}

	status := tripStatusToString(req.Msg.Status)

	// Budget fields — proto optional int64 produces a pointer when set.
	var budgetCents *int64
	if req.Msg.BudgetCents != nil {
		budgetCents = req.Msg.BudgetCents
	}

	t, err := h.tripSvc.Update(ctx, userID, tripID, req.Msg.Title, req.Msg.Description, status, startDate, endDate, budgetCents, req.Msg.Currency, req.Msg.Notes, req.Msg.CoverImageUrl, req.Msg.Timezone)
	if err != nil {
		if errors.Is(err, trip.ErrInvalidStatusTransition) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, internalError(ctx, "trip operation", err)
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

	// Verify the trip exists and belongs to the caller before deleting.
	// Returning NotFound for missing trips makes the API behave correctly
	// against concurrent deletes and double-clicks instead of silently
	// succeeding (#188).
	if _, err := h.tripSvc.GetByID(ctx, userID, tripID); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("trip not found"))
	}

	// Use lifecycle service to purge Firestore chat data + Postgres
	if err := h.lifecycleSvc.DeleteTrip(ctx, userID, tripID); err != nil {
		return nil, internalError(ctx, "trip operation", err)
	}

	return connect.NewResponse(&toquiv1.DeleteTripResponse{}), nil
}

func (h *TripHandler) CloneTrip(ctx context.Context, req *connect.Request[toquiv1.CloneTripRequest]) (*connect.Response[toquiv1.CloneTripResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	tripID, err := uuid.Parse(req.Msg.TripId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	cloned, err := h.tripSvc.CloneTrip(ctx, userID, tripID, req.Msg.Title)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("trip not found"))
		}
		return nil, internalError(ctx, "clone trip", err)
	}

	// Fire-and-forget: tag themes for the cloned trip.
	if h.themeSvc != nil {
		h.themeSvc.TagTripAsync(userID, cloned.ID, cloned.Title, cloned.Description.String)
	}

	return connect.NewResponse(&toquiv1.CloneTripResponse{Trip: tripToProto(cloned)}), nil
}

func (h *TripHandler) GetItinerary(ctx context.Context, req *connect.Request[toquiv1.GetItineraryRequest]) (*connect.Response[toquiv1.GetItineraryResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	tripID, err := uuid.Parse(req.Msg.TripId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Verify trip ownership or collaborator access before returning itinerary.
	if _, err := h.tripSvc.GetByIDOrCollaborator(ctx, userID, tripID); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	items, err := h.tripSvc.GetItinerary(ctx, tripID)
	if err != nil {
		return nil, internalError(ctx, "trip operation", err)
	}

	// Best-effort: fetch coordinates for geocoded items. Failure just means the
	// map shows no pins — it never blocks the itinerary response.
	coordsMap := make(map[uuid.UUID]trip.ItineraryItemCoords)
	if coords, err := h.tripSvc.GetItineraryCoords(ctx, tripID); err == nil {
		for _, c := range coords {
			coordsMap[c.ID] = c
		}
	} else {
		slog.Warn("get itinerary coords failed, map pins unavailable", "trip_id", tripID, "error", err)
	}

	return connect.NewResponse(&toquiv1.GetItineraryResponse{
		Itinerary: itineraryToProto(req.Msg.TripId, items, coordsMap),
	}), nil
}

func (h *TripHandler) UpdateItinerary(ctx context.Context, req *connect.Request[toquiv1.UpdateItineraryRequest]) (*connect.Response[toquiv1.UpdateItineraryResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	tripID, err := uuid.Parse(req.Msg.TripId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Access check — trip owners and editor-role collaborators can rewrite
	// itineraries. Viewers and non-collaborators are rejected (#263).
	if _, err := h.tripSvc.GetByIDOrCollaborator(ctx, userID, tripID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("trip not found"))
		}
		return nil, internalError(ctx, "get trip", err)
	}
	canEdit, err := h.tripSvc.CanEditTrip(ctx, userID, tripID)
	if err != nil {
		return nil, internalError(ctx, "check edit access", err)
	}
	if !canEdit {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("editor access required to modify itinerary"))
	}

	itin := req.Msg.GetItinerary()
	if itin == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("itinerary is required"))
	}

	// UpdateItinerary is a full rewrite: delete existing items for the trip,
	// then insert the new items in order. Previously the RPC was an
	// unimplemented stub (Run 4 N-05 P1) which meant clients had no
	// non-AI path to manage itinerary content.
	var flat []trip.ReplaceItineraryItem
	for _, day := range itin.GetDays() {
		for _, item := range day.GetItems() {
			ri := trip.ReplaceItineraryItem{
				DayNumber:    int(day.GetDayNumber()),
				OrderInDay:   int(item.GetOrderInDay()),
				Type:         item.GetType(),
				Title:        item.GetTitle(),
				Description:  item.GetDescription(),
				DaySummary:   day.GetSummary(),
				DayDate:      day.GetDate(),
				CostCurrency: item.GetCostCurrency(),
			}
			if item.EstimatedCostCents != nil {
				ri.EstimatedCostCents = item.EstimatedCostCents
			}
			flat = append(flat, ri)
		}
	}
	if err := h.tripSvc.ReplaceItineraryForOwnerOrEditor(ctx, userID, tripID, flat); err != nil {
		// Defence-in-depth: the handler already gates via CanEditTrip
		// above, but the service enforces its own authz precondition
		// so direct service callers (e.g. tools wired up by future
		// code) can't sneak in unauthorised writes (#343).
		if errors.Is(err, trip.ErrNotOwnerOrEditor) {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("editor access required to modify itinerary"))
		}
		return nil, internalError(ctx, "replace itinerary", err)
	}

	items, err := h.tripSvc.GetItinerary(ctx, tripID)
	if err != nil {
		return nil, internalError(ctx, "reload itinerary", err)
	}
	return connect.NewResponse(&toquiv1.UpdateItineraryResponse{
		Itinerary: itineraryToProto(req.Msg.TripId, items, nil),
	}), nil
}

func (h *TripHandler) ReorderItineraryItem(ctx context.Context, req *connect.Request[toquiv1.ReorderItineraryItemRequest]) (*connect.Response[toquiv1.ReorderItineraryItemResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	tripID, err := uuid.Parse(req.Msg.TripId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid trip_id"))
	}
	itemID, err := uuid.Parse(req.Msg.ItemId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid item_id"))
	}

	// Access check: trip owner or editor collaborator.
	canEdit, err := h.tripSvc.CanEditTrip(ctx, userID, tripID)
	if err != nil {
		return nil, internalError(ctx, "check edit access", err)
	}
	if !canEdit {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("editor access required to reorder itinerary"))
	}

	moved, err := h.tripSvc.MoveItineraryItem(ctx, userID, tripID, itemID, int(req.Msg.TargetDay), int(req.Msg.TargetPosition))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("itinerary item not found"))
		}
		return nil, internalError(ctx, "reorder itinerary item", err)
	}

	protoItem := &toquiv1.ItineraryItem{
		Id: moved.ID.String(),
	}
	if moved.Title.Valid {
		protoItem.Title = moved.Title.String
	}
	if moved.OrderInDay.Valid {
		protoItem.OrderInDay = moved.OrderInDay.Int32
	}
	if moved.Type.Valid {
		protoItem.Type = moved.Type.String
	}
	if moved.Description.Valid {
		protoItem.Description = moved.Description.String
	}
	if moved.StartTime.Valid {
		protoItem.StartTime = timestamppb.New(moved.StartTime.Time)
	}
	if moved.EndTime.Valid {
		protoItem.EndTime = timestamppb.New(moved.EndTime.Time)
	}
	if moved.EstimatedCostCents.Valid {
		protoItem.EstimatedCostCents = &moved.EstimatedCostCents.Int64
	}
	if moved.CostCurrency.Valid {
		protoItem.CostCurrency = moved.CostCurrency.String
	}

	return connect.NewResponse(&toquiv1.ReorderItineraryItemResponse{Item: protoItem}), nil
}

func (h *TripHandler) ListTripTemplates(ctx context.Context, req *connect.Request[toquiv1.ListTripTemplatesRequest]) (*connect.Response[toquiv1.ListTripTemplatesResponse], error) {
	// Templates are public — no auth required for listing, but we still want
	// the request to pass through the auth interceptor for rate limiting.
	// If we eventually want auth, we can add it here.
	_, _ = auth.UserIDFromContext(ctx)

	limit := clampPageSize(req.Msg.GetPagination().GetPageSize(), 20, 100)
	offset := int32(0)

	templates, count, err := h.tripSvc.ListTemplates(ctx, limit, offset)
	if err != nil {
		return nil, internalError(ctx, "list templates", err)
	}

	protoTrips := make([]*toquiv1.Trip, len(templates))
	for i, t := range templates {
		protoTrips[i] = tripToProto(&t)
	}

	return connect.NewResponse(&toquiv1.ListTripTemplatesResponse{
		Templates: protoTrips,
		Pagination: &toquiv1.PaginationResponse{
			TotalCount: int32(count),
		},
	}), nil
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
	// Multi-country trips: prefer the array column, falling back to the legacy
	// single-country field so old trips still surface their destination (#133).
	if len(t.DestinationCountries) > 0 {
		proto.DestinationCountries = t.DestinationCountries
	} else if t.DestinationCountry.Valid && t.DestinationCountry.String != "" {
		proto.DestinationCountries = []string{t.DestinationCountry.String}
	}
	if t.BudgetCents.Valid {
		proto.BudgetCents = &t.BudgetCents.Int64
	}
	if t.Currency.Valid && t.Currency.String != "" {
		proto.Currency = t.Currency.String
	}
	if t.Notes.Valid {
		proto.Notes = t.Notes.String
	}
	if t.CoverImageUrl.Valid {
		proto.CoverImageUrl = t.CoverImageUrl.String
	}
	if t.Timezone.Valid {
		proto.Timezone = t.Timezone.String
	}
	proto.IsTemplate = t.IsTemplate
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

// itineraryToProto converts DB itinerary items to the proto representation.
// coordsMap maps item IDs to their geocoded coordinates (may be nil or empty).
func itineraryToProto(tripID string, items []dbgen.ItineraryItem, coordsMap map[uuid.UUID]trip.ItineraryItemCoords) *toquiv1.Itinerary {
	dayMap := make(map[int32]*toquiv1.ItineraryDay)

	for _, item := range items {
		dayNum := int32(0)
		if item.DayNumber.Valid {
			dayNum = item.DayNumber.Int32
		}

		day, ok := dayMap[dayNum]
		if !ok {
			// Default summary is "Day N". If UpdateItinerary stashed a
			// custom summary or date on the item's metadata JSONB we
			// prefer those (Run 5 R-07/N-05 P2). The first item
			// encountered for this day wins — all items for the same day
			// should carry the same day-level metadata, and the sqlc
			// ListItineraryItemsByTrip query orders by day_number so
			// we're reading them grouped.
			//
			// Decode into json.RawMessage so sibling keys of non-string
			// types (numbers, arrays, objects added by future metadata
			// writers) don't kill the whole decode and blank out the
			// summary for every item on this day.
			summary := fmt.Sprintf("Day %d", dayNum)
			if dayNum == 0 {
				summary = "Unscheduled"
			}
			date := ""
			if len(item.Metadata) > 0 {
				var md map[string]json.RawMessage
				if err := json.Unmarshal(item.Metadata, &md); err == nil {
					if raw, ok := md["day_summary"]; ok {
						var v string
						if json.Unmarshal(raw, &v) == nil && v != "" {
							summary = v
						}
					}
					if raw, ok := md["day_date"]; ok {
						var v string
						if json.Unmarshal(raw, &v) == nil && v != "" {
							date = v
						}
					}
				}
			}
			day = &toquiv1.ItineraryDay{
				Id:        uuid.New().String(),
				DayNumber: dayNum,
				Summary:   summary,
				Date:      date,
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
		if c, ok := coordsMap[item.ID]; ok && (c.Latitude != 0 || c.Longitude != 0) {
			protoItem.Location = &toquiv1.LatLng{
				Latitude:  c.Latitude,
				Longitude: c.Longitude,
			}
		}
		if item.EstimatedCostCents.Valid {
			protoItem.EstimatedCostCents = &item.EstimatedCostCents.Int64
		}
		if item.CostCurrency.Valid {
			protoItem.CostCurrency = item.CostCurrency.String
		}

		day.Items = append(day.Items, protoItem)
	}

	days := make([]*toquiv1.ItineraryDay, 0, len(dayMap))
	for _, day := range dayMap {
		days = append(days, day)
	}
	slices.SortFunc(days, func(a, b *toquiv1.ItineraryDay) int {
		return int(a.DayNumber) - int(b.DayNumber)
	})

	return &toquiv1.Itinerary{
		TripId: tripID,
		Days:   days,
	}
}
