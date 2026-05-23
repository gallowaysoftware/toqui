package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// meterName is the instrumentation scope for all Toqui backend metrics.
const meterName = "toqui-backend"

// Metrics holds all OpenTelemetry metric instruments for the Toqui backend.
// Instruments are safe for concurrent use. When the OTEL exporter is not
// configured, the global MeterProvider returns no-op meters — all Record/Add
// calls become free.
type Metrics struct {
	// RequestDuration records HTTP request latency in seconds.
	// Attributes: http.request.method, http.route, http.response.status_code
	RequestDuration metric.Float64Histogram

	// RequestCount counts total HTTP requests.
	// Attributes: http.request.method, http.route, http.response.status_code
	RequestCount metric.Int64Counter

	// ErrorCount counts errors by type.
	// Attributes: error.type
	ErrorCount metric.Int64Counter

	// AITokenUsage counts AI tokens consumed.
	// Attributes: ai.provider, ai.model_tier, deployment.environment
	AITokenUsage metric.Int64Counter

	// RateLimitHits counts rate limit rejections.
	// Attributes: ratelimit.type (user, ip)
	RateLimitHits metric.Int64Counter

	// ActiveConnections tracks active SSE streaming connections.
	// Attributes: (none)
	ActiveConnections metric.Int64UpDownCounter

	// AICostCents records estimated AI cost in cents per request.
	// Attributes: ai.provider, ai.model_tier, user.tier
	AICostCents metric.Int64Counter
}

// NewMetrics creates and registers all metric instruments. It uses the global
// MeterProvider set by Init. If Init was not called or OTEL is disabled, the
// global provider returns no-op instruments that are safe but do nothing.
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter(meterName)

	requestDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP server requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	requestCount, err := meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Total number of HTTP server requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	errorCount, err := meter.Int64Counter(
		"errors.count",
		metric.WithDescription("Total number of errors by type"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	aiTokenUsage, err := meter.Int64Counter(
		"ai.token.usage",
		metric.WithDescription("Total AI tokens consumed"),
		metric.WithUnit("{token}"),
	)
	if err != nil {
		return nil, err
	}

	rateLimitHits, err := meter.Int64Counter(
		"ratelimit.hits",
		metric.WithDescription("Total rate limit rejections"),
		metric.WithUnit("{rejection}"),
	)
	if err != nil {
		return nil, err
	}

	activeConnections, err := meter.Int64UpDownCounter(
		"connections.active",
		metric.WithDescription("Number of active SSE streaming connections"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return nil, err
	}

	aiCostCents, err := meter.Int64Counter(
		"ai.cost.cents",
		metric.WithDescription("Estimated AI cost in cents per request"),
		metric.WithUnit("{cent}"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		RequestDuration:   requestDuration,
		RequestCount:      requestCount,
		ErrorCount:        errorCount,
		AITokenUsage:      aiTokenUsage,
		RateLimitHits:     rateLimitHits,
		ActiveConnections: activeConnections,
		AICostCents:       aiCostCents,
	}, nil
}
