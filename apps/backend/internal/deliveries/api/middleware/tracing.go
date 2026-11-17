package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
)

// meterName identifies the API's instrumentation scope in the trace backend.
const meterName = "getojobu-api"

// histogramName is the per-route request duration histogram. It is recorded
// now and becomes visible when a metrics backend is added (phase 2).
const histogramName = "getojobu_http_request_duration_ms"

// histograms caches one histogram per meter provider (the otel global changes
// between disabled and enabled states and in tests; each provider gets its own
// instrument).
var histograms sync.Map

// durationHistogram returns the request duration histogram for the current
// global meter provider, creating it on first use for that provider.
func durationHistogram() api.Int64Histogram {
	key := otel.GetMeterProvider()
	if v, ok := histograms.Load(key); ok {
		return v.(api.Int64Histogram)
	}
	h, err := key.Meter(meterName).Int64Histogram(histogramName, api.WithUnit("ms"))
	if err != nil {
		// Instrument creation only fails on invalid name/unit; fall back to a
		// no-op instrument so requests are never blocked.
		h = noopHistogram{}
	}
	actual, _ := histograms.LoadOrStore(key, h)
	return actual.(api.Int64Histogram)
}

// noopHistogram satisfies api.Int64Histogram while recording nothing.
type noopHistogram struct {
	embedded.Int64Histogram
}

func (noopHistogram) Record(context.Context, int64, ...api.RecordOption) {}
func (noopHistogram) Add(context.Context, int64, ...api.AddOption)       {}
func (noopHistogram) Enabled(context.Context) bool                       { return false }

// spanName returns the provisional span name before the mux has matched.
func spanName(r *http.Request) string {
	return r.Method + " " + r.URL.Path
}

// routePattern returns the matched route pattern after dispatch, or "".
func routePattern(r *http.Request) string {
	return r.Pattern
}

// Tracing starts a server span per request, continues an incoming traceparent
// trace when present, and records the request duration in the per-route
// histogram. Span names use the route pattern (e.g. GET /api/jobs/{id}), never
// the raw path with IDs.
func Tracing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagationHeaderCarrier{r.Header})
		tracer := otel.GetTracerProvider().Tracer(meterName)
		ctx, span := tracer.Start(ctx, spanName(r), trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		// served is the request the mux dispatches; ServeMux sets its Pattern
		// field during match, so read the pattern from it, not from r.
		served := r.WithContext(ctx)
		next.ServeHTTP(rec, served)
		pattern := routePattern(served)

		// Rename the span once the mux has matched a route pattern. The
		// method-aware mux pattern already includes the method
		// ("GET /api/jobs/{id}"); prepend only when it does not.
		name := pattern
		if !strings.HasPrefix(name, r.Method+" ") {
			name = r.Method + " " + name
		}
		span.SetName(name)
		if pattern != "" {
			span.SetAttributes(semconv.HTTPRoute(pattern))
		} else {
			span.SetAttributes(semconv.HTTPRoute(r.URL.Path))
		}

		span.SetAttributes(
			semconv.HTTPRequestMethodKey.String(r.Method),
			semconv.HTTPResponseStatusCode(rec.status),
		)
		if rec.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, "")
			span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(rec.status)))
		}

		route := pattern
		if route == "" {
			route = r.URL.Path
		}
		durationHistogram().Record(ctx, time.Since(start).Milliseconds(),
			api.WithAttributes(
				semconv.HTTPRoute(route),
				semconv.HTTPRequestMethodKey.String(r.Method),
				attribute.Int("http.response.status_code", rec.status),
			),
		)
	})
}

// propagationHeaderCarrier adapts http.Header to the otel text map carrier.
type propagationHeaderCarrier struct {
	header http.Header
}

func (c propagationHeaderCarrier) Get(key string) string {
	return c.header.Get(key)
}

func (c propagationHeaderCarrier) Set(key, value string) {
	c.header.Set(key, value)
}

func (c propagationHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.header))
	for k := range c.header {
		keys = append(keys, k)
	}
	return keys
}
