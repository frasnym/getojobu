"""Glassdoor scraper via Apify actor valig/glassdoor-jobs-scraper."""

from __future__ import annotations

import re
from datetime import UTC, datetime, timedelta
from typing import Any, cast

from repository.scraper.apify import ApifyScraper

DEFAULT_ACTOR = "valig~glassdoor-jobs-scraper"

_HTML_RE = re.compile(r"<[^>]+>")
_WS_RE = re.compile(r"\s+")


def strip_html(text: str | None) -> str:
    """Remove HTML tags, collapse whitespace, and clean up punctuation spacing."""
    if not text:
        return ""
    text = _HTML_RE.sub(" ", text)
    text = re.sub(r"\s+([.,!?:;])", r"\1", text)
    text = _WS_RE.sub(" ", text).strip()
    return text


class GlassdoorApifyValigScraper(ApifyScraper):
    """Scrapes Glassdoor jobs using valig/glassdoor-jobs-scraper."""

    def __init__(self, actor_id: str = DEFAULT_ACTOR) -> None:
        super().__init__(
            name="glassdoor-apify-valig",
            vendor="glassdoor",
            actor_id=actor_id,
        )

    def build_run_input(self, keyword: str, location: str) -> dict[str, Any]:
        return {
            "keywords": keyword,
            "location": location,
            "daysOld": 7,
            "limit": 20,
            "sortBy": "date_desc",
        }

    def parse_items(self, items: list[dict[str, Any]]) -> list[dict[str, Any]]:
        results: list[dict[str, Any]] = []
        now = datetime.now(UTC)

        for raw in items:
            title = (raw.get("title") or "").strip()

            employer_raw = raw.get("employer")
            if isinstance(employer_raw, dict):
                company = str(
                    cast(dict[str, Any], employer_raw).get("name") or ""
                ).strip()
            else:
                company = str(employer_raw or "").strip()

            loc_raw = raw.get("location")
            if isinstance(loc_raw, dict):
                location = str(
                    cast(dict[str, Any], loc_raw).get("name") or ""
                ).strip()
            else:
                location = str(loc_raw or "").strip()

            posted_date = ""
            age = raw.get("ageInDays")
            if isinstance(age, (int, float)):
                posted_date = (now - timedelta(days=int(age))).date().isoformat()

            desc = strip_html(raw.get("description") or "")
            source_url = raw.get("url") or raw.get("seoUrl") or ""
            apply_url = raw.get("applyUrl") or source_url
            is_remote = bool(raw.get("isRemote") or "remote" in location.lower())

            results.append(
                {
                    "title": title,
                    "company": company,
                    "location": location,
                    "posted_date": posted_date,
                    "description": desc,
                    "source_url": source_url,
                    "apply_url": apply_url,
                    "is_remote": is_remote,
                    "site": self.vendor,
                }
            )
        return results
