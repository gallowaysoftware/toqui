package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// geocodeHTTPClient is the shared HTTP client for geocoding requests.
// 5-second timeout per request to avoid blocking the background worker.
var geocodeHTTPClient = &http.Client{Timeout: 5 * time.Second}

// geocodeLocation resolves a human-readable location name to WGS-84 coordinates
// using the Google Geocoding API. Returns (0, 0, nil) when apiKey is empty so
// callers can silently skip geocoding without special-casing.
func geocodeLocation(ctx context.Context, apiKey, locationName string) (lat, lng float64, err error) {
	if apiKey == "" || locationName == "" {
		return 0, 0, nil
	}

	reqURL := fmt.Sprintf(
		"https://maps.googleapis.com/maps/api/geocode/json?address=%s&key=%s",
		url.QueryEscape(locationName),
		apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("build geocode request: %w", err)
	}

	resp, err := geocodeHTTPClient.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return 0, 0, fmt.Errorf("read geocode response: %w", err)
	}

	var result struct {
		Status  string `json:"status"`
		Results []struct {
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, 0, fmt.Errorf("parse geocode response: %w", err)
	}

	if result.Status != "OK" || len(result.Results) == 0 {
		return 0, 0, fmt.Errorf("geocode returned status %q with %d results", result.Status, len(result.Results))
	}

	loc := result.Results[0].Geometry.Location
	return loc.Lat, loc.Lng, nil
}
