package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestWeatherTool_Definition pins the tool name and parameter schema.
// The AI dispatches by name, so a rename would silently break every
// "what's the weather like in X" prompt. The schema lets the AI pass
// EITHER lat/lng OR city — neither is required at the schema level
// (validation happens at runtime in Execute). Pin that flexibility so a
// future tightening of the schema is a deliberate change.
func TestWeatherTool_Definition(t *testing.T) {
	def := NewWeatherTool().Definition()

	if def.Name != "get_weather" {
		t.Errorf("Name = %q, want get_weather", def.Name)
	}
	if def.Description == "" {
		t.Error("Description must be non-empty (the AI uses it to decide when to call this tool)")
	}

	var schema struct {
		Type       string         `json:"type"`
		Properties map[string]any `json:"properties"`
		Required   []string       `json:"required"`
	}
	if err := json.Unmarshal(def.Parameters, &schema); err != nil {
		t.Fatalf("Parameters not valid JSON Schema: %v", err)
	}
	for _, want := range []string{"latitude", "longitude", "city"} {
		if _, ok := schema.Properties[want]; !ok {
			t.Errorf("schema missing property %q", want)
		}
	}
	// Important: NO field is required at the schema level — the tool
	// supports both lat/lng AND city-name lookups. Runtime validation
	// in Execute returns no_location if both are absent.
	if len(schema.Required) != 0 {
		t.Errorf("Required = %v, want [] (tool accepts either lat/lng or city)", schema.Required)
	}
}

// TestWeatherTool_HappyPath_LatLng covers the simplest case: caller
// provides explicit coordinates. Pin the request shape (URL contains
// lat/lng to 4 decimal places, requests timezone=auto, asks for 7 days)
// and the response shape (current + forecast_7day).
func TestWeatherTool_HappyPath_LatLng(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{
			"timezone": "Asia/Tokyo",
			"current": {
				"temperature_2m": 22.5,
				"relative_humidity_2m": 60,
				"apparent_temperature": 21.0,
				"precipitation": 0.2,
				"weather_code": 2,
				"wind_speed_10m": 5.5
			},
			"daily": {
				"time": ["2026-04-29", "2026-04-30"],
				"temperature_2m_max": [25.0, 26.0],
				"temperature_2m_min": [15.0, 16.0],
				"precipitation_sum": [0.5, 0.0],
				"weather_code": [3, 0]
			}
		}`))
	}))
	defer srv.Close()

	tool := NewWeatherTool().WithBaseURLs(srv.URL, "")

	args, _ := json.Marshal(map[string]any{"latitude": 35.6762, "longitude": 139.6503})
	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute errored: %v", err)
	}

	// The forecast URL must include 7 days, both daily and current
	// blocks, and timezone=auto — these are the things the AI's user
	// expects to see in the answer.
	for _, want := range []string{"latitude=35.6762", "longitude=139.6503", "forecast_days=7", "timezone=auto", "current=temperature_2m"} {
		if !strings.Contains(capturedQuery, want) {
			t.Errorf("upstream URL missing %q; got %q", want, capturedQuery)
		}
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if _, ok := got["current"]; !ok {
		t.Error("output missing 'current' block")
	}
	if _, ok := got["forecast_7day"]; !ok {
		t.Error("output missing 'forecast_7day' block")
	}
	if _, ok := got["location"]; !ok {
		t.Error("output missing 'location' block")
	}
	// The condition string is what the AI surfaces — pin that the
	// translation from numeric weather code to text actually happens.
	current := got["current"].(map[string]any)
	if current["condition"] != "Partly cloudy" {
		t.Errorf("condition for code 2 = %q, want 'Partly cloudy'", current["condition"])
	}
}

// TestWeatherTool_CityGeocoding_FollowsTwoHopFlow exercises the path
// where the user provides a city name instead of coordinates. The tool
// MUST call the geocoding endpoint first, then the forecast endpoint
// with the resolved coordinates. A future refactor that breaks the
// hand-off would silently turn city-only requests into "no_location"
// errors, so this test is the contract.
func TestWeatherTool_CityGeocoding_FollowsTwoHopFlow(t *testing.T) {
	geoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Open-Meteo geocoding URL-encodes the city name.
		if !strings.Contains(r.URL.RawQuery, "name=Tokyo") {
			t.Errorf("geocode missing name=Tokyo, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"results":[{"latitude":35.6762,"longitude":139.6503}]}`))
	}))
	defer geoSrv.Close()

	var forecastCalled bool
	weatherSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forecastCalled = true
		// The geocoded coordinates must flow into the forecast call.
		if !strings.Contains(r.URL.RawQuery, "latitude=35.6762") {
			t.Errorf("forecast didn't get geocoded coords, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"timezone":"Asia/Tokyo","current":{"temperature_2m":22},"daily":{"time":[]}}`))
	}))
	defer weatherSrv.Close()

	tool := NewWeatherTool().WithBaseURLs(weatherSrv.URL, geoSrv.URL)

	args, _ := json.Marshal(map[string]any{"city": "Tokyo"})
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("Execute errored: %v", err)
	}
	if !forecastCalled {
		t.Error("forecast endpoint was not called after geocoding succeeded")
	}
}

