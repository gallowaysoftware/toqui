package handlers

import (
	"context"

	"connectrpc.com/connect"
	"github.com/gallowaysoftware/toqui-backend/internal/location"

	toquiv1 "github.com/gallowaysoftware/toqui-backend/gen/toqui/v1"
)

type LocationHandler struct {
	locationSvc *location.Service
}

func NewLocationHandler(locationSvc *location.Service) *LocationHandler {
	return &LocationHandler{locationSvc: locationSvc}
}

func (h *LocationHandler) UpdateLocation(ctx context.Context, req *connect.Request[toquiv1.UpdateLocationRequest]) (*connect.Response[toquiv1.UpdateLocationResponse], error) {
	// TODO: process location update and return suggestions
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
		return nil, connect.NewError(connect.CodeInternal, err)
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
