// Package telemetry initializes OpenTelemetry for the API. When the OTLP
// endpoint is configured, it installs global tracer and meter providers that
// export via OTLP/gRPC; when it is empty, telemetry is a complete no-op and
// local development is unchanged.
package telemetry

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

// serviceName identifies this service in the trace backend.
const serviceName = "getojobu-api"

// Init installs the global OpenTelemetry providers. When endpoint is empty it
// installs no-op providers and returns a nil stopper. When configured, it
// returns a stopper that flushes and shuts both providers down; export is
// asynchronous, so Init itself never contacts the collector.
func Init(ctx context.Context, endpoint string) (stop func(context.Context) error, err error) {
	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, fmt.Errorf("create telemetry resource: %w", err)
	}

	if endpoint == "" {
		// Disabled: no SDK, no export, no behavior change. The otel global
		// defaults are already no-op providers, so nothing is installed.
		return nil, nil
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter),
	)
	otel.SetTracerProvider(tracerProvider)

	// Extract incoming traceparent headers so callers can continue their trace.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpoint(endpoint), otlpmetricgrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
	)
	otel.SetMeterProvider(meterProvider)

	var shutdownOnce sync.Once
	return func(ctx context.Context) error {
		var shutdownErr error
		shutdownOnce.Do(func() {
			if err := tracerProvider.Shutdown(ctx); err != nil {
				shutdownErr = fmt.Errorf("shutdown tracer provider: %w", err)
			}
			if err := meterProvider.Shutdown(ctx); err != nil {
				shutdownErr = fmt.Errorf("shutdown meter provider: %w", err)
			}
		})
		return shutdownErr
	}, nil
}
