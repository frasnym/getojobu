package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func TestInit_empty_endpoint_is_noop(t *testing.T) {
	stop, err := Init(context.Background(), "")

	require.NoError(t, err, "noop init must not fail")
	assert.Nil(t, stop, "expected nil stopper when telemetry disabled")
}

func TestInit_configured_endpoint_returns_stopper(t *testing.T) {
	stop, err := Init(context.Background(), "localhost:4317")

	require.NoError(t, err, "init with endpoint must not fail")
	require.NotNil(t, stop, "expected non-nil stopper when telemetry enabled")

	// The W3C TraceContext propagator must be installed so incoming
	// traceparent headers continue the caller's trace.
	_, ok := otel.GetTextMapPropagator().(propagation.TraceContext)
	assert.True(t, ok, "expected TraceContext propagator installed when telemetry enabled")
}
