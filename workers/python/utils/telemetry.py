"""OpenTelemetry setup for the Python Temporal worker.

Mirrors apps/backend/internal/utils/telemetry/telemetry.go: opt-in via
OTEL_EXPORTER_OTLP_ENDPOINT. When the endpoint is empty, nothing is
installed and the process behaves exactly as before (no spans, no
connections). When set, a global TracerProvider exports spans over OTLP
gRPC to that endpoint (the Jaeger collector) and the Temporal tracing
interceptor becomes available for the workers.
"""

from __future__ import annotations

from collections.abc import MutableMapping

from opentelemetry import trace
from opentelemetry.propagate import inject
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

_TRACER_PROVIDER: TracerProvider | None = None

SERVICE_NAME = "getojobu-worker"


def setup_otel(endpoint: str, service_name: str = SERVICE_NAME) -> None:
    """Install the OTLP tracer provider when tracing is enabled.

    No-op when ``endpoint`` is empty so local development without Jaeger
    stays unchanged.
    """
    global _TRACER_PROVIDER

    if not endpoint:
        return

    provider = TracerProvider(resource=Resource.create({"service.name": service_name}))
    provider.add_span_processor(
        BatchSpanProcessor(OTLPSpanExporter(endpoint=endpoint, insecure=True))
    )
    trace.set_tracer_provider(provider)
    _TRACER_PROVIDER = provider


def is_enabled() -> bool:
    """Whether setup_otel installed a real provider."""
    return _TRACER_PROVIDER is not None


def inject_trace_context(headers: MutableMapping[str, str]) -> None:
    """Copy the active span into outgoing headers as a W3C traceparent.

    No-op when there is no active span (OTel disabled), so local development
    without Jaeger stays unchanged.
    """
    inject(headers)
