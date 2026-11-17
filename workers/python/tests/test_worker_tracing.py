"""Tests for the worker's Temporal tracing interceptor wiring."""

from temporalio.contrib.opentelemetry import TracingInterceptor

from worker.main import build_worker_interceptors


def test_no_interceptors_when_otel_disabled():
    assert build_worker_interceptors(otel_enabled=False) == []


def test_interceptor_creates_workflow_spans_without_client_parent():
    """Workflows started without an API span (cron/CLI/schedules) still need a
    RunWorkflow span; otherwise each activity becomes an orphan root trace.
    """
    interceptors = build_worker_interceptors(otel_enabled=True)

    assert len(interceptors) == 1
    assert isinstance(interceptors[0], TracingInterceptor)
    assert interceptors[0]._always_create_workflow_spans is True
