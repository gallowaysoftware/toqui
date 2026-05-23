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
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/gallowaysoftware/toqui/backend/internal/config"
)

// resolveTraceSampler returns the OTel sampler to use, derived from the
// OTEL_TRACES_SAMPLER_ARG environment variable.
//
// Why a sampler at all: the SDK's default tracer provider samples 100% of
// traces (AlwaysSample). At any meaningful traffic volume this saturates
// the OTel collector cardinality budget and inflates spend on whatever
// downstream backend is collecting (Grafana Cloud, Honeycomb, etc).
// 10% head-based sampling is the industry standard starting point — it
// preserves enough fidelity for debugging while making the cost story
// linear with traffic instead of explosive.
//
// Why ParentBased over plain TraceIDRatioBased: parent-based ensures
// that if an upstream service (e.g. a load balancer or a frontend) has
// already made a sampling decision and propagated it via traceparent,
// we honour it. Without ParentBased, a 10% local sampler would drop 90%
// of the traces an upstream tool decided to keep — fragmenting traces
// in collateral.
//
// Why env-configurable: the cardinality budget differs by environment
// (dev wants 100%, staging wants 100%, prod wants 1-10% depending on
// traffic). Using OTEL_TRACES_SAMPLER_ARG (the OTel-standard env var
// for sampler ratios) keeps this aligned with the rest of the OTLP
// configuration which is also env-driven.
//
// Values:
//   - empty / unparseable → ParentBased(AlwaysSample) — preserves the
//     pre-this-change behaviour so a deploy without setting the env var
//     doesn't suddenly drop 90% of traces.
//   - "0.0" through "1.0" → ParentBased(TraceIDRatioBased(rate)).
//   - "0" → effectively NeverSample (still ParentBased so explicit
//     traceparent headers from clients are honoured).
//
// Out-of-range values (negative, > 1) clamp to the nearest valid value
// rather than erroring — bad env config shouldn't crash the boot path.
func resolveTraceSampler() (sdktrace.Sampler, string) {
	raw := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG"))
	if raw == "" {
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), "always"
	}
	rate, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		slog.Warn("OTEL_TRACES_SAMPLER_ARG is not a valid float, falling back to AlwaysSample",
			"value", raw, "error", err)
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), "always"
	}
	switch {
	case rate <= 0:
		return sdktrace.ParentBased(sdktrace.NeverSample()), "never"
	case rate >= 1:
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), "always"
	default:
		return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(rate)), strconv.FormatFloat(rate, 'f', -1, 64)
	}
}

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

	sampler, samplerLabel := resolveTraceSampler()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
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
		"trace_sampler", samplerLabel,
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
