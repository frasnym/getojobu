"""LinkedIn scraper via Apify actor curious_coder/linkedin-jobs-scraper."""

from __future__ import annotations

import urllib.parse
from typing import Any

from repository.scraper.apify import ApifyScraper

DEFAULT_ACTOR = "curious_coder~linkedin-jobs-scraper"


class LinkedinApifyCuriousCoderScraper(ApifyScraper):
    """Scrapes LinkedIn jobs using curious_coder/linkedin-jobs-scraper."""

    def __init__(self, actor_id: str = DEFAULT_ACTOR) -> None:
        super().__init__(
            name="linkedin-apify-curious_coder",
            vendor="linkedin",
            actor_id=actor_id,
        )

    def build_run_input(self, keyword: str, location: str) -> dict[str, Any]:
        # f_TPR=r86400 restricts to the last 24 hours
        q = urllib.parse.urlencode(
            {"keywords": keyword, "location": location, "f_TPR": "r86400"}
        )
        url = f"https://www.linkedin.com/jobs/search/?{q}"
        return {
            "urls": [url],
            "limitPerSource": 20,
            "scrapeCompany": False,
            "autoConvertToAiSearch": True,
        }

    def parse_items(self, items: list[dict[str, Any]]) -> list[dict[str, Any]]:
        results: list[dict[str, Any]] = []
        for raw in items:
            title = (raw.get("title") or "").strip()
            company = (raw.get("companyName") or raw.get("company") or "").strip()
            location = (raw.get("location") or "").strip()
            source_url = raw.get("link") or raw.get("url") or ""
            apply_url = raw.get("applyUrl") or source_url
            posted_date = (raw.get("postedAt") or "")[:10]
            description = (
                raw.get("descriptionText") or raw.get("description") or ""
            ).strip()
            is_remote = bool(raw.get("isRemote") or "remote" in location.lower())

            results.append(
                {
                    "title": title,
                    "company": company,
                    "location": location,
                    "posted_date": posted_date,
                    "description": description,
                    "source_url": source_url,
                    "apply_url": apply_url,
                    "is_remote": is_remote,
                    "site": self.vendor,
                }
            )
        return results
