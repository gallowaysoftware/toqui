package location

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Place represents a nearby point of interest returned by GetNearby.
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

// nearbyCacheEntry holds cached GetNearby results for a given location+query.
type nearbyCacheEntry struct {
	places    []Place
	expiresAt time.Time
}

const (
	// nearbyCacheTTL is how long nearby-place results are cached.
	nearbyCacheTTL = 5 * time.Minute

	// nearbyCacheMaxEntries is the maximum number of cached query results.
	nearbyCacheMaxEntries = 500

	// maxNearbyResults caps the number of places returned to the caller.
	maxNearbyResults = 20

	// maxResponseBytes limits the body size read from the Places API.
	maxResponseBytes = 2 << 20 // 2 MB
)

// HTTPClient is the interface used by Service to make HTTP requests.
// It exists to allow injecting a mock client in tests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Service provides location-related operations including nearby place search.
type Service struct {
	apiKey string
	client HTTPClient

	nearbyMu    sync.RWMutex
	nearbyCache map[string]*nearbyCacheEntry
}

// NewService creates a location service. If apiKey is empty, GetNearby
// returns an error indicating the feature is not configured.
func NewService(apiKey string, opts ...ServiceOption) *Service {
	s := &Service{
		apiKey:      apiKey,
		client:      &http.Client{Timeout: 15 * time.Second},
		nearbyCache: make(map[string]*nearbyCacheEntry),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// ServiceOption configures the location Service.
type ServiceOption func(*Service)

// WithHTTPClient sets a custom HTTP client (useful for testing).
func WithHTTPClient(c HTTPClient) ServiceOption {
	return func(s *Service) {
		s.client = c
	}
}

// nearbyCacheKey builds a deterministic cache key from the search parameters.
// Coordinates are rounded to ~110 m precision to group nearby requests.
func nearbyCacheKey(lat, lng float64, query string, radiusM int) string {
	// Round to 3 decimal places (~111 m at equator) so slight GPS jitter
	// doesn't bypass the cache.
	rlat := math.Round(lat*1000) / 1000
	rlng := math.Round(lng*1000) / 1000
	return fmt.Sprintf("%.3f:%.3f:%s:%d", rlat, rlng, query, radiusM)
}

// GetNearby searches for nearby places around the given coordinates using
// the Google Places API Nearby Search endpoint. Results are cached for 5
// minutes to avoid redundant API calls for the same location.
func (s *Service) GetNearby(ctx context.Context, lat, lng float64, category string, radiusM int) ([]Place, error) {
	if s.apiKey == "" {
		return nil, fmt.Errorf("nearby places not available: GOOGLE_PLACES_API_KEY is not configured")
	}

	if category == "" {
		return nil, fmt.Errorf("category/query is required")
	}

	// Check cache first.
	key := nearbyCacheKey(lat, lng, category, radiusM)
	if cached := s.getCached(key); cached != nil {
		slog.Debug("nearby places cache hit", "key", key)
		return cached, nil
	}

	// Build the Google Places Nearby Search (legacy) request.
	// This uses the same API key format as the existing PlaceLookup tool.
	reqURL := fmt.Sprintf(
		"https://maps.googleapis.com/maps/api/place/nearbysearch/json?location=%f,%f&radius=%d&keyword=%s&key=%s",
		lat, lng, radiusM, url.QueryEscape(category), s.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nearby search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("google places API error",
			"status", resp.StatusCode,
			"body", string(body),
		)
		return nil, fmt.Errorf("google places API returned status %d", resp.StatusCode)
	}

	var apiResp struct {
		Status  string `json:"status"`
		Results []struct {
			Name     string   `json:"name"`
			PlaceID  string   `json:"place_id"`
			Vicinity string   `json:"vicinity"`
			Rating   float64  `json:"rating"`
			Types    []string `json:"types"`
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
		} `json:"results"`
		ErrorMessage string `json:"error_message"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parse places response: %w", err)
	}

	if apiResp.Status != "OK" && apiResp.Status != "ZERO_RESULTS" {
		slog.Error("google places API status error",
			"status", apiResp.Status,
			"error_message", apiResp.ErrorMessage,
		)
		return nil, fmt.Errorf("google places API error: %s", apiResp.Status)
	}

	places := make([]Place, 0, len(apiResp.Results))
	for _, r := range apiResp.Results {
		if len(places) >= maxNearbyResults {
			break
		}

		cat := ""
		if len(r.Types) > 0 {
			cat = r.Types[0]
		}

		places = append(places, Place{
			Name:          r.Name,
			Category:      cat,
			Address:       r.Vicinity,
			Latitude:      r.Geometry.Location.Lat,
			Longitude:     r.Geometry.Location.Lng,
			Rating:        r.Rating,
			GooglePlaceID: r.PlaceID,
			DistanceM:     haversineDistance(lat, lng, r.Geometry.Location.Lat, r.Geometry.Location.Lng),
		})
	}

	// Cache the results.
	s.setCached(key, places)

	slog.Info("nearby places search completed",
		"query", category,
		"radius_m", radiusM,
		"results", len(places),
	)

	return places, nil
}

// getCached returns cached places for the key, or nil if absent/expired.
func (s *Service) getCached(key string) []Place {
	s.nearbyMu.RLock()
	defer s.nearbyMu.RUnlock()

	entry, ok := s.nearbyCache[key]
	if !ok {
		return nil
	}
	if time.Now().After(entry.expiresAt) {
		return nil
	}
	return entry.places
}

// setCached stores places in the cache. If the cache exceeds its max size,
// all expired entries are purged first.
func (s *Service) setCached(key string, places []Place) {
	s.nearbyMu.Lock()
	defer s.nearbyMu.Unlock()

	// Evict expired entries if we're at capacity.
	if len(s.nearbyCache) >= nearbyCacheMaxEntries {
		now := time.Now()
		for k, v := range s.nearbyCache {
			if now.After(v.expiresAt) {
				delete(s.nearbyCache, k)
			}
		}
	}

	s.nearbyCache[key] = &nearbyCacheEntry{
		places:    places,
		expiresAt: time.Now().Add(nearbyCacheTTL),
	}
}

// haversineDistance returns the distance in meters between two lat/lng points.
func haversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6_371_000.0

	dLat := degreesToRadians(lat2 - lat1)
	dLng := degreesToRadians(lng2 - lng1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(degreesToRadians(lat1))*math.Cos(degreesToRadians(lat2))*
			math.Sin(dLng/2)*math.Sin(dLng/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusM * c
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180
}
