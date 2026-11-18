"""Unit tests for the Arbeitnow feed scraper."""

from datetime import UTC, datetime
from unittest.mock import AsyncMock, patch

import httpx
import pytest

from repository.scraper.arbeitnow_api import (
    ArbeitnowApiScraper,
    matches_keyword,
    plain_text,
)


@pytest.fixture(autouse=True)
def _no_sleep():
    """Keep pacing and retry backoff out of the test runtime."""
    with patch("repository.scraper.arbeitnow_api.asyncio.sleep", new=AsyncMock()):
        yield


def test_matches_keyword_is_token_and_order_insensitive():
    assert matches_keyword("Backend Engineer - Team Optimize", "backend engineer")
    assert matches_keyword("Engineer, Backend", "backend engineer")
    assert matches_keyword(
        "Senior Software Engineer (Backend-focused)", "backend engineer"
    )
    assert not matches_keyword("Backend Developer", "backend engineer")
    assert not matches_keyword("Frontend Engineer", "backend engineer")


def test_plain_text_strips_html_and_caps_length():
    assert plain_text("<p>Hello&nbsp;<b>World</b></p>") == "Hello World"
    assert plain_text(None) == ""
    assert len(plain_text("x" * 20000)) == 16000


def _row(**over):
    row = {
        "slug": "backend-engineer-golang-123",
        "title": "Backend Engineer (Golang)",
        "company_name": "AMBOSS",
        "location": "Berlin",
        "description": "<p>We use <b>Go</b></p>",
        "remote": False,
        "url": (
            "https://www.arbeitnow.com/jobs/companies/amboss/"
            "backend-engineer-golang-123"
        ),
        "tags": ["backend"],
        "job_types": ["full_time"],
        "created_at": int(datetime.now(UTC).timestamp()),
    }
    row.update(over)
    return row


def _client_factory(pages, calls):
    real_async_client = httpx.AsyncClient

    def make_client(*args, **kwargs):
        def handler(request: httpx.Request):
            page = int(dict(request.url.params)["page"])
            calls.append(page)
            return httpx.Response(200, json={"data": pages.get(page, []), "meta": {}})

        kwargs["transport"] = httpx.MockTransport(handler)
        return real_async_client(*args, **kwargs)

    return make_client


@pytest.mark.asyncio
async def test_scrape_maps_feed_row_to_enrich_shape():
    calls = []
    scraper = ArbeitnowApiScraper()
    with patch(
        "repository.scraper.arbeitnow_api.httpx.AsyncClient",
        side_effect=_client_factory({1: [_row()]}, calls),
    ):
        jobs = await scraper.scrape("golang", "germany")

    assert calls == [1, 2]
    assert len(jobs) == 1
    job = jobs[0]
    assert job["title"] == "Backend Engineer (Golang)"
    assert job["company"] == "AMBOSS"
    assert job["location"] == "Berlin"
    assert job["description"] == "We use Go"
    assert job["is_remote"] is False
    assert job["site"] == "arbeitnow"
    assert job["apply_url"] == job["job_url"]
    assert job["date_posted"].tzinfo is not None


@pytest.mark.asyncio
async def test_scrape_matches_keyword_in_tags():
    calls = []
    scraper = ArbeitnowApiScraper()
    pages = {
        1: [
            _row(
                slug="tagged-row",
                title="Staff Engineer",
                tags=["golang", "distributed-systems"],
            ),
            _row(
                slug="untagged-row",
                title="Staff Engineer",
                tags=["python"],
            ),
        ]
    }
    with patch(
        "repository.scraper.arbeitnow_api.httpx.AsyncClient",
        side_effect=_client_factory(pages, calls),
    ):
        jobs = await scraper.scrape("golang", "germany")

    assert len(jobs) == 1
    assert jobs[0]["title"] == "Staff Engineer"


