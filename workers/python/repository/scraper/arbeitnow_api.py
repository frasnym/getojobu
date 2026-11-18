"""Arbeitnow scraper via the documented public job-board feed."""

from __future__ import annotations

import asyncio
import html
import re
import time
from datetime import UTC, datetime, timedelta
from typing import cast, final
from urllib.parse import urlparse

import httpx

from repository.scraper.base import ScraperRepository
from repository.scraper.text import strip_html
from utils.logging import get_logger

logger = get_logger("scraper.arbeitnow")

FEED_URL = "https://www.arbeitnow.com/api/job-board-api"
MAX_PAGES = 33
PAGE_TTL_SECONDS = 600.0
RECENT_DAYS = 1
REQUEST_PAUSE_SECONDS = 1.5
MAX_ATTEMPTS_PER_PAGE = 3
RETRY_BACKOFF_SECONDS = 15.0
DESCRIPTION_MAX_CHARS = 16000

# User location (normalized) -> board host. The feed merges every country site,
# so the host of a job URL is the only country signal available.
HOSTS_BY_LOCATION: dict[str, str] = {
    "germany": "www.arbeitnow.com",
    "united kingdom": "www.arbeitnow.co.uk",
    "france": "www.arbeitnow.fr",
    "netherlands": "www.arbeitnow.nl",
    "switzerland": "www.arbeitnow.ch",
}


def matches_keyword(text: str, keyword: str) -> bool:
    """Token-AND: every keyword token must appear as a whole word in the text."""
    tokens = [t for t in re.split(r"\s+", keyword.strip().lower()) if t]
    if not tokens:
        return False
    haystack = text.lower()
    return all(re.search(rf"\b{re.escape(t)}\b", haystack) for t in tokens)


def plain_text(raw: object) -> str:
    """HTML-escaped feed description -> single-line plain text, capped for prompts."""
    if not isinstance(raw, str) or not raw:
        return ""
    return strip_html(html.unescape(raw))[:DESCRIPTION_MAX_CHARS]


@final
class ArbeitnowApiScraper(ScraperRepository):
    """Scrapes Arbeitnow through its documented public job-board feed."""

    def __init__(self) -> None:
        self.name: str = "arbeitnow-api"
        self.vendor: str = "arbeitnow"
        self._pages: dict[int, tuple[float, list[dict[str, object]]]] = {}

    async def scrape(self, keyword: str, location: str) -> list[dict[str, object]]:
        host = HOSTS_BY_LOCATION.get(location.strip().lower())
        if not host:
            return []

        rows = await self._feed()
        cutoff = datetime.now(UTC) - timedelta(days=RECENT_DAYS)
        seen: set[str] = set()
        jobs: list[dict[str, object]] = []

        for row in rows:
            slug = row.get("slug")
            if not isinstance(slug, str) or not slug or slug in seen:
                continue

            created = row.get("created_at")
            if not isinstance(created, int):
                continue
            posted = datetime.fromtimestamp(created, UTC)
            if posted < cutoff:
                continue

            url = str(row.get("url") or "")
            if urlparse(url).netloc != host:
                continue

            title = str(row.get("title") or "").strip()
            company = str(row.get("company_name") or "").strip()
            if not title or not company:
                continue

            raw_tags = row.get("tags")
            tags: list[str] = (
                [t for t in cast(list[object], raw_tags) if isinstance(t, str)]
                if isinstance(raw_tags, list)
                else []
            )
            searchable = f"{title} {' '.join(tags)}" if tags else title
            if not matches_keyword(searchable, keyword):
                continue

            is_remote = row.get("remote") is True
            job_location = str(row.get("location") or "").strip()
            if not job_location:
                if not is_remote:
                    continue
                job_location = "Remote"

            seen.add(slug)
            jobs.append(
                {
                    "title": title,
                    "company": company,
                    "location": job_location,
                    "description": plain_text(row.get("description")),
                    "date_posted": posted,
                    "is_remote": is_remote,
                    "job_url": url,
                    "apply_url": url,
                    "site": self.vendor,
                }
            )

        return jobs

    async def _feed(self) -> list[dict[str, object]]:
        """Return the whole feed, reusing a short-lived in-process page cache.

        The feed's ordering is not stable (pages overlap and invert), so the walk
        always starts at page 1 and stops only on an empty page. The walk is
        paced, and a page that keeps being rate limited ends the walk with the
        pages fetched so far rather than failing the whole scan.
        """
        started = time.monotonic()
        cached = self._pages.get(1)
        if cached and started - cached[0] < PAGE_TTL_SECONDS:
            cached_rows: list[dict[str, object]] = []
            for page in range(1, MAX_PAGES + 1):
                entry = self._pages.get(page)
                if entry is None:
                    break
                cached_rows.extend(entry[1])
            return cached_rows

        self._pages.clear()
        rows: list[dict[str, object]] = []
        async with httpx.AsyncClient(timeout=30.0) as client:
            for page in range(1, MAX_PAGES + 1):
                page_rows = await self._fetch_page(client, page)
                if page_rows is None:
                    logger.warning(
                        "arbeitnow_feed_truncated",
                        pages_fetched=len(self._pages),
                        rows=len(rows),
                    )
                    break
                if not page_rows:
                    break
                self._pages[page] = (started, page_rows)
                rows.extend(page_rows)
                await asyncio.sleep(REQUEST_PAUSE_SECONDS)

        return rows

    async def _fetch_page(
        self, client: httpx.AsyncClient, page: int
    ) -> list[dict[str, object]] | None:
        """Fetch one page; return None when the page stays rate limited."""
        for attempt in range(1, MAX_ATTEMPTS_PER_PAGE + 1):
            resp = await client.get(FEED_URL, params={"page": page})
            if resp.status_code != 429:
                _ = resp.raise_for_status()
                raw_json = cast(object, resp.json())
                if isinstance(raw_json, dict):
                    payload = cast(dict[str, object], raw_json)
                    raw_data: object = payload.get("data")
                    if isinstance(raw_data, list):
                        data_list = cast(list[object], raw_data)
                        return [
                            cast(dict[str, object], item)
                            for item in data_list
                            if isinstance(item, dict)
                        ]
                return []

            if attempt == MAX_ATTEMPTS_PER_PAGE:
                return None

            delay = RETRY_BACKOFF_SECONDS * attempt
            if "retry-after" in resp.headers:
                retry_header = resp.headers["retry-after"].strip()
                if retry_header.isdigit():
                    delay = float(retry_header)
            logger.warning(
                "arbeitnow_rate_limited",
                page=page,
                attempt=attempt,
                retry_in_seconds=delay,
            )
            await asyncio.sleep(delay)

        return None
