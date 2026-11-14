"""Glassdoor scraper via JobSpy."""

from __future__ import annotations

import asyncio
from typing import Any, cast

import jobspy

from repository.scraper.base import ScraperRepository


class GlassdoorJobspyScraper(ScraperRepository):
    """Scrapes Glassdoor jobs using python-jobspy."""

    def __init__(self) -> None:
        self.name = "glassdoor-jobspy"
        self.vendor = "glassdoor"

    async def scrape(self, keyword: str, location: str) -> list[dict[str, Any]]:
        return await asyncio.to_thread(self._scrape_sync, keyword, location)

    def _scrape_sync(self, keyword: str, location: str) -> list[dict[str, Any]]:
        result = jobspy.scrape_jobs(
            site_name=self.vendor,
            search_term=keyword,
            location=location,
            results_wanted=20,
            hours_old=24,
            description_format="plain",
            fetch_description=True,
        )

        records: list[dict[str, Any]] = []
        if hasattr(result, "to_dict"):
            records = cast(list[dict[str, Any]], result.to_dict("records"))
        elif isinstance(result, list):
            records = cast(list[dict[str, Any]], result)

        sanitized: list[dict[str, Any]] = []
        for r in records:
            item = dict(r)
            if not item.get("site"):
                item["site"] = self.vendor
            sanitized.append(item)

        return sanitized
