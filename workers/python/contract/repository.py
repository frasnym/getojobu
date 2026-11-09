"""Repository layer contract.

Mirrors apps/backend/internal/contract/repository/contract.go: aggregates
repository implementations behind a single container.
"""

from __future__ import annotations

from config.config import Config
from repository.getojobu_api import GetojobuAPI, NewGetojobuAPI


class RepositoryContract:
    def __init__(self, api: GetojobuAPI) -> None:
        self.api = api


def new(cfg: Config) -> RepositoryContract:
    """Build the repository contract from configuration.

    Creates a GetojobuAPI-backed client for the Go API.
    """
    api = NewGetojobuAPI(
        base_url=cfg.api.base_url,
        api_key=cfg.api.api_key,
    )
    return RepositoryContract(api=api)
