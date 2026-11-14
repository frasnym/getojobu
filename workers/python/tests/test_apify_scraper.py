"""Unit tests for ApifyScraper base class."""

from unittest.mock import patch

import httpx
import pytest

from repository.scraper.apify import ApifyScraper


class DummyApifyScraper(ApifyScraper):
    poll_interval: float = 0.01

    def build_run_input(self, keyword: str, location: str):
        return {"query": keyword, "loc": location}

    def parse_items(self, items):
        return [{"parsed_title": it.get("title")} for it in items]


@pytest.mark.asyncio
async def test_apify_scraper_success():
    scraper = DummyApifyScraper(
        name="test-scraper", vendor="test", actor_id="test~actor"
    )

    async def mock_handler(request: httpx.Request):
        url = str(request.url)
        if "/acts/test~actor/runs" in url:
            return httpx.Response(
                201, json={"data": {"id": "run_123", "defaultDatasetId": "ds_123"}}
            )
        if "/actor-runs/run_123" in url:
            return httpx.Response(
                200,
                json={"data": {"status": "SUCCEEDED", "defaultDatasetId": "ds_123"}},
            )
        if "/datasets/ds_123/items" in url:
            return httpx.Response(200, json=[{"title": "Software Engineer"}])
        return httpx.Response(404)

    mock_transport = httpx.MockTransport(mock_handler)
    real_async_client = httpx.AsyncClient

    def make_client(*args, **kwargs):
        kwargs["transport"] = mock_transport
        return real_async_client(*args, **kwargs)

    with (
        patch(
            "repository.scraper.apify_accounts.pick_token",
            return_value=("fake-tok", "dev (1/1)", {"credit_remaining_usd": 2.0}),
        ),
        patch("repository.scraper.apify_accounts._usage", return_value={}),
        patch("repository.scraper.apify.httpx.AsyncClient", side_effect=make_client),
    ):
        jobs = await scraper.scrape("python", "dubai")
        assert len(jobs) == 1
        assert jobs[0]["parsed_title"] == "Software Engineer"


@pytest.mark.asyncio
async def test_apify_scraper_failed_run_raises():
    scraper = DummyApifyScraper(
        name="test-scraper", vendor="test", actor_id="test~actor"
    )

    async def mock_handler(request: httpx.Request):
        url = str(request.url)
        if "/acts/test~actor/runs" in url:
            return httpx.Response(201, json={"data": {"id": "run_fail"}})
        if "/actor-runs/run_fail" in url:
            return httpx.Response(200, json={"data": {"status": "FAILED"}})
        return httpx.Response(404)

    mock_transport = httpx.MockTransport(mock_handler)
    real_async_client = httpx.AsyncClient

    def make_client(*args, **kwargs):
        kwargs["transport"] = mock_transport
        return real_async_client(*args, **kwargs)

    with (
        patch(
            "repository.scraper.apify_accounts.pick_token",
            return_value=("fake-tok", "dev (1/1)", {"credit_remaining_usd": 2.0}),
        ),
        patch("repository.scraper.apify_accounts._usage", return_value={}),
        patch("repository.scraper.apify.httpx.AsyncClient", side_effect=make_client),
        pytest.raises(RuntimeError, match="ended with status: FAILED"),
    ):
        await scraper.scrape("python", "dubai")


@pytest.mark.asyncio
async def test_apify_scraper_no_credit():
    scraper = DummyApifyScraper(
        name="test-scraper", vendor="test", actor_id="test~actor"
    )
    with patch(
        "repository.scraper.apify_accounts.pick_token",
        return_value=(None, None, {}),
    ), pytest.raises(RuntimeError, match="no Apify account has enough credit"):
        await scraper.scrape("python", "dubai")
