"""Unit tests for UserJobScrapeWorkflow and its activities."""

from __future__ import annotations

from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from deliveries.consumer.activities.user_job_scrape import (
    fetch_and_extract_job,
    init_user_job_scrape_container,
    queue_scraped_job_analysis,
    update_scraped_job,
)
from workflows.user_job_scrape import UserJobScrapeWorkflow


class FakeScrapeService:
    def __init__(
        self, result: dict[str, Any] | None = None, error: Exception | None = None
    ):
        self._result = result or {
            "title": "Senior Engineer",
            "company": "Acme",
            "location": "Berlin",
            "description": "Build things",
            "apply_url": "https://example.com/apply",
            "posted_date": "2026-11-01",
            "is_remote": True,
            "visa_relocation": False,
        }
        self._error = error

    async def extract(self, page_text: str) -> dict[str, Any]:
        if self._error is not None:
            raise self._error
        return self._result


@pytest.fixture(autouse=True)
def setup_container():
    container = MagicMock()
    container.usecase.user_job_scrape = FakeScrapeService()
    init_user_job_scrape_container(container)
    with patch(
        "deliveries.consumer.activities.user_job_scrape.socket.getaddrinfo",
        return_value=[(2, 1, 6, "", ("93.184.216.34", 80))],
    ):
        yield
    init_user_job_scrape_container(None)


def _html_response() -> MagicMock:
    response = MagicMock()
    response.raise_for_status = MagicMock()
    response.text = (
        "<html><body><h1>Senior Engineer</h1><p>Build things</p></body></html>"
    )
    return response


@pytest.mark.asyncio
async def test_fetch_and_extract_job_returns_fields():
    client = MagicMock()
    client.__aenter__ = AsyncMock(return_value=client)
    client.__aexit__ = AsyncMock(return_value=None)
    client.get = AsyncMock(return_value=_html_response())

    with patch(
        "deliveries.consumer.activities.user_job_scrape.httpx.AsyncClient",
        return_value=client,
    ):
        result = await fetch_and_extract_job(
            {"source_url": "https://example.com/jobs/1"}
        )

    assert result["title"] == "Senior Engineer"
    assert result["company"] == "Acme"
    assert result["location"] == "Berlin"
    assert result["is_remote"] is True


@pytest.mark.asyncio
async def test_fetch_redirect_to_private_url_is_rejected():
    import httpx

    client = MagicMock()
    client.__aenter__ = AsyncMock(return_value=client)
    client.__aexit__ = AsyncMock(return_value=None)
    redirect = MagicMock()
    redirect.status_code = 302
    redirect.headers = {"location": "http://169.254.169.254/latest/meta-data"}
    redirect.raise_for_status = MagicMock()
    client.get = AsyncMock(return_value=redirect)

    with patch(
        "deliveries.consumer.activities.user_job_scrape.httpx.AsyncClient",
        return_value=client,
    ):
        with pytest.raises(ValueError):
            await fetch_and_extract_job({"source_url": "https://example.com/jobs/1"})


@pytest.mark.asyncio
async def test_fetch_and_extract_job_rejects_private_url():
    with pytest.raises(ValueError):
        await fetch_and_extract_job({"source_url": "http://127.0.0.1/jobs/1"})


@pytest.mark.asyncio
async def test_fetch_and_extract_job_http_error():
    import httpx

    client = MagicMock()
    client.__aenter__ = AsyncMock(return_value=client)
    client.__aexit__ = AsyncMock(return_value=None)
    client.get = AsyncMock(
        side_effect=httpx.HTTPStatusError(
            "boom",
            request=httpx.Request("GET", "https://example.com"),
            response=httpx.Response(500),
        )
    )

    with patch(
        "deliveries.consumer.activities.user_job_scrape.httpx.AsyncClient",
        return_value=client,
    ):
        with pytest.raises(httpx.HTTPError):
            await fetch_and_extract_job({"source_url": "https://example.com/jobs/1"})


