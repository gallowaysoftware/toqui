package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
)

// WeatherTool is a chat tool that provides weather data for a location.
// Uses the free Open-Meteo API (no API key required).
type WeatherTool struct {
	client *http.Client
}

type weatherArgs struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city"`
}

func NewWeatherTool() *WeatherTool {
	return &WeatherTool{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *WeatherTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "get_weather",
		Description: "Get current weather and 7-day forecast for a location. Use when the user asks about weather, climate, or what to pack for a destination. Provide either latitude/longitude or a city name.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"latitude": {
					"type": "number",
					"description": "Latitude of the location (e.g., 35.6762 for Tokyo)"
				},
				"longitude": {
					"type": "number",
					"description": "Longitude of the location (e.g., 139.6503 for Tokyo)"
				},
				"city": {
					"type": "string",
					"description": "City name for geocoding (e.g., 'Tokyo, Japan'). Used if lat/lng not provided."
				}
			}
		}`),
	}
}

func (t *WeatherTool) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var params weatherArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("parse args: %w", err)
	}

	lat, lng := params.Latitude, params.Longitude

	// If no coordinates, geocode the city name via Open-Meteo's geocoding API.
	if lat == 0 && lng == 0 && params.City != "" {
		var err error
		lat, lng, err = t.geocodeCity(ctx, params.City)
		if err != nil {
			return json.Marshal(map[string]string{
				"error":   "geocode_failed",
				"message": fmt.Sprintf("Could not find coordinates for '%s': %v", params.City, err),
			})
		}
	}

	if lat == 0 && lng == 0 {
		return json.Marshal(map[string]string{
			"error":   "no_location",
			"message": "Provide either latitude/longitude or a city name.",
		})
	}

	// Fetch weather from Open-Meteo (free, no API key).
	u := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,relative_humidity_2m,apparent_temperature,precipitation,weather_code,wind_speed_10m&daily=temperature_2m_max,temperature_2m_min,precipitation_sum,weather_code&timezone=auto&forecast_days=7",
		lat, lng,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("create weather request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return json.Marshal(map[string]string{
			"error":   "weather_unavailable",
			"message": fmt.Sprintf("Weather API error: %v", err),
		})
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("read weather response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Warn("weather API error", "status", resp.StatusCode, "body", string(body))
		return json.Marshal(map[string]string{
			"error":   "weather_api_error",
			"message": fmt.Sprintf("Weather API returned status %d", resp.StatusCode),
		})
	}

	// Parse and simplify the response for the AI.
	var weatherResp openMeteoResponse
	if err := json.Unmarshal(body, &weatherResp); err != nil {
		return nil, fmt.Errorf("parse weather response: %w", err)
	}

	result := map[string]any{
		"location": map[string]any{
			"latitude":  lat,
			"longitude": lng,
			"timezone":  weatherResp.Timezone,
		},
		"current": map[string]any{
			"temperature_c":    weatherResp.Current.Temperature,
			"feels_like_c":     weatherResp.Current.ApparentTemp,
			"humidity_percent": weatherResp.Current.Humidity,
			"precipitation_mm": weatherResp.Current.Precipitation,
			"wind_speed_kmh":   weatherResp.Current.WindSpeed,
			"condition":        weatherCodeToText(weatherResp.Current.WeatherCode),
		},
	}

	// Add 7-day forecast.
	if len(weatherResp.Daily.Time) > 0 {
		forecast := make([]map[string]any, 0, len(weatherResp.Daily.Time))
		for i, date := range weatherResp.Daily.Time {
			day := map[string]any{"date": date}
			if i < len(weatherResp.Daily.TempMax) {
				day["high_c"] = weatherResp.Daily.TempMax[i]
			}
			if i < len(weatherResp.Daily.TempMin) {
				day["low_c"] = weatherResp.Daily.TempMin[i]
			}
			if i < len(weatherResp.Daily.PrecipSum) {
				day["precipitation_mm"] = weatherResp.Daily.PrecipSum[i]
			}
			if i < len(weatherResp.Daily.WeatherCode) {
				day["condition"] = weatherCodeToText(weatherResp.Daily.WeatherCode[i])
			}
			forecast = append(forecast, day)
		}
		result["forecast_7day"] = forecast
	}

	return json.Marshal(result)
}

func (t *WeatherTool) geocodeCity(ctx context.Context, city string) (float64, float64, error) {
	u := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=en", url.QueryEscape(city))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, 0, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024))
	if err != nil {
		return 0, 0, err
	}

	var geoResp struct {
		Results []struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &geoResp); err != nil {
		return 0, 0, err
	}
	if len(geoResp.Results) == 0 {
		return 0, 0, fmt.Errorf("no results found")
	}
	return geoResp.Results[0].Latitude, geoResp.Results[0].Longitude, nil
}

type openMeteoResponse struct {
	Timezone string `json:"timezone"`
	Current  struct {
		Temperature   float64 `json:"temperature_2m"`
		Humidity      float64 `json:"relative_humidity_2m"`
		ApparentTemp  float64 `json:"apparent_temperature"`
		Precipitation float64 `json:"precipitation"`
		WeatherCode   int     `json:"weather_code"`
		WindSpeed     float64 `json:"wind_speed_10m"`
	} `json:"current"`
	Daily struct {
		Time        []string  `json:"time"`
		TempMax     []float64 `json:"temperature_2m_max"`
		TempMin     []float64 `json:"temperature_2m_min"`
		PrecipSum   []float64 `json:"precipitation_sum"`
		WeatherCode []int     `json:"weather_code"`
	} `json:"daily"`
}

// weatherCodeToText converts WMO weather codes to human-readable text.
func weatherCodeToText(code int) string {
	switch {
	case code == 0:
		return "Clear sky"
	case code <= 3:
		return "Partly cloudy"
	case code <= 48:
		return "Foggy"
	case code <= 57:
		return "Drizzle"
	case code <= 67:
		return "Rain"
	case code <= 77:
		return "Snow"
	case code <= 82:
		return "Rain showers"
	case code <= 86:
		return "Snow showers"
	case code <= 99:
		return "Thunderstorm"
	default:
		return "Unknown"
	}
}
