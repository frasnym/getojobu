package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
)

// setupTestTelemetry installs fresh test tracer and meter providers as the
// otel globals and restores no-op providers on cleanup. It returns the span
// recorder and metric manual reader for assertions.
func setupTestTelemetry(t *testing.T) (*tracetest.SpanRecorder, *sdkmetric.ManualReader) {
	t.Helper()

	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })

	// Production telemetry.Init installs the W3C TraceContext propagator when
	// enabled; tests mirror that so traceparent extraction works.
	otel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() { otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator()) })

	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(sdkmetric.NewMeterProvider()) })

	return rec, reader
}

// tracedServeMux returns a mux with one pattern route and the request wrapped
// for serving, so ServeMux sets r.Pattern during dispatch.
func tracedServeMux() (*http.ServeMux, *http.Request) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/jobs/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/42", nil)
	return mux, req
}

func TestTracing_span_named_by_route_pattern_not_raw_path(t *testing.T) {
	// Arrange.
	rec, _ := setupTestTelemetry(t)
	mux, req := tracedServeMux()

	// Act.
	Tracing(mux).ServeHTTP(httptest.NewRecorder(), req)

	// Assert.
	spans := rec.Ended()
	require.Len(t, spans, 1, "expected exactly one server span")
	assert.Equal(t, "GET /api/jobs/{id}", spans[0].Name(), "span must be named by route pattern, not raw path")
}

func TestTracing_continues_incoming_traceparent(t *testing.T) {
	// Arrange.
	rec, _ := setupTestTelemetry(t)
	const wantTraceID = "11111111111111111111111111111111"
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Header.Set("Traceparent", "00-"+wantTraceID+"-2222222222222222-01")

	// Act.
	Tracing(http.NotFoundHandler()).ServeHTTP(httptest.NewRecorder(), req)

	// Assert.
	spans := rec.Ended()
	require.Len(t, spans, 1, "expected exactly one server span")
	assert.Equal(t, wantTraceID, spans[0].SpanContext().TraceID().String(), "server span must continue the caller's trace")
}

func TestTracing_request_id_equals_trace_id_when_no_header(t *testing.T) {
	// Arrange.
	rec, _ := setupTestTelemetry(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)

	// Act: Tracing wraps RequestID so the span exists when RequestID runs.
	w := httptest.NewRecorder()
	Tracing(RequestID(http.NotFoundHandler())).ServeHTTP(w, req)

	// Assert: the response request id equals the server span's trace id.
	spans := rec.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, spans[0].SpanContext().TraceID().String(), w.Header().Get("X-Request-Id"), "request id must equal the trace id")
}

func TestTracing_caller_request_id_takes_precedence_over_trace_id(t *testing.T) {
	// Arrange.
	_, _ = setupTestTelemetry(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Header.Set("X-Request-Id", "req-abc123")
	req.Header.Set("Traceparent", "00-11111111111111111111111111111111-2222222222222222-01")

	// Act.
	w := httptest.NewRecorder()
	Tracing(RequestID(http.NotFoundHandler())).ServeHTTP(w, req)

	// Assert: echoed id wins over the trace id.
	assert.Equal(t, "req-abc123", w.Header().Get("X-Request-Id"))
}

func TestTracing_records_duration_histogram_with_route_attribute(t *testing.T) {
	// Arrange.
	_, reader := setupTestTelemetry(t)
	mux, req := tracedServeMux()

	// Act.
	Tracing(mux).ServeHTTP(httptest.NewRecorder(), req)

	// Assert.
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	found := false
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "getojobu_http_request_duration_ms" {
				continue
			}
			found = true
			hist, ok := m.Data.(metricdata.Histogram[int64])
			require.True(t, ok, "expected an int64 histogram")
			require.Len(t, hist.DataPoints, 1, "expected one datapoint for one request")
			dp := hist.DataPoints[0]
			assert.Equal(t, "GET /api/jobs/{id}", attrValue(dp.Attributes, "http.route"))
			assert.Equal(t, "GET", attrValue(dp.Attributes, "http.request.method"))
		}
	}
	require.True(t, found, "expected %s metric to be recorded", "getojobu_http_request_duration_ms")
}

func attrValue(set attribute.Set, key string) string {
	v, ok := set.Value(attribute.Key(key))
	if !ok {
		return ""
	}
	return v.AsString()
}
