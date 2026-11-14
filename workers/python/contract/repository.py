"""Repository layer contract.

Mirrors apps/backend/internal/contract/repository/contract.go: aggregates
repository implementations behind a single container.
"""

from __future__ import annotations

from config.config import Config, R2Config
from repository.getojobu_api import GetojobuAPI, NewGetojobuAPI
from repository.storage import R2StorageRepository, StorageRepository


class RepositoryContract:
    api: GetojobuAPI
    getojobu_api: GetojobuAPI
    storage: StorageRepository

    def __init__(
        self, api: GetojobuAPI, storage: StorageRepository | None = None
    ) -> None:
        self.api = api
        self.getojobu_api = api
        self.storage = storage or R2StorageRepository(R2Config())


def new(cfg: Config) -> RepositoryContract:
    """Build the repository contract from configuration.

    Creates a GetojobuAPI-backed client for the Go API and an R2 storage adapter.
    """
    api = NewGetojobuAPI(
        base_url=cfg.api.base_url,
        api_key=cfg.api.api_key,
    )
    storage = R2StorageRepository(cfg.r2)
    return RepositoryContract(api=api, storage=storage)
