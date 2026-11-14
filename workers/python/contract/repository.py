"""Repository layer contract.

Mirrors apps/backend/internal/contract/repository/contract.go: aggregates
repository implementations behind a single container.
"""

from __future__ import annotations

from config.config import Config, R2Config
from repository.getojobu_api import GetojobuAPI, NewGetojobuAPI
from repository.scraper.base import ScraperRepository
from repository.scraper.glassdoor_apify_valig import GlassdoorApifyValigScraper
from repository.scraper.glassdoor_jobspy import GlassdoorJobspyScraper
from repository.scraper.linkedin_apify_curious_coder import (
    LinkedinApifyCuriousCoderScraper,
)
from repository.storage import R2StorageRepository, StorageRepository


class RepositoryContract:
    api: GetojobuAPI
    getojobu_api: GetojobuAPI
    storage: StorageRepository
    scrapers: dict[str, ScraperRepository]

    def __init__(
        self,
        api: GetojobuAPI,
        storage: StorageRepository | None = None,
        scrapers: dict[str, ScraperRepository] | None = None,
    ) -> None:
        self.api = api
        self.getojobu_api = api
        self.storage = storage or R2StorageRepository(R2Config())
        self.scrapers = scrapers or {}


def new(cfg: Config) -> RepositoryContract:
    """Build the repository contract from configuration.

    Creates a GetojobuAPI-backed client for the Go API, an R2 storage adapter,
    and registered scraper methods.
    """
    api = NewGetojobuAPI(
        base_url=cfg.api.base_url,
        api_key=cfg.api.api_key,
    )
    storage = R2StorageRepository(cfg.r2)
    scrapers: dict[str, ScraperRepository] = {
        "linkedin-apify-curious_coder": LinkedinApifyCuriousCoderScraper(),
        "glassdoor-apify-valig": GlassdoorApifyValigScraper(),
        "glassdoor-jobspy": GlassdoorJobspyScraper(),
    }
    return RepositoryContract(api=api, storage=storage, scrapers=scrapers)
