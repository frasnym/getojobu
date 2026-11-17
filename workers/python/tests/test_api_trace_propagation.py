"""Tests that worker -> Go API HTTP calls carry the active OTel trace context."""

import httpx
import pytest
from opentelemetry.sdk.trace import TracerProvider

from repository.getojobu_api import new_api_http_client


def _client_capturing_headers(seen: dict) -> httpx.AsyncClient:
    def handler(request: httpx.Request) -> httpx.Response:
        seen["headers"] = request.headers
        return httpx.Response(200, json={"ok": True})

    return new_api_http_client(
        base_url="http://testserver",
        api_key="secret-key",
        transport=httpx.MockTransport(handler),
    )


@pytest.mark.asyncio
async def test_api_request_carries_traceparent_when_span_active():
    seen: dict = {}
    client = _client_capturing_headers(seen)
    tracer = TracerProvider().get_tracer("test")

    with tracer.start_as_current_span("run_activity"):
        await client.get("/v1/jobs/test-slug")

    traceparent = seen["headers"].get("traceparent")
    assert traceparent is not None
    assert traceparent.startswith("00-")
    await client.aclose()


@pytest.mark.asyncio
async def test_api_request_has_no_traceparent_without_active_span():
    seen: dict = {}
    client = _client_capturing_headers(seen)

    await client.get("/v1/jobs/test-slug")

    assert "traceparent" not in seen["headers"]
    await client.aclose()
