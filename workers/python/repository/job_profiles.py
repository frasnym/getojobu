"""Repository interface and implementation for job_profiles.

Mirrors apps/backend/internal/repository/experiences.go: interface +
pgx-backed impl, parameterized SQL, explicit error wrapping.
"""

from __future__ import annotations

from typing import Optional, Protocol, runtime_checkable

from models.job_profile import JobProfile
from utils.database import Database


@runtime_checkable
class JobProfiles(Protocol):
    """Data access for normalized job profiles."""

    async def create(self, profile: JobProfile) -> JobProfile: ...
    async def find_by_job_id(self, job_id: str) -> Optional[JobProfile]: ...


class jobProfilesRepository:
    def __init__(self, db: Database) -> None:
        self._db = db

    async def create(self, profile: JobProfile) -> JobProfile:
        """Insert a normalized job profile. Returns the stored profile."""
        profile_data = profile.profile_data or {}
        sql = """
            INSERT INTO job_profiles (
                id, job_id, version, schema_version, family, title, company,
                location, work_arrangement, employment_type, seniority,
                minimum_years_experience, requirements, responsibilities,
                profile_data, prompt_version, provider, model, created_at
            )
            VALUES (
                $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
                $15, $16, $17, $18, $19
            )
            RETURNING id, job_id, version, schema_version, family, title,
                company, location, work_arrangement, employment_type,
                seniority, minimum_years_experience, requirements,
                responsibilities, profile_data, prompt_version, provider,
                model, created_at
        """
        row = await self._db.fetchrow(
            sql,
            str(profile.id),
            profile.job_id,
            profile.version,
            profile.schema_version,
            profile.family,
            profile.title,
            profile.company,
            profile.location,
            profile.work_arrangement,
            profile.employment_type,
            profile.seniority,
            profile.minimum_years_experience,
            [r.model_dump() for r in profile.requirements],
            profile.responsibilities,
            profile_data,
            profile.prompt_version,
            profile.provider,
            profile.model,
            profile.created_at,
        )
        if row is None:
            raise RuntimeError("create job profile: no row returned")
        return self._row_to_profile(row)

    async def find_by_job_id(self, job_id: str) -> Optional[JobProfile]:
        """Fetch a normalized job profile by its job reference."""
        sql = """
            SELECT id, job_id, version, schema_version, family, title,
                company, location, work_arrangement, employment_type,
                seniority, minimum_years_experience, requirements,
                responsibilities, profile_data, prompt_version, provider,
                model, created_at
            FROM job_profiles
            WHERE job_id = $1
            ORDER BY created_at DESC
            LIMIT 1
        """
        row = await self._db.fetchrow(sql, job_id)
        if row is None:
            return None
        return self._row_to_profile(row)

    @staticmethod
    def _row_to_profile(row: dict) -> JobProfile:
        requirements = [
            JobRequirement.model_validate(r) for r in (row.get("requirements") or [])
        ]
        return JobProfile(
            id=row["id"],
            job_id=row.get("job_id"),
            version=row.get("version", 1),
            schema_version=row.get("schema_version", "1.0"),
            family=row["family"],
            title=row.get("title"),
            company=row.get("company"),
            location=row.get("location"),
            work_arrangement=row.get("work_arrangement"),
            employment_type=row.get("employment_type"),
            seniority=row["seniority"],
            minimum_years_experience=row.get("minimum_years_experience"),
            requirements=requirements,
            responsibilities=row.get("responsibilities") or [],
            profile_data=row.get("profile_data") or {},
            prompt_version=row.get("prompt_version", "extract-job-v1"),
            provider=row.get("provider", "openai"),
            model=row.get("model", "gpt-4o-mini"),
            created_at=row.get("created_at"),
        )
