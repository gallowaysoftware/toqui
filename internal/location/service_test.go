package location

import (
	"context"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

// mockHTTPClient implements HTTPClient for testing.
type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

func jsonResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestGetNearby_NoAPIKey(t *testing.T) {
	svc := NewService("")

	_, err := svc.GetNearby(context.Background(), 35.6762, 139.6503, "restaurants", 1000)
	if err == nil {
		t.Fatal("expected error when API key is not configured")
	}
	if !strings.Contains(err.Error(), "GOOGLE_PLACES_API_KEY") {
		t.Errorf("expected error to mention GOOGLE_PLACES_API_KEY, got: %s", err.Error())
	}
}

func TestGetNearby_EmptyCategory(t *testing.T) {
	svc := NewService("test-key")

	_, err := svc.GetNearby(context.Background(), 35.6762, 139.6503, "", 1000)
	if err == nil {
		t.Fatal("expected error for empty category")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected error to mention 'required', got: %s", err.Error())
	}
}

func TestGetNearby_Success(t *testing.T) {
	client := &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			// Verify the request URL contains expected parameters
			url := req.URL.String()
			if !strings.Contains(url, "nearbysearch") {
				t.Errorf("expected nearbysearch in URL, got: %s", url)
			}
			if !strings.Contains(url, "keyword=restaurants") {
				t.Errorf("expected keyword=restaurants in URL, got: %s", url)
			}
			if !strings.Contains(url, "key=test-key") {
				t.Errorf("expected key=test-key in URL, got: %s", url)
			}

			return jsonResponse(http.StatusOK, `{
				"status": "OK",
				"results": [
					{
						"name": "Sushi Bar",
						"place_id": "ChIJ123",
						"vicinity": "123 Main St, Tokyo",
						"rating": 4.5,
						"types": ["restaurant", "food"],
						"geometry": {
							"location": {"lat": 35.677, "lng": 139.651}
						}
					},
					{
						"name": "Ramen House",
						"place_id": "ChIJ456",
						"vicinity": "456 Side St, Tokyo",
						"rating": 4.2,
						"types": ["restaurant"],
						"geometry": {
							"location": {"lat": 35.678, "lng": 139.652}
						}
					}
				]
			}`), nil
		},
	}

	svc := NewService("test-key", WithHTTPClient(client))
	places, err := svc.GetNearby(context.Background(), 35.6762, 139.6503, "restaurants", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(places) != 2 {
		t.Fatalf("expected 2 places, got %d", len(places))
	}

	// Check first result
	if places[0].Name != "Sushi Bar" {
		t.Errorf("expected name %q, got %q", "Sushi Bar", places[0].Name)
	}
	if places[0].GooglePlaceID != "ChIJ123" {
		t.Errorf("expected place_id %q, got %q", "ChIJ123", places[0].GooglePlaceID)
	}
	if places[0].Address != "123 Main St, Tokyo" {
		t.Errorf("expected address %q, got %q", "123 Main St, Tokyo", places[0].Address)
	}
	if places[0].Rating != 4.5 {
		t.Errorf("expected rating 4.5, got %f", places[0].Rating)
	}
	if places[0].Category != "restaurant" {
		t.Errorf("expected category %q, got %q", "restaurant", places[0].Category)
	}

	// Distance should be non-zero
	if places[0].DistanceM == 0 {
		t.Error("expected non-zero distance")
	}
}

