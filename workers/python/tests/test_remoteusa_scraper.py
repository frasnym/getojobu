"""Unit tests for the RemoteUSA sitemap scraper."""

import json
from datetime import UTC, datetime, timedelta
from unittest.mock import AsyncMock, patch

import httpx
import pytest

from repository.scraper.remoteusa_sitemap import RemoteUsaSitemapScraper

BASE = "https://remoteusa.co"


@pytest.fixture(autouse=True)
def _no_sleep():
    with patch("repository.scraper.remoteusa_sitemap.asyncio.sleep", new=AsyncMock()):
        yield


def _recent() -> datetime:
    return (datetime.now(UTC) - timedelta(hours=2)).replace(microsecond=0)


def _sitemap(entries: list[tuple[str, datetime]]) -> str:
    urls = "".join(
        f"<url><loc>{BASE}/jobs/{slug}</loc>"
        f"<lastmod>{lastmod.isoformat()}</lastmod></url>"
        for slug, lastmod in entries
    )
    return f'<?xml version="1.0"?><urlset>{urls}</urlset>'


def _job_page(
    *,
    slug: str = "software-engineer-abc",
    title: str = "Software Engineer",
    company: str = "Acme",
    country: str = "United States",
    description: str = "<p>Build Go systems</p>",
) -> str:
    posting = {
        "@context": "https://schema.org/",
        "@type": "JobPosting",
        "title": title,
        "description": description,
        "url": f"{BASE}/jobs/{slug}",
        "datePosted": "2026-11-19",
        "hiringOrganization": {"@type": "Organization", "name": company},
        "jobLocationType": "TELECOMMUTE",
        "applicantLocationRequirements": {"@type": "Country", "name": country},
    }
    return (
        '<html><body><script type="application/ld+json">'
        f"{json.dumps(posting)}"
        "</script></body></html>"
    )


def _client_factory(pages: dict[str, str], fetched: list[str]):
    real_async_client = httpx.AsyncClient

    def make_client(*args, **kwargs):
        def handler(request: httpx.Request):
            path = request.url.path
            fetched.append(path)
            return httpx.Response(200, text=pages.get(path, ""))

        kwargs["transport"] = httpx.MockTransport(handler)
        return real_async_client(*args, **kwargs)

    return make_client


@pytest.mark.asyncio
async def test_scrape_maps_job_posting_to_enrich_shape():
    recent = _recent()
    fetched: list[str] = []
    pages = {
        "/sitemap.xml": _sitemap([("software-engineer-abc", recent)]),
        "/jobs/software-engineer-abc": _job_page(),
    }

    with patch(
        "repository.scraper.remoteusa_sitemap.httpx.AsyncClient",
        side_effect=_client_factory(pages, fetched),
    ):
        jobs = await RemoteUsaSitemapScraper().scrape(
            "software engineer", "united states"
        )

    assert len(jobs) == 1
    job = jobs[0]
    assert job["title"] == "Software Engineer"
    assert job["company"] == "Acme"
    assert job["location"] == "United States"
    assert job["description"] == "Build Go systems"
    assert job["is_remote"] is True
    assert job["site"] == "remoteusa"
    assert job["job_url"] == f"{BASE}/jobs/software-engineer-abc"
    assert job["apply_url"] == job["job_url"]


@pytest.mark.asyncio
async def test_scrape_uses_sitemap_lastmod_as_posted_date():
    recent = _recent()
    fetched: list[str] = []
    pages = {
        "/sitemap.xml": _sitemap([("software-engineer-abc", recent)]),
        "/jobs/software-engineer-abc": _job_page(),
    }

    with patch(
        "repository.scraper.remoteusa_sitemap.httpx.AsyncClient",
        side_effect=_client_factory(pages, fetched),
    ):
        jobs = await RemoteUsaSitemapScraper().scrape("engineer", "united states")

    assert jobs[0]["date_posted"] == recent


@pytest.mark.asyncio
async def test_scrape_skips_stale_sitemap_entries():
    stale = datetime.now(UTC) - timedelta(days=10)
    fetched: list[str] = []
    pages = {
        "/sitemap.xml": _sitemap(
            [
                ("software-engineer-abc", _recent()),
                ("old-job-xyz", stale),
            ]
        ),
        "/jobs/software-engineer-abc": _job_page(),
        "/jobs/old-job-xyz": _job_page(
            slug="old-job-xyz", title="Old Engineer", company="Old Corp"
        ),
    }

    with patch(
        "repository.scraper.remoteusa_sitemap.httpx.AsyncClient",
        side_effect=_client_factory(pages, fetched),
    ):
        jobs = await RemoteUsaSitemapScraper().scrape("engineer", "united states")

    assert "/jobs/old-job-xyz" not in fetched
    assert [j["title"] for j in jobs] == ["Software Engineer"]


@pytest.mark.asyncio
async def test_scrape_skips_non_us_job_postings():
    fetched: list[str] = []
    pages = {
        "/sitemap.xml": _sitemap([("canada-job-abc", _recent())]),
        "/jobs/canada-job-abc": _job_page(slug="canada-job-abc", country="Canada"),
    }

    with patch(
        "repository.scraper.remoteusa_sitemap.httpx.AsyncClient",
        side_effect=_client_factory(pages, fetched),
    ):
        jobs = await RemoteUsaSitemapScraper().scrape("engineer", "united states")

    assert jobs == []


@pytest.mark.asyncio
async def test_scrape_filters_by_keyword_in_title():
    fetched: list[str] = []
    pages = {
        "/sitemap.xml": _sitemap(
            [
                ("software-engineer-abc", _recent()),
                ("data-engineer-xyz", _recent()),
            ]
        ),
        "/jobs/software-engineer-abc": _job_page(),
        "/jobs/data-engineer-xyz": _job_page(
            slug="data-engineer-xyz", title="Data Engineer", company="Data Corp"
        ),
    }

    with patch(
        "repository.scraper.remoteusa_sitemap.httpx.AsyncClient",
        side_effect=_client_factory(pages, fetched),
    ):
        jobs = await RemoteUsaSitemapScraper().scrape(
            "software engineer", "united states"
        )

    assert [j["title"] for j in jobs] == ["Software Engineer"]


@pytest.mark.asyncio
async def test_scrape_skips_pages_without_job_posting():
    fetched: list[str] = []
    pages = {
        "/sitemap.xml": _sitemap([("no-data-abc", _recent())]),
        "/jobs/no-data-abc": "<html><body>No structured data</body></html>",
    }

    with patch(
        "repository.scraper.remoteusa_sitemap.httpx.AsyncClient",
        side_effect=_client_factory(pages, fetched),
    ):
        jobs = await RemoteUsaSitemapScraper().scrape("engineer", "united states")

    assert jobs == []


@pytest.mark.asyncio
async def test_scrape_does_not_depend_on_location_argument():
    fetched: list[str] = []
    pages = {
        "/sitemap.xml": _sitemap([("software-engineer-abc", _recent())]),
        "/jobs/software-engineer-abc": _job_page(),
    }

    with patch(
        "repository.scraper.remoteusa_sitemap.httpx.AsyncClient",
        side_effect=_client_factory(pages, fetched),
    ):
        jobs = await RemoteUsaSitemapScraper().scrape("software engineer", "indonesia")

    assert len(jobs) == 1
    assert jobs[0]["location"] == "United States"