// TestWeatherTool_CityGeocoding_NoResults_ReturnsStructuredError
// covers the case where the user gives a city the geocoder can't find.
// Tool MUST return a structured-error JSON (not a Go error) so the AI
// can ask the user to clarify rather than the chat handler treating
// the call as fatal.
func TestWeatherTool_CityGeocoding_NoResults_ReturnsStructuredError(t *testing.T) {
	geoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer geoSrv.Close()

	tool := NewWeatherTool().WithBaseURLs("", geoSrv.URL)

	args, _ := json.Marshal(map[string]any{"city": "Atlantis"})
	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute should not return Go error on geocode miss: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if got["error"] != "geocode_failed" {
		t.Errorf("error = %q, want geocode_failed", got["error"])
	}
}

// TestWeatherTool_NoLocation_ReturnsStructuredError covers the case
// where the AI calls the tool with neither lat/lng nor city. The
// schema doesn't require either, so this is a runtime check.
func TestWeatherTool_NoLocation_ReturnsStructuredError(t *testing.T) {
	tool := NewWeatherTool()

	args, _ := json.Marshal(map[string]any{})
	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute should not error on missing location: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if got["error"] != "no_location" {
		t.Errorf("error = %q, want no_location", got["error"])
	}
}

// TestWeatherTool_UpstreamHTTPError_ReturnsStructuredError pins the
// non-200 path: forecast endpoint returns 5xx, tool returns a
// structured error that lets the AI say "weather is temporarily
// unavailable" rather than crashing the chat turn.
func TestWeatherTool_UpstreamHTTPError_ReturnsStructuredError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("upstream down"))
	}))
	defer srv.Close()

	tool := NewWeatherTool().WithBaseURLs(srv.URL, "")
	args, _ := json.Marshal(map[string]any{"latitude": 35.0, "longitude": 139.0})

	out, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute should not return Go error on 503: %v", err)
	}
	var got map[string]string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if got["error"] != "weather_api_error" {
		t.Errorf("error = %q, want weather_api_error", got["error"])
	}
}

// TestWeatherCodeToText_BoundaryConditions pins the WMO code → text
// mapping at every boundary so a future refactor (e.g. someone tweaks
// "<= 67" to "<= 65" thinking it doesn't matter) breaks loudly.
// These mappings show up verbatim in user-facing chat responses.
func TestWeatherCodeToText_BoundaryConditions(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{0, "Clear sky"},
		{1, "Partly cloudy"},
		{3, "Partly cloudy"},
		{4, "Foggy"},
		{48, "Foggy"},
		{49, "Drizzle"},
		{57, "Drizzle"},
		{58, "Rain"},
		{67, "Rain"},
		{68, "Snow"},
		{77, "Snow"},
		{78, "Rain showers"},
		{82, "Rain showers"},
		{83, "Snow showers"},
		{86, "Snow showers"},
		{87, "Thunderstorm"},
		{99, "Thunderstorm"},
		{100, "Unknown"},
		{-1, "Unknown"},
	}
	for _, c := range cases {
		if got := weatherCodeToText(c.code); got != c.want {
			t.Errorf("weatherCodeToText(%d) = %q, want %q", c.code, got, c.want)
		}
	}
}

// TestWeatherTool_MalformedArgs_ReturnsGoError pins the contract
// boundary: malformed JSON args return a Go error (programming bug),
// not a structured error (user-recoverable).
func TestWeatherTool_MalformedArgs_ReturnsGoError(t *testing.T) {
	tool := NewWeatherTool()
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{not valid`)); err == nil {
		t.Error("expected Go error on malformed args")
	}
}
