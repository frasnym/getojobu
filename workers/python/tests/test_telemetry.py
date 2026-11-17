"""Tests for utils/telemetry.py and the otel_endpoint config field."""

from unittest.mock import patch

from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

from config.config import load
from utils.telemetry import setup_otel

BASE_ENV = {
    "TEMPORAL_NAMESPACE": "getojobu",
    "API_BASE_URL": "http://localhost:8080",
    "API_KEY": "test-api-key",
    "OMNIROUTE_BASE_URL": "http://localhost:20128",
    "OMNIROUTE_API_KEY": "test-omni-key",
    "SCRAPER_METHODS_BY_BOARD_LINKEDIN": "linkedin-jobspy",
}


def test_setup_otel_empty_endpoint_is_noop():
    """Empty endpoint installs nothing and needs no network."""
    before = trace.get_tracer_provider()

    setup_otel("")

    assert trace.get_tracer_provider() is before


def test_setup_otel_with_endpoint_installs_otlp_provider():
    """A non-empty endpoint installs a TracerProvider with an OTLP exporter."""
    setup_otel("localhost:4317")

    provider = trace.get_tracer_provider()
    assert isinstance(provider, TracerProvider)
    # Exactly one span processor, a batch processor wrapping an OTLP gRPC exporter.
    processors = provider._active_span_processor._span_processors
    assert len(processors) == 1
    assert isinstance(processors[0], BatchSpanProcessor)
    exporter = processors[0].span_exporter
    assert type(exporter).__name__ == "OTLPSpanExporter"


def test_config_otel_endpoint_from_env():
    """OTEL_EXPORTER_OTLP_ENDPOINT passes through to cfg.otel_endpoint."""
    # Unset ⇒ empty string.
    with patch.dict("os.environ", BASE_ENV, clear=True):
        cfg = load()
        assert cfg.otel_endpoint == ""

    # Set ⇒ value passes through.
    env = {**BASE_ENV, "OTEL_EXPORTER_OTLP_ENDPOINT": "http://jaeger:4317"}
    with patch.dict("os.environ", env, clear=True):
        cfg = load()
        assert cfg.otel_endpoint == "http://jaeger:4317"
