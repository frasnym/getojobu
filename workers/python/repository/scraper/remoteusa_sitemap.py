"""RemoteUSA scraper via sitemap discovery and JobPosting JSON-LD."""

from __future__ import annotations

import asyncio
import json
import re
from datetime import UTC, datetime, timedelta
from html.parser import HTMLParser
from typing import Any, cast
from urllib.parse import urlparse

import httpx

from repository.scraper.base import ScraperRepository
from repository.scraper.text import strip_html
from utils.logging import get_logger

logger = get_logger("scraper.remoteusa")

SITEMAP_URL = "https://remoteusa.co/sitemap.xml"
US_COUNTRY = "United States"
RECENT_DAYS = 2
MAX_JOBS = 50
REQUEST_PAUSE_SECONDS = 1.0
DESCRIPTION_MAX_CHARS = 16000

_URL_BLOCK_RE = re.compile(r"<url>(.*?)</url>", re.DOTALL)
_LOC_RE = re.compile(r"<loc>\s*([^<]+?)\s*</loc>")
_LASTMOD_RE = re.compile(r"<lastmod>\s*([^<]+?)\s*</lastmod>")


class _JsonLdCollector(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.blocks: list[str] = []
        self._collecting = False
        self._buffer: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if tag == "script" and dict(attrs).get("type") == "application/ld+json":
            self._collecting = True
            self._buffer = []

    def handle_data(self, data: str) -> None:
        if self._collecting:
            self._buffer.append(data)

    def handle_endtag(self, tag: str) -> None:
        if tag == "script" and self._collecting:
            self._collecting = False
            raw = "".join(self._buffer).strip()
            if raw:
                self.blocks.append(raw)


def sitemap_entries(xml: str) -> list[tuple[str, datetime]]:
    """Return (job_url, lastmod) pairs from the sitemap's job entries."""
    entries: list[tuple[str, datetime]] = []
    for block in _URL_BLOCK_RE.findall(xml):
        loc = _LOC_RE.search(block)
        lastmod = _LASTMOD_RE.search(block)
        if loc is None or lastmod is None:
            continue
        url = loc.group(1).strip()
        if urlparse(url).netloc != "remoteusa.co" or "/jobs/" not in urlparse(url).path:
            continue
        try:
            modified = datetime.fromisoformat(lastmod.group(1).strip())
        except ValueError:
            continue
        if modified.tzinfo is None:
            modified = modified.replace(tzinfo=UTC)
        entries.append((url, modified))
    return entries


def job_postings(html: str) -> list[dict[str, Any]]:
    """Extract JobPosting JSON-LD objects from a job page."""
    collector = _JsonLdCollector()
    collector.feed(html)
    postings: list[dict[str, Any]] = []
    for raw in collector.blocks:
        try:
            data = cast(object, json.loads(raw))
        except ValueError:
            continue
        items = data if isinstance(data, list) else [data]
        for item in items:
            if isinstance(item, dict) and item.get("@type") == "JobPosting":
                postings.append(cast(dict[str, Any], item))
    return postings


def matches_keyword(text: str, keyword: str) -> bool:
    tokens = [t for t in re.split(r"\s+", keyword.strip().lower()) if t]
    if not tokens:
        return False
    haystack = text.lower()
    return all(re.search(rf"\b{re.escape(t)}\b", haystack) for t in tokens)


class RemoteUsaSitemapScraper(ScraperRepository):
    """Scrapes RemoteUSA by walking the sitemap and reading JobPosting JSON-LD."""

    def __init__(self) -> None:
        self.name = "remoteusa-sitemap"
        self.vendor = "remoteusa"

    async def scrape(self, keyword: str, location: str) -> list[dict[str, Any]]:
        async with httpx.AsyncClient(timeout=30.0) as client:
            response = await client.get(SITEMAP_URL)
            response.raise_for_status()

            cutoff = datetime.now(UTC) - timedelta(days=RECENT_DAYS)
            recent = [e for e in sitemap_entries(response.text) if e[1] >= cutoff]
            recent.sort(key=lambda e: e[1], reverse=True)

            jobs: list[dict[str, Any]] = []
            for url, modified in recent[:MAX_JOBS]:
                page = await client.get(url)
                if page.status_code != 200:
                    logger.warning(
                        "remoteusa_page_failed", url=url, status=page.status_code
                    )
                    await asyncio.sleep(REQUEST_PAUSE_SECONDS)
                    continue

                for posting in job_postings(page.text):
                    job = self._map_posting(posting, url, modified, keyword)
                    if job is not None:
                        jobs.append(job)

                await asyncio.sleep(REQUEST_PAUSE_SECONDS)

            return jobs

    def _map_posting(
        self,
        posting: dict[str, Any],
        fallback_url: str,
        modified: datetime,
        keyword: str,
    ) -> dict[str, Any] | None:
        applicant = posting.get("applicantLocationRequirements")
        country = applicant.get("name") if isinstance(applicant, dict) else None
        if country != US_COUNTRY:
            return None

        title = str(posting.get("title") or "").strip()
        if not title or not matches_keyword(title, keyword):
            return None

        org = posting.get("hiringOrganization")
        company = org.get("name") if isinstance(org, dict) else None
        if not isinstance(company, str) or not company.strip():
            return None

        description = strip_html(str(posting.get("description") or ""))[
            :DESCRIPTION_MAX_CHARS
        ]
        job_url = str(posting.get("url") or fallback_url).strip()

        return {
            "title": title,
            "company": company.strip(),
            "location": US_COUNTRY,
            "description": description,
            "date_posted": modified,
            "is_remote": True,
            "job_url": job_url,
            "apply_url": job_url,
            "site": self.vendor,
        }
