"""Tests for GetojobuAPI response contract handling."""

from collections.abc import Callable
from typing import Any

import httpx
import pytest

from models.api_response import ApiResponse
from models.scoring_config import ScoringConfig
from repository.getojobu_api import NewGetojobuAPI


def _mock_transport(
    handler: Callable[[httpx.Request], httpx.Response],
    api_key: str = "secret-key",
) -> httpx.AsyncClient:
    transport = httpx.MockTransport(handler)
    return httpx.AsyncClient(
        transport=transport,
        base_url="http://testserver",
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        },
    )


@pytest.mark.asyncio
async def test_get_job_unwraps_standard_success_response():
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.headers["authorization"] == "Bearer secret-key"
        assert request.url.path == "/v1/jobs/test-slug"
        return httpx.Response(
            200,
            json={
                "success": True,
                "data": {
                    "id": 101,
                    "slug": "test-slug",
                    "status": "new",
                    "description": "Go engineer needed",
                },
                "meta": {
                    "timestamp": "2026-11-11T08:57:00Z",
                    "requestId": "req_123",
                },
                "error": None,
            },
        )

    api = NewGetojobuAPI(base_url="http://testserver", api_key="secret-key")
    api._client = _mock_transport(handler)

    job = await api.get_job("test-slug")
    assert job["id"] == 101
    assert job["slug"] == "test-slug"
    assert job["status"] == "new"
    await api.aclose()


@pytest.mark.asyncio
async def test_get_job_raises_on_standard_error_response():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200,  # Or 404
            json={
                "success": False,
                "data": None,
                "meta": {
                    "timestamp": "2026-11-11T08:58:00Z",
                    "requestId": "req_99abc123",
                },
                "error": {
                    "code": "NOT_FOUND",
                    "message": "job not found",
                    "details": [],
                },
            },
        )

    api = NewGetojobuAPI(base_url="http://testserver", api_key="secret-key")
    api._client = _mock_transport(handler)

    with pytest.raises(
        RuntimeError, match="failed with API error \\(NOT_FOUND\\): job not found"
    ):
        await api.get_job("non-existent-slug")
    await api.aclose()


@pytest.mark.asyncio
async def test_get_job_raises_on_non_dict_raw_response():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json="invalid-string-not-dict")

    api = NewGetojobuAPI(base_url="http://testserver", api_key="secret-key")
    api._client = _mock_transport(handler)

    with pytest.raises(RuntimeError, match="response is not a valid JSON object"):
        await api.get_job("test-slug")
    await api.aclose()


@pytest.mark.asyncio
async def test_get_job_raises_on_non_dict_data_field():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json={"success": True, "data": "not-a-dict"})

    api = NewGetojobuAPI(base_url="http://testserver", api_key="secret-key")
    api._client = _mock_transport(handler)

    with pytest.raises(RuntimeError, match="'data' is not an object"):
        await api.get_job("test-slug")
    await api.aclose()


@pytest.mark.asyncio
async def test_patch_job_status_unwraps_standard_success_response():
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.method == "PATCH"
        return httpx.Response(
            200,
            json={
                "success": True,
                "data": {
                    "id": 102,
                    "slug": "test-slug",
                    "status": "analyze_pending",
                },
                "meta": {
                    "timestamp": "2026-11-11T08:57:00Z",
                    "requestId": "req_124",
                },
                "error": None,
            },
        )

    api = NewGetojobuAPI(base_url="http://testserver", api_key="secret-key")
    api._client = _mock_transport(handler)

    result = await api.patch_job_status("test-slug", "analyze_pending")
    assert result["status"] == "analyze_pending"
    await api.aclose()


@pytest.mark.asyncio
async def test_patch_job_status_raises_on_api_error_response():
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200,
            json={
                "success": False,
                "data": None,
                "meta": {
                    "timestamp": "2026-11-11T08:58:00Z",
                    "requestId": "req_error",
                },
                "error": {
                    "code": "VALIDATION_FAILED",
                    "message": "invalid status transition: job is already finalized",
                    "details": [
                        {"field": "status", "issue": "cannot transition from analyzed"}
                    ],
                },
            },
        )

    api = NewGetojobuAPI(base_url="http://testserver", api_key="secret-key")
    api._client = _mock_transport(handler)

    with pytest.raises(
        RuntimeError, match="VALIDATION_FAILED.*invalid status transition"
    ):
        await api.patch_job_status("test-slug", "analyze_pending")
    await api.aclose()


