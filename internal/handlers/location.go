package handlers

import (
	"context"
	"log/slog"

	"connectrpc.com/connect"

	"github.com/gallowaysoftware/toqui-backend/internal/auth"
	"github.com/gallowaysoftware/toqui-backend/internal/location"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
)

type LocationHandler struct {
	locationSvc   *location.Service
	locationCache *location.Cache
}

func NewLocationHandler(locationSvc *location.Service, locationCache *location.Cache) *LocationHandler {
	return &LocationHandler{locationSvc: locationSvc, locationCache: locationCache}
}

func (h *LocationHandler) UpdateLocation(ctx context.Context, req *connect.Request[toquiv1.UpdateLocationRequest]) (*connect.Response[toquiv1.UpdateLocationResponse], error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	if req.Msg.Location == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}

	// Store location in ephemeral cache for companion mode context injection.
	// PRIVACY: This is never persisted to a database — it lives only in memory
	// and expires after the cache TTL (30 minutes).
	if h.locationCache != nil {
		h.locationCache.Set(userID, req.Msg.Location.Latitude, req.Msg.Location.Longitude, 0)
		slog.Debug("cached user location",
			"user_id", userID,
			"lat", req.Msg.Location.Latitude,
			"lng", req.Msg.Location.Longitude,
		)
	}

	return connect.NewResponse(&toquiv1.UpdateLocationResponse{}), nil
}

func (h *LocationHandler) GetNearby(ctx context.Context, req *connect.Request[toquiv1.GetNearbyRequest]) (*connect.Response[toquiv1.GetNearbyResponse], error) {
	if req.Msg.Location == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, nil)
	}

	radiusM := int(req.Msg.RadiusMeters)
	if radiusM == 0 {
		radiusM = 1000
	}

	places, err := h.locationSvc.GetNearby(ctx, req.Msg.Location.Latitude, req.Msg.Location.Longitude, req.Msg.Category, radiusM)
	if err != nil {
		return nil, internalError(ctx, "get nearby", err)
	}

	protoPlaces := make([]*toquiv1.NearbyPlace, len(places))
	for i, p := range places {
		protoPlaces[i] = &toquiv1.NearbyPlace{
			Name:        p.Name,
			Description: p.Description,
			Category:    p.Category,
			Location: &toquiv1.LatLng{
				Latitude:  p.Latitude,
				Longitude: p.Longitude,
			},
			Address:        p.Address,
			DistanceMeters: p.DistanceM,
			Rating:         p.Rating,
			GooglePlaceId:  p.GooglePlaceID,
		}
	}

	return connect.NewResponse(&toquiv1.GetNearbyResponse{
		Places: protoPlaces,
	}), nil
}
