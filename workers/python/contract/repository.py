"""Repository layer contract.

Mirrors apps/backend/internal/contract/repository/contract.go: aggregates
repository implementations behind a single container.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

from repository.job_profiles import JobProfiles, jobProfilesRepository

if TYPE_CHECKING:
    from contract.common import Contract as CommonContract


class Contract:
    def __init__(self, job_profiles: JobProfiles) -> None:
        self.job_profiles = job_profiles


def new(common: "CommonContract") -> Contract:
    """Build the repository contract from the common infrastructure contract."""
    return Contract(job_profiles=jobProfilesRepository(db=common.db))
