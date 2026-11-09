"""Repository layer contract.

Mirrors apps/backend/internal/contract/repository/contract.go: aggregates
repository implementations behind a single container.
"""

from __future__ import annotations

from repository.getojobu_api import JobProfiles, NewGetojobuAPI


class Contract:
    def __init__(self, api_client: JobProfiles) -> None:
        self.api_client = api_client


def new(cfg) -> Contract:
    """Build the repository contract from configuration.

    Creates a GetojobuAPI-backed JobProfiles repository.
    """
    api_client = NewGetojobuAPI(
        base_url=cfg.api.base_url,
        api_key=cfg.api.api_key,
    )
    return Contract(api_client=api_client)
