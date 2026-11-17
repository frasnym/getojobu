package temporal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.temporal.io/sdk/interceptor"
)

func TestOtelTracer_options_and_round_trip(t *testing.T) {
	// Arrange.
	tr := otelTracer{}

	// Act.
	opts := tr.Options()

	// Assert: tracing interceptor requires a span context key and header key.
	require.NotNil(t, opts.SpanContextKey, "SpanContextKey must be set")
	assert.NotEmpty(t, opts.HeaderKey, "HeaderKey must be set")
	assert.False(t, opts.EncodeHeaders, "SDK serializes the carrier into the standard JSON payload")
}

func TestOtelTracer_start_finish_and_context_round_trip(t *testing.T) {
	// Arrange: a real SDK provider so spans carry valid contexts, mirroring
	// telemetry.Init having been called. Under the no-op provider the span
	// context is invalid and SpanFromContext correctly returns nil.
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	defer otel.SetTracerProvider(noop.NewTracerProvider())

	tr := otelTracer{}
	ctx := context.Background()

	// Act: start a span, stash it in a context, pull it back out.
	span, err := tr.StartSpan(&interceptor.TracerStartSpanOptions{
		Operation: "StartWorkflow",
		Name:      "AnalyzeJob",
		Tags:      map[string]string{"temporalWorkflowID": "wf-1"},
	})
	require.NoError(t, err)
	require.NotNil(t, span)

	ctx2 := tr.ContextWithSpan(ctx, span)
	got := tr.SpanFromContext(ctx2)
	require.NotNil(t, got)

	// Act/Assert: finish with no error records a successful span.
	span.Finish(&interceptor.TracerFinishSpanOptions{})
}

func TestOtelTracer_marshal_unmarshal_header_round_trip(t *testing.T) {
	// Arrange: a real SDK provider so spans carry valid (sampled) contexts,
	// mirroring telemetry.Init having been called.
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	defer otel.SetTracerProvider(noop.NewTracerProvider())

	tr := otelTracer{}
	span, err := tr.StartSpan(&interceptor.TracerStartSpanOptions{
		Operation: "RunActivity",
		Name:      "evaluate",
	})
	require.NoError(t, err)

	// Act: marshal into headers, then unmarshal back.
	headers, err := tr.MarshalSpan(span)
	require.NoError(t, err)
	require.NotEmpty(t, headers, "a sampled span must produce tracing headers")

	ref, err := tr.UnmarshalSpan(headers)
	require.NoError(t, err)
	require.NotNil(t, ref)

	// Assert: unmarshaled ref is usable as a parent for a new span.
	child, err := tr.StartSpan(&interceptor.TracerStartSpanOptions{
		Operation: "RunActivity",
		Name:      "child",
		Parent:    ref,
	})
	require.NoError(t, err)
	require.NotNil(t, child)
	child.Finish(&interceptor.TracerFinishSpanOptions{})
	span.Finish(&interceptor.TracerFinishSpanOptions{})
}

func TestOtelTracer_marshal_matches_python_worker_header_contract(t *testing.T) {
	// The Python worker's temporalio.contrib.opentelemetry TracingInterceptor reads
	// trace context from header "_tracer-data" and expects a W3C propagator carrier
	// with a "traceparent" key. Any other header or carrier key makes Python skip
	// the workflow span, so every activity shows up as an orphan root trace.
	otel.SetTracerProvider(sdktrace.NewTracerProvider())
	defer otel.SetTracerProvider(noop.NewTracerProvider())

	tr := otelTracer{}
	assert.Equal(t, "_tracer-data", tr.Options().HeaderKey)

	span, err := tr.StartSpan(&interceptor.TracerStartSpanOptions{
		Operation: "StartWorkflow",
		Name:      "JobAnalyzeWorkflow",
	})
	require.NoError(t, err)
	defer span.Finish(&interceptor.TracerFinishSpanOptions{})

	headers, err := tr.MarshalSpan(span)
	require.NoError(t, err)
	require.Len(t, headers, 1, "carrier must contain only the W3C traceparent key")
	traceparent, ok := headers["traceparent"]
	require.True(t, ok, "carrier must use the W3C traceparent key, got %v", headers)
	assert.Regexp(t, `^00-[0-9a-f]{32}-[0-9a-f]{16}-0[01]$`, traceparent)

	// Python feeds the decoded carrier to its propagator, so UnmarshalSpan must
	// accept the same W3C carrier shape.
	ref, err := tr.UnmarshalSpan(map[string]string{"traceparent": traceparent})
	require.NoError(t, err)
	require.NotNil(t, ref)
}

func TestOtelTracer_attributes_and_status(t *testing.T) {
	// Arrange.
	tr := otelTracer{}
	span, err := tr.StartSpan(&interceptor.TracerStartSpanOptions{
		Operation: "StartWorkflow",
		Name:      "AnalyzeJob",
		Tags: map[string]string{
			"temporalWorkflowID": "wf-1",
			"temporalRunID":      "run-1",
		},
	})
	require.NoError(t, err)

	// Act: finish with an error.
	span.Finish(&interceptor.TracerFinishSpanOptions{
		Error: assert.AnError,
	})

	// Assert: the span was created and finished without panicking.
	require.NotNil(t, span)
}

// compile-time guards.
var _ interceptor.Tracer = (*otelTracer)(nil)