func TestGetNearby_ZeroResults(t *testing.T) {
	client := &mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{
				"status": "ZERO_RESULTS",
				"results": []
			}`), nil
		},
	}

	svc := NewService("test-key", WithHTTPClient(client))
	places, err := svc.GetNearby(context.Background(), 0.0, 0.0, "restaurants", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(places) != 0 {
		t.Errorf("expected 0 places, got %d", len(places))
	}
}

func TestGetNearby_APIError(t *testing.T) {
	client := &mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `{
				"status": "REQUEST_DENIED",
				"error_message": "The provided API key is invalid."
			}`), nil
		},
	}

	svc := NewService("bad-key", WithHTTPClient(client))
	_, err := svc.GetNearby(context.Background(), 35.6762, 139.6503, "restaurants", 1000)
	if err == nil {
		t.Fatal("expected error for REQUEST_DENIED status")
	}
	if !strings.Contains(err.Error(), "REQUEST_DENIED") {
		t.Errorf("expected error to contain REQUEST_DENIED, got: %s", err.Error())
	}
}

func TestGetNearby_HTTPError(t *testing.T) {
	client := &mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusInternalServerError, `Internal Server Error`), nil
		},
	}

	svc := NewService("test-key", WithHTTPClient(client))
	_, err := svc.GetNearby(context.Background(), 35.6762, 139.6503, "restaurants", 1000)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status 500, got: %s", err.Error())
	}
}

func TestGetNearby_Caching(t *testing.T) {
	callCount := 0
	client := &mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			callCount++
			return jsonResponse(http.StatusOK, `{
				"status": "OK",
				"results": [{
					"name": "Test Place",
					"place_id": "ChIJtest",
					"vicinity": "Test Address",
					"rating": 4.0,
					"types": ["restaurant"],
					"geometry": {"location": {"lat": 35.677, "lng": 139.651}}
				}]
			}`), nil
		},
	}

	svc := NewService("test-key", WithHTTPClient(client))

	// First call should hit the API
	places1, err := svc.GetNearby(context.Background(), 35.6762, 139.6503, "restaurants", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 API call, got %d", callCount)
	}

	// Second call with same params should use cache
	places2, err := svc.GetNearby(context.Background(), 35.6762, 139.6503, "restaurants", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected cache hit (1 API call), got %d", callCount)
	}
	if len(places1) != len(places2) {
		t.Errorf("cached result length mismatch: %d vs %d", len(places1), len(places2))
	}

	// Different query should hit the API again
	_, err = svc.GetNearby(context.Background(), 35.6762, 139.6503, "coffee", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls (different query), got %d", callCount)
	}
}

func TestGetNearby_CacheCoordinateRounding(t *testing.T) {
	callCount := 0
	client := &mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			callCount++
			return jsonResponse(http.StatusOK, `{
				"status": "OK",
				"results": []
			}`), nil
		},
	}

	svc := NewService("test-key", WithHTTPClient(client))

	// First call
	_, _ = svc.GetNearby(context.Background(), 35.67624, 139.65031, "restaurants", 1000)
	if callCount != 1 {
		t.Fatalf("expected 1 API call, got %d", callCount)
	}

	// Slightly different coords that round to the same value should be a cache hit
	_, _ = svc.GetNearby(context.Background(), 35.67629, 139.65039, "restaurants", 1000)
	if callCount != 1 {
		t.Errorf("expected cache hit for rounded coords (1 call), got %d", callCount)
	}

	// Significantly different coords should miss
	_, _ = svc.GetNearby(context.Background(), 35.680, 139.660, "restaurants", 1000)
	if callCount != 2 {
		t.Errorf("expected cache miss for different coords (2 calls), got %d", callCount)
	}
}

func TestGetNearby_MaxResults(t *testing.T) {
	// Build a response with 25 results (more than maxNearbyResults = 20)
	results := `{"status":"OK","results":[`
	for i := range 25 {
		if i > 0 {
			results += ","
		}
		results += `{"name":"Place","place_id":"id","vicinity":"addr","rating":4.0,"types":["restaurant"],"geometry":{"location":{"lat":35.0,"lng":139.0}}}`
	}
	results += `]}`

	client := &mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, results), nil
		},
	}

	svc := NewService("test-key", WithHTTPClient(client))
	places, err := svc.GetNearby(context.Background(), 35.0, 139.0, "restaurants", 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(places) != maxNearbyResults {
		t.Errorf("expected max %d places, got %d", maxNearbyResults, len(places))
	}
}

func TestNearbyCacheKey(t *testing.T) {
	// Same coords rounded should produce same key
	k1 := nearbyCacheKey(35.67624, 139.65031, "restaurants", 1000)
	k2 := nearbyCacheKey(35.67629, 139.65039, "restaurants", 1000)
	if k1 != k2 {
		t.Errorf("expected same cache key for nearby coords, got %q vs %q", k1, k2)
	}

	// Different query should produce different key
	k3 := nearbyCacheKey(35.676, 139.650, "coffee", 1000)
	if k1 == k3 {
		t.Error("expected different cache key for different query")
	}

	// Different radius should produce different key
	k4 := nearbyCacheKey(35.676, 139.650, "restaurants", 2000)
	if k1 == k4 {
		t.Error("expected different cache key for different radius")
	}
}

func TestHaversineDistance(t *testing.T) {
	// Known distance: Tokyo to Yokohama is ~27 km
	dist := haversineDistance(35.6762, 139.6503, 35.4437, 139.6380)

	// Allow 1 km tolerance
	if math.Abs(dist-25860) > 1000 {
		t.Errorf("expected ~25860 m (Tokyo-Yokohama), got %.0f m", dist)
	}

	// Same point should be 0
	dist = haversineDistance(35.6762, 139.6503, 35.6762, 139.6503)
	if dist != 0 {
		t.Errorf("expected 0 distance for same point, got %f", dist)
	}
}

func TestGetNearby_CacheExpiry(t *testing.T) {
	svc := NewService("test-key")

	// Manually insert an expired cache entry
	key := nearbyCacheKey(35.676, 139.650, "restaurants", 1000)
	svc.nearbyMu.Lock()
	svc.nearbyCache[key] = &nearbyCacheEntry{
		places:    []Place{{Name: "Expired Place"}},
		expiresAt: time.Now().Add(-1 * time.Minute), // already expired
	}
	svc.nearbyMu.Unlock()

	// getCached should return nil for expired entry
	if cached := svc.getCached(key); cached != nil {
		t.Error("expected nil for expired cache entry")
	}
}

func TestGetNearby_InvalidJSON(t *testing.T) {
	client := &mockHTTPClient{
		doFunc: func(_ *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusOK, `not valid json`), nil
		},
	}

	svc := NewService("test-key", WithHTTPClient(client))
	_, err := svc.GetNearby(context.Background(), 35.676, 139.650, "restaurants", 1000)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}
