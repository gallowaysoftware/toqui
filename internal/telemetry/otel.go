// Package telemetry provides OpenTelemetry initialization for the Toqui backend.
// It configures an OTLP HTTP trace exporter using standard OTEL_EXPORTER_OTLP_*
// environment variables, with support for resolving gcsm:// secret references
// in OTEL_EXPORTER_OTLP_HEADERS before the SDK reads them.
package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/gallowaysoftware/toqui-backend/internal/config"
)

// Init initializes OpenTelemetry tracing with an OTLP HTTP exporter.
//
// It resolves any gcsm:// prefix in OTEL_EXPORTER_OTLP_HEADERS using GCP
// Secret Manager before the OTEL SDK reads it. The projectID is used for
// short-form gcsm:// expansion.
//
// The returned shutdown function must be called on server exit to flush
// pending spans. Returns a no-op shutdown if OTEL_EXPORTER_OTLP_ENDPOINT
// is not set.
func Init(ctx context.Context, serviceName, projectID string) (shutdown func(context.Context) error, err error) {
	noop := func(context.Context) error { return nil }

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		slog.Info("OTEL_EXPORTER_OTLP_ENDPOINT not set, tracing disabled")
		return noop, nil
	}

	// Resolve gcsm:// in OTEL_EXPORTER_OTLP_HEADERS before OTEL SDK reads it.
	if headers := os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"); strings.HasPrefix(headers, "gcsm://") {
		resolved, err := config.ResolveSecretValue(headers, projectID)
		if err != nil {
			return noop, fmt.Errorf("resolve OTEL_EXPORTER_OTLP_HEADERS secret: %w", err)
		}
		os.Setenv("OTEL_EXPORTER_OTLP_HEADERS", resolved)
		slog.Info("resolved OTEL_EXPORTER_OTLP_HEADERS from GCP Secret Manager")
	}

	// Create OTLP HTTP exporter. It reads OTEL_EXPORTER_OTLP_* env vars
	// automatically (endpoint, protocol, headers, etc.).
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return noop, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	// Build resource with service name. We avoid resource.Merge with
	// resource.Default() because the SDK's default resource uses a newer
	// schema URL than semconv/v1.26.0, causing a conflict error.
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
	)

	// Create TracerProvider with batch span processor.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Register as global tracer provider and set W3C propagators.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	slog.Info("OpenTelemetry tracing initialized",
		"endpoint", endpoint,
		"service", serviceName,
	)

	return tp.Shutdown, nil
}