def test_api_response_model_validation():
    payload = {
        "success": True,
        "data": {"id": 1, "title": "Developer"},
        "meta": {
            "timestamp": "2026-11-11T08:57:00Z",
            "requestId": "req_88jfkd99",
            "pagination": {
                "page": 1,
                "limit": 20,
                "totalItems": 145,
                "totalPages": 8,
                "hasNext": True,
                "hasPrev": False,
            },
        },
        "error": None,
    }

    resp = ApiResponse[dict[str, Any]].model_validate(payload)
    assert resp.success is True
    assert resp.data["id"] == 1
    assert resp.meta.request_id == "req_88jfkd99"
    assert resp.meta.pagination.total_items == 145
    assert resp.meta.pagination.total_pages == 8
    assert resp.meta.pagination.has_next is True
    assert resp.error is None


def test_api_response_model_error_validation():
    payload = {
        "success": False,
        "data": None,
        "meta": {
            "timestamp": "2026-11-11T08:58:00Z",
            "requestId": "req_99abc123",
        },
        "error": {
            "code": "VALIDATION_FAILED",
            "message": "The input provided is invalid.",
            "details": [
                {
                    "field": "email",
                    "issue": "Format is invalid.",
                }
            ],
        },
    }

    resp = ApiResponse[None].model_validate(payload)
    assert resp.success is False
    assert resp.data is None
    assert resp.error.code == "VALIDATION_FAILED"
    assert resp.error.message == "The input provided is invalid."
    assert len(resp.error.details) == 1
    assert resp.error.details[0].field == "email"
    assert resp.error.details[0].issue == "Format is invalid."


@pytest.mark.asyncio
async def test_get_active_scoring_config_success():
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.headers["authorization"] == "Bearer secret-key"
        assert request.url.path == "/v1/scoring-configs/active"
        assert request.url.params["family"] == "technology"
        return httpx.Response(
            200,
            json={
                "success": True,
                "data": {
                    "id": "e0b8d3a1-1234-5678-90ab-cdef12345678",
                    "family": "technology",
                    "version": 1,
                    "categories": [
                        {
                            "name": "technical_skills",
                            "weight": 0.60,
                            "description": "skills",
                        },
                        {"name": "experience", "weight": 0.40, "description": "exp"},
                    ],
                    "is_active": True,
                    "created_at": "2026-11-11T12:00:00Z",
                    "updated_at": "2026-11-11T12:00:00Z",
                },
                "meta": {
                    "timestamp": "2026-11-11T08:57:00Z",
                    "requestId": "req_123",
                },
                "error": None,
            },
        )

    api = NewGetojobuAPI(base_url="http://testserver", api_key="secret-key")
    api._client = _mock_transport(handler)

    config = await api.get_active_scoring_config("technology")
    validated = ScoringConfig.model_validate(config)
    assert validated.family == "technology"
    assert validated.version == 1
    assert validated.is_active is True
    assert len(validated.categories) == 2
    await api.aclose()


@pytest.mark.asyncio
async def test_post_success_dict():
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.headers["authorization"] == "Bearer secret-key"
        assert request.url.path == "/v1/jobs/bulk-queue-analyze"
        return httpx.Response(
            200,
            json={
                "success": True,
                "data": {"workflow_ids": ["job-analyze-1", "job-analyze-2"]},
                "error": None,
            },
        )

    api = NewGetojobuAPI(base_url="http://testserver", api_key="secret-key")
    api._client = _mock_transport(handler)

    res = await api.post("/v1/jobs/bulk-queue-analyze", json=[{"slug": "s1"}])
    assert res == {"workflow_ids": ["job-analyze-1", "job-analyze-2"]}
    await api.aclose()


@pytest.mark.asyncio
async def test_post_success_list():
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.headers["authorization"] == "Bearer secret-key"
        assert request.url.path == "/v1/jobs/bulk"
        return httpx.Response(
            200,
            json={
                "success": True,
                "data": [
                    {"slug": "job-1", "created": True},
                    {"slug": "job-2", "created": False},
                ],
                "error": None,
            },
        )

    api = NewGetojobuAPI(base_url="http://testserver", api_key="secret-key")
    api._client = _mock_transport(handler)

    res = await api.post("/v1/jobs/bulk", json={"jobs": []})
    assert isinstance(res, list)
    assert len(res) == 2
    assert res[0]["slug"] == "job-1"
    assert res[0]["created"] is True
    await api.aclose()
