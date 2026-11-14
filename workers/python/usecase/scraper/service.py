"""Scraper resolution, validation, and serialized execution service."""

from __future__ import annotations

import asyncio
from typing import Any

from repository.scraper.base import ScraperRepository
from utils.logging import get_logger

logger = get_logger("usecase.scraper")


class ScraperMethodNotConfiguredError(ValueError):
    """Raised when a configured scraper method is unknown or invalid for the vendor."""


class ScraperService:
    """Manages active scraper methods per vendor and serializes actor execution."""

    def __init__(
        self,
        configured_methods: dict[str, str],
        scrapers: dict[str, ScraperRepository],
    ) -> None:
        self.configured_methods = {
            k.strip().lower(): v.strip() for k, v in configured_methods.items()
        }
        self.scrapers = scrapers
        self._semaphore = asyncio.Semaphore(1)

    def validate_configured_methods(self) -> None:
        """Validate that any enabled configured method is registered and matches its vendor."""
        for vendor, method_name in self.configured_methods.items():
            if not method_name:
                continue
            if method_name not in self.scrapers:
                raise ScraperMethodNotConfiguredError(
                    f"method '{method_name}' not registered for vendor '{vendor}'"
                )
            scraper = self.scrapers[method_name]
            if scraper.vendor.lower() != vendor:
                raise ScraperMethodNotConfiguredError(
                    f"method '{method_name}' (vendor '{scraper.vendor}') does not match requested vendor '{vendor}'"
                )

    def resolve(self, vendor: str) -> ScraperRepository | None:
        """Resolve the single active scraper method for the vendor.

        Returns None if vendor has no active method configured (disabled).
        Raises ScraperMethodNotConfiguredError if configured method is unknown
        or belongs to a different vendor.
        """
        v = vendor.strip().lower()
        method_name = self.configured_methods.get(v)
        if not method_name:
            return None

        if method_name not in self.scrapers:
            raise ScraperMethodNotConfiguredError(
                f"method '{method_name}' not registered for vendor '{v}'"
            )

        scraper = self.scrapers[method_name]
        if scraper.vendor.lower() != v:
            raise ScraperMethodNotConfiguredError(
                f"method '{method_name}' (vendor '{scraper.vendor}') does not match requested vendor '{v}'"
            )

        return scraper

    async def scrape(
        self, vendor: str, keyword: str, location: str
    ) -> list[dict[str, Any]]:
        """Resolve and execute the scraper for the vendor, serializing execution."""
        scraper = self.resolve(vendor)
        if scraper is None:
            logger.info("scraper_vendor_disabled", vendor=vendor)
            return []

        async with self._semaphore:
            logger.info(
                "scraper_executing",
                vendor=vendor,
                method=scraper.name,
                keyword=keyword,
                location=location,
            )
            return await scraper.scrape(keyword, location)
