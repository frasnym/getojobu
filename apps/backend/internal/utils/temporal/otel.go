// Package temporal bridges OpenTelemetry to the Temporal SDK's interceptor.Tracer
// so Temporal client calls (StartWorkflow, SignalWithStart, ...) become child
// spans of the request that started them, with W3C traceparent propagated
// through Temporal headers for the Python worker to continue.
//
// The contrib module (go.temporal.io/sdk/contrib/opentelemetry) was not
// fetchable in this environment, so this is a self-contained OTel Tracer
// implementation. It implements go.temporal.io/sdk/interceptor.Tracer and is
// wrapped with interceptor.NewTracingInterceptor.
package temporal

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/log"
)

// spanContextKey is the context key used to stash a TracerSpan under
// SpanContextKey. It is unexported so unrelated WithValue calls cannot collide.
type spanContextKey struct{}

// headerKey is the Temporal header carrying the W3C traceparent. The Python
// worker's temporalio.contrib.opentelemetry interceptor uses the same key.
const (
	headerKey  = "temporal-trace-context"
	tracerName = "getojobu-temporal"
)

// otelTracer implements interceptor.Tracer on top of the global OpenTelemetry
// providers installed by telemetry.Init (or the no-op defaults when disabled).
type otelTracer struct {
	interceptor.BaseTracer
}

// Tracer returns an interceptor.Tracer backed by the global OpenTelemetry
// providers (no-op when telemetry is disabled). Wrap with
// interceptor.NewTracingInterceptor and register on client and worker options.
func Tracer() interceptor.Tracer {
	return otelTracer{}
}

// Options satisfies interceptor.Tracer.
func (t otelTracer) Options() interceptor.TracerOptions {
	return interceptor.TracerOptions{
		SpanContextKey: spanContextKey{},
		HeaderKey:      headerKey,
		// Headers stay plain W3C traceparent; the Python worker's
		// temporalio.contrib.opentelemetry interceptor reads the same format.
		EncodeHeaders: false,
	}
}

// UnmarshalSpan parses a W3C traceparent header map back into a span reference.
func (t otelTracer) UnmarshalSpan(headers map[string]string) (interceptor.TracerSpanRef, error) {
	tp, ok := headers[headerKey]
	if !ok || tp == "" {
		return nil, nil
	}
	sc, err := parseTraceparent(tp)
	if err != nil {
		return nil, err
	}
	return &otelSpanRef{sc: sc}, nil
}

// MarshalSpan writes the current span as a W3C traceparent into the header map.
func (t otelTracer) MarshalSpan(span interceptor.TracerSpan) (map[string]string, error) {
	s, ok := span.(*otelSpanImpl)
	if !ok || s == nil || !s.sc.IsValid() {
		return map[string]string{}, nil
	}
	return map[string]string{headerKey: formatTraceparent(s.sc)}, nil
}

// SpanFromContext returns the span stored on ctx, or nil.
func (t otelTracer) SpanFromContext(ctx context.Context) interceptor.TracerSpan {
	if s, ok := ctx.Value(spanContextKey{}).(*otelSpanImpl); ok {
		return s
	}
	return nil
}

// ContextWithSpan stores span on ctx.
func (t otelTracer) ContextWithSpan(ctx context.Context, span interceptor.TracerSpan) context.Context {
	s, _ := span.(*otelSpanImpl)
	return context.WithValue(ctx, spanContextKey{}, s)
}

// StartSpan starts an OTel child span of the parent (if any) and returns it.
func (t otelTracer) StartSpan(options *interceptor.TracerStartSpanOptions) (interceptor.TracerSpan, error) {
	ctx := context.Background()

	var opts []trace.SpanStartOption
	if options.Parent != nil {
		if ref, ok := options.Parent.(*otelSpanRef); ok && ref.sc.IsValid() {
			// Attach the remote parent so the new span links to it. The SDK
			// passes header-derived parents here, which are remote contexts.
			ctx = trace.ContextWithRemoteSpanContext(ctx, ref.sc)
		}
	}
	opts = append(opts, trace.WithSpanKind(trace.SpanKindClient))

	attrs := make([]attribute.KeyValue, 0, len(options.Tags))
	for k, v := range options.Tags {
		attrs = append(attrs, attribute.String(k, v))
	}
	if len(attrs) > 0 {
		opts = append(opts, trace.WithAttributes(attrs...))
	}

	_, otelSpan := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, options.Name, opts...)
	return &otelSpanImpl{span: otelSpan, sc: otelSpan.SpanContext()}, nil
}

// GetLogger satisfies the Tracer interface via BaseTracer embedding.
var _ = otelTracer{}.GetLogger

// otelSpanRef carries a span context as a TracerSpanRef.
type otelSpanRef struct {
	sc trace.SpanContext
}

// otelSpanImpl is a TracerSpan backed by an OTel span.
type otelSpanImpl struct {
	span trace.Span
	sc   trace.SpanContext
}

var _ interceptor.TracerSpan = (*otelSpanImpl)(nil)

// Finish ends the span, recording an error status when one is provided.
func (s *otelSpanImpl) Finish(options *interceptor.TracerFinishSpanOptions) {
	if s == nil || s.span == nil {
		return
	}
	if options != nil && options.Error != nil {
		s.span.SetStatus(codes.Error, options.Error.Error())
		s.span.SetAttributes(attribute.Bool("error", true))
	}
	s.span.End()
}

// formatTraceparent renders a span context as W3C traceparent:
// 00-{32 hex trace id}-{16 hex span id}-{2 hex flags}
func formatTraceparent(sc trace.SpanContext) string {
	flags := byte(0)
	if sc.TraceFlags().IsSampled() {
		flags = 0x01
	}
	return fmt.Sprintf("00-%s-%s-%02x", sc.TraceID(), sc.SpanID(), flags)
}

// parseTraceparent parses a W3C traceparent into a (remote) span context.
func parseTraceparent(tp string) (trace.SpanContext, error) {
	parts := strings.SplitN(tp, "-", 4)
	if len(parts) != 4 || parts[0] != "00" {
		return trace.SpanContext{}, fmt.Errorf("unsupported traceparent version/format: %q", tp)
	}
	traceID, err := trace.TraceIDFromHex(parts[1])
	if err != nil {
		return trace.SpanContext{}, fmt.Errorf("invalid trace id in traceparent: %w", err)
	}
	spanID, err := trace.SpanIDFromHex(parts[2])
	if err != nil {
		return trace.SpanContext{}, fmt.Errorf("invalid span id in traceparent: %w", err)
	}
	var traceFlags trace.TraceFlags
	if n, err := strconv.ParseUint(parts[3], 16, 8); err == nil {
		traceFlags = trace.TraceFlags(n)
	}
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: traceFlags,
		Remote:     true,
	}), nil
}

// compile-time guards: otelTracer must satisfy the full SDK Tracer surface
// (GetLogger and SpanName come from the embedded BaseTracer).
var (
	_ interceptor.Tracer        = otelTracer{}
	_ log.Logger                = log.Logger(nil)
	_ interceptor.TracerSpanRef = (*otelSpanRef)(nil)
)
