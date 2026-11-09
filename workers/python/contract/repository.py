"""Repository layer contract.

Mirrors apps/backend/internal/contract/repository/contract.go: aggregates
repository implementations behind a single container.
"""

from __future__ import annotations

from config.config import Config
from repository.getojobu_api import JobProfiles, NewGetojobuAPI


class RepositoryContract:
    def __init__(self, job_profiles: JobProfiles) -> None:
        self.job_profiles = job_profiles


def new(cfg: Config) -> RepositoryContract:
    """Build the repository contract from configuration.

    Creates a GetojobuAPI-backed JobProfiles repository.
    """
    job_profiles = NewGetojobuAPI(
        base_url=cfg.api.base_url,
        api_key=cfg.api.api_key,
    )
    return RepositoryContract(job_profiles=job_profiles)
