package location

import (
	"context"
)

type Place struct {
	Name          string
	Description   string
	Category      string
	Latitude      float64
	Longitude     float64
	Address       string
	DistanceM     float64
	Rating        float64
	GooglePlaceID string
}

type Service struct {
	// Will hold Google Maps client when configured
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) GetNearby(_ context.Context, lat, lng float64, category string, radiusM int) ([]Place, error) {
	// TODO: Integrate with Google Maps Places API
	// For now, return empty results
	return nil, nil
}