@pytest.mark.asyncio
async def test_scrape_filters_by_country_host():
    calls = []
    scraper = ArbeitnowApiScraper()
    pages = {
        1: [
            _row(
                slug="uk-row",
                url="https://www.arbeitnow.co.uk/jobs/companies/x/uk-row",
            ),
            _row(),
        ]
    }
    with patch(
        "repository.scraper.arbeitnow_api.httpx.AsyncClient",
        side_effect=_client_factory(pages, calls),
    ):
        jobs = await scraper.scrape("golang", "germany")

    assert [j["title"] for j in jobs] == ["Backend Engineer (Golang)"]


@pytest.mark.asyncio
async def test_scrape_drops_old_rows():
    calls = []
    scraper = ArbeitnowApiScraper()
    stale = int(datetime.now(UTC).timestamp()) - 2 * 86400
    with patch(
        "repository.scraper.arbeitnow_api.httpx.AsyncClient",
        side_effect=_client_factory({1: [_row(created_at=stale)]}, calls),
    ):
        jobs = await scraper.scrape("golang", "germany")

    assert jobs == []


@pytest.mark.asyncio
async def test_scrape_uses_remote_when_location_empty():
    calls = []
    scraper = ArbeitnowApiScraper()
    pages = {
        1: [
            _row(slug="remote-row", location="", remote=True),
            _row(slug="nowhere-row", location="", remote=False),
        ]
    }
    with patch(
        "repository.scraper.arbeitnow_api.httpx.AsyncClient",
        side_effect=_client_factory(pages, calls),
    ):
        jobs = await scraper.scrape("golang", "germany")

    assert [j["location"] for j in jobs] == ["Remote"]
    assert jobs[0]["is_remote"] is True


@pytest.mark.asyncio
async def test_scrape_unknown_location_makes_no_requests():
    calls = []
    scraper = ArbeitnowApiScraper()
    with patch(
        "repository.scraper.arbeitnow_api.httpx.AsyncClient",
        side_effect=_client_factory({}, calls),
    ):
        jobs = await scraper.scrape("golang", "indonesia")

    assert jobs == []
    assert calls == []


@pytest.mark.asyncio
async def test_scrape_caches_feed_across_keywords():
    calls = []
    scraper = ArbeitnowApiScraper()
    with patch(
        "repository.scraper.arbeitnow_api.httpx.AsyncClient",
        side_effect=_client_factory({1: [_row()]}, calls),
    ):
        await scraper.scrape("golang", "germany")
        await scraper.scrape("backend engineer", "germany")

    assert calls == [1, 2]


def _status_factory(pages, calls, statuses):
    """Serve pages, returning the next status from `statuses` per page call."""
    real_async_client = httpx.AsyncClient

    def make_client(*args, **kwargs):
        def handler(request: httpx.Request):
            page = int(dict(request.url.params)["page"])
            calls.append(page)
            queued = statuses.get(page) or []
            status = queued.pop(0) if queued else 200
            if status != 200:
                return httpx.Response(status, headers={"retry-after": "1"})
            return httpx.Response(200, json={"data": pages.get(page, []), "meta": {}})

        kwargs["transport"] = httpx.MockTransport(handler)
        return real_async_client(*args, **kwargs)

    return make_client


@pytest.mark.asyncio
async def test_scrape_retries_after_rate_limit():
    calls = []
    scraper = ArbeitnowApiScraper()
    with patch(
        "repository.scraper.arbeitnow_api.httpx.AsyncClient",
        side_effect=_status_factory({1: [_row()]}, calls, {1: [429]}),
    ):
        jobs = await scraper.scrape("golang", "germany")

    assert calls == [1, 1, 2]
    assert len(jobs) == 1


@pytest.mark.asyncio
async def test_scrape_keeps_partial_feed_when_rate_limit_persists():
    calls = []
    scraper = ArbeitnowApiScraper()
    with patch(
        "repository.scraper.arbeitnow_api.httpx.AsyncClient",
        side_effect=_status_factory({1: [_row()]}, calls, {2: [429, 429, 429]}),
    ):
        jobs = await scraper.scrape("golang", "germany")

    assert calls == [1, 2, 2, 2]
    assert len(jobs) == 1
