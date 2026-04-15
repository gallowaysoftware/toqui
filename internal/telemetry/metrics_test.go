package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestNewMetrics(t *testing.T) {
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("NewMetrics() error: %v", err)
	}
	if m.RequestDuration == nil {
		t.Error("RequestDuration is nil")
	}
	if m.RequestCount == nil {
		t.Error("RequestCount is nil")
	}
	if m.ErrorCount == nil {
		t.Error("ErrorCount is nil")
	}
	if m.AITokenUsage == nil {
		t.Error("AITokenUsage is nil")
	}
	if m.RateLimitHits == nil {
		t.Error("RateLimitHits is nil")
	}
	if m.ActiveConnections == nil {
		t.Error("ActiveConnections is nil")
	}
	if m.AICostCents == nil {
		t.Error("AICostCents is nil")
	}
}

func TestMiddleware_RecordsMetrics(t *testing.T) {
	// Use a test MeterProvider with a ManualReader to inspect recorded metrics.
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { provider.Shutdown(context.Background()) })

	// Create metrics using the test provider's meter.
	meter := provider.Meter(meterName)
	reqDuration, _ := meter.Float64Histogram("http.server.request.duration")
	reqCount, _ := meter.Int64Counter("http.server.request.count")

	m := &Metrics{
		RequestDuration: reqDuration,
		RequestCount:    reqCount,
	}

	// Create a test handler that returns 200.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	handler := Middleware(m, inner)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Collect metrics and verify they were recorded.
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if len(rm.ScopeMetrics) == 0 {
		t.Fatal("expected at least one ScopeMetrics entry")
	}

	foundCounter := false
	foundHistogram := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch m.Name {
			case "http.server.request.count":
				foundCounter = true
				sum, ok := m.Data.(metricdata.Sum[int64])
				if !ok {
					t.Fatalf("expected Sum[int64], got %T", m.Data)
				}
				if len(sum.DataPoints) == 0 {
					t.Fatal("no data points for request count")
				}
				if sum.DataPoints[0].Value != 1 {
					t.Errorf("expected request count 1, got %d", sum.DataPoints[0].Value)
				}
				// Verify attributes.
				attrs := sum.DataPoints[0].Attributes
				if v, ok := attrValue(attrs, "http.request.method"); !ok || v != "GET" {
					t.Errorf("expected http.request.method=GET, got %q", v)
				}
				if v, ok := attrValue(attrs, "http.route"); !ok || v != "/healthz" {
					t.Errorf("expected http.route=/healthz, got %q", v)
				}
				if v, ok := attrValue(attrs, "http.response.status_code"); !ok || v != "200" {
					t.Errorf("expected http.response.status_code=200, got %q", v)
				}
			case "http.server.request.duration":
				foundHistogram = true
				hist, ok := m.Data.(metricdata.Histogram[float64])
				if !ok {
					t.Fatalf("expected Histogram[float64], got %T", m.Data)
				}
				if len(hist.DataPoints) == 0 {
					t.Fatal("no data points for request duration")
				}
				if hist.DataPoints[0].Count != 1 {
					t.Errorf("expected histogram count 1, got %d", hist.DataPoints[0].Count)
				}
			}
		}
	}

	if !foundCounter {
		t.Error("http.server.request.count metric not found")
	}
	if !foundHistogram {
		t.Error("http.server.request.duration metric not found")
	}
}

func TestMiddleware_RecordsNon200Status(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { provider.Shutdown(context.Background()) })

	meter := provider.Meter(meterName)
	reqDuration, _ := meter.Float64Histogram("http.server.request.duration")
	reqCount, _ := meter.Int64Counter("http.server.request.count")

	m := &Metrics{
		RequestDuration: reqDuration,
		RequestCount:    reqCount,
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := Middleware(m, inner)

	req := httptest.NewRequest(http.MethodPost, "/api/missing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "http.server.request.count" {
				sum := m.Data.(metricdata.Sum[int64])
				attrs := sum.DataPoints[0].Attributes
				if v, ok := attrValue(attrs, "http.response.status_code"); !ok || v != "404" {
					t.Errorf("expected status_code=404, got %q", v)
				}
				if v, ok := attrValue(attrs, "http.request.method"); !ok || v != "POST" {
					t.Errorf("expected method=POST, got %q", v)
				}
			}
		}
	}
}

func TestMiddleware_NilMetrics(t *testing.T) {
	// When metrics is nil, the middleware should pass through without panicking.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := Middleware(nil, inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

// attrValue extracts a string attribute value from an attribute set.
func attrValue(attrs attribute.Set, key string) (string, bool) {
	v, ok := attrs.Value(attribute.Key(key))
	if !ok {
		return "", false
	}
	return v.Emit(), true
}
