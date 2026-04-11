// Package telemetry provides OpenTelemetry initialization for the Toqui backend.
// It configures OTLP HTTP trace and metric exporters using standard OTEL_EXPORTER_OTLP_*
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
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/gallowaysoftware/toqui-backend/internal/config"
)

// Init initializes OpenTelemetry tracing and metrics with OTLP HTTP exporters.
//
// It resolves any gcsm:// prefix in OTEL_EXPORTER_OTLP_HEADERS using GCP
// Secret Manager before the OTEL SDK reads it. The projectID is used for
// short-form gcsm:// expansion.
//
// The returned shutdown function must be called on server exit to flush
// pending spans and metrics. Returns a no-op shutdown if OTEL_EXPORTER_OTLP_ENDPOINT
// is not set.
func Init(ctx context.Context, serviceName, projectID string) (shutdown func(context.Context) error, err error) {
	noop := func(context.Context) error { return nil }

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		slog.Info("OTEL_EXPORTER_OTLP_ENDPOINT not set, telemetry disabled")
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

	// Build resource with service name. We avoid resource.Merge with
	// resource.Default() because the SDK's default resource uses a newer
	// schema URL than semconv/v1.26.0, causing a conflict error.
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
	)

	// --- Traces ---

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return noop, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// --- Metrics ---

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		// Non-fatal: traces still work without metrics.
		slog.Error("failed to create OTLP metric exporter, metrics disabled", "error", err)
	}

	var mp *sdkmetric.MeterProvider
	if metricExporter != nil {
		mp = sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
	}

	slog.Info("OpenTelemetry initialized",
		"endpoint", endpoint,
		"service", serviceName,
		"traces", true,
		"metrics", mp != nil,
	)

	// Combined shutdown flushes both traces and metrics.
	return func(ctx context.Context) error {
		var firstErr error
		if mp != nil {
			if err := mp.Shutdown(ctx); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("meter provider shutdown: %w", err)
			}
		}
		if err := tp.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("tracer provider shutdown: %w", err)
		}
		return firstErr
	}, nil
}
