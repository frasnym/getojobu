"""Base interface for all job board scraper implementations."""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any


class ScraperRepository(ABC):
    """Abstract base repository for a job board scraper method."""

    name: str
    vendor: str

    @abstractmethod
    async def scrape(self, keyword: str, location: str) -> list[dict[str, Any]]:
        """Scrape jobs for a given keyword and location.

        Returns raw job dictionaries that can be processed by enrich_jobs.
        """
        raise NotImplementedError