@pytest.mark.asyncio
async def test_update_scraped_job_patches_fields_and_slug():
    api = MagicMock()
    api.patch_job_status = AsyncMock(return_value={"slug": "new-slug"})
    container = MagicMock()
    container.repository.api = api
    container.usecase.user_job_scrape = FakeScrapeService()
    init_user_job_scrape_container(container)

    extracted = {
        "title": "Senior Engineer",
        "company": "Acme",
        "location": "Berlin",
        "description": "Build things",
        "apply_url": "https://example.com/apply",
        "posted_date": "2026-11-01",
        "is_remote": True,
        "visa_relocation": False,
    }
    result = await update_scraped_job(
        {
            "old_slug": "m-abcdef12-scrape-123",
            "owner_id": "abcdef12-3456-7890",
            "extracted": extracted,
        }
    )

    args, kwargs = api.patch_job_status.call_args
    assert args[0] == "m-abcdef12-scrape-123"
    assert args[1] == "new"
    assert kwargs["new_slug"] == "m-abcdef12-senior-engineer-acme-berlin"
    assert kwargs["title"] == "Senior Engineer"
    assert result["slug"] == "m-abcdef12-senior-engineer-acme-berlin"


@pytest.mark.asyncio
async def test_update_scraped_job_is_idempotent_on_404():
    import httpx

    api = MagicMock()

    async def fake_patch(slug, status, **kwargs):
        raise httpx.HTTPStatusError(
            "not found",
            request=httpx.Request("PATCH", f"https://api/{slug}"),
            response=httpx.Response(404),
        )

    api.patch_job_status = AsyncMock(side_effect=fake_patch)
    container = MagicMock()
    container.repository.api = api
    init_user_job_scrape_container(container)

    result = await update_scraped_job(
        {
            "old_slug": "m-abcdef12-scrape-123",
            "owner_id": "abcdef12-3456-7890",
            "extracted": {
                "title": "Senior Engineer",
                "company": "Acme",
                "location": "Berlin",
            },
        }
    )
    assert result["slug"] == "m-abcdef12-senior-engineer-acme-berlin"


@pytest.mark.asyncio
async def test_queue_scraped_job_analysis():
    api = MagicMock()
    api.post = AsyncMock(return_value={"workflow_ids": ["job-analyze-1"]})
    container = MagicMock()
    container.repository.api = api
    init_user_job_scrape_container(container)

    result = await queue_scraped_job_analysis(
        {"slug": "m-abcdef12-senior-engineer-acme-berlin"}
    )
    api.post.assert_awaited_once()
    assert api.post.call_args.args[0] == "/v1/jobs/bulk-queue-analyze"
    assert result["workflow_ids"] == ["job-analyze-1"]


@pytest.mark.asyncio
async def test_workflow_sequence():
    calls: list[str] = []

    async def fake_activity(_activity, _input, **_kwargs):
        name = getattr(_activity, "__name__", "")
        calls.append(name)
        if name == "fetch_and_extract_job":
            return {"title": "Senior Engineer"}
        if name == "update_scraped_job":
            return {"slug": "m-abcdef12-senior-engineer-acme-berlin"}
        return {"workflow_ids": ["job-analyze-1"]}

    with patch(
        "workflows.user_job_scrape.workflow.execute_activity",
        side_effect=fake_activity,
    ):
        result = await UserJobScrapeWorkflow().run(
            {
                "job_id": 1,
                "slug": "m-abcdef12-scrape-1",
                "source_url": "https://example.com/jobs/1",
                "user_id": "abcdef12-3456-7890",
                "owner_id": "abcdef12-3456-7890",
            }
        )

    assert calls == [
        "fetch_and_extract_job",
        "update_scraped_job",
        "queue_scraped_job_analysis",
    ]
    assert result["status"] == "new"


@pytest.mark.asyncio
async def test_workflow_failure_uses_current_slug():
    calls: list[tuple[str, dict[str, Any]]] = []

    async def fake_activity(activity, payload, **_kwargs):
        name = getattr(activity, "__name__", "")
        calls.append((name, payload))
        if name == "fetch_and_extract_job":
            return {"title": "Senior Engineer"}
        if name == "update_scraped_job":
            return {"slug": "m-abcdef12-senior-engineer-acme-berlin"}
        if name == "queue_scraped_job_analysis":
            raise RuntimeError("analysis unavailable")
        return {}

    with patch(
        "workflows.user_job_scrape.workflow.execute_activity", side_effect=fake_activity
    ):
        result = await UserJobScrapeWorkflow().run(
            {
                "slug": "m-abcdef12-scrape-1",
                "owner_id": "abcdef12-3456-7890",
                "source_url": "https://example.com/jobs/1",
            }
        )

    assert result["status"] == "scrape_failed"
    failed = [p for name, p in calls if name == "mark_scrape_failed"]
    assert len(failed) == 1
    assert failed[0]["slug"] == "m-abcdef12-senior-engineer-acme-berlin"
