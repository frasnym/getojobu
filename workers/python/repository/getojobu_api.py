"""Repository for persisting job profiles via the Go API.

Mirrors apps/backend/internal/repository/experiences.go: interface +
implementation, parameterized requests, explicit error handling.

The worker does NOT access Neon directly. It persists JobProfile data by
calling the Go API over the internal Docker network.
"""

from __future__ import annotations

import json
import uuid
from typing import Any, Protocol, runtime_checkable

import httpx

from models.job_profile import JobProfile


@runtime_checkable
class JobProfiles(Protocol):
    """Data access for normalized job profiles via the Go API."""

    async def create_job_profile(
        self,
        *,
        job_description: str,
        job_id: str | None,
        profile: dict[str, Any],
        idempotency_key: str | None = None,
    ) -> dict[str, Any]: ...

    async def aclose(self) -> None: ...


class _getojobuAPIRepository:
    """httpx-backed implementation of JobProfiles via the Go API."""

    def __init__(
        self,
        *,
        base_url: str,
        api_key: str,
        timeout_seconds: float = 30.0,
    ) -> None:
        self._base_url = base_url.rstrip("/")
        self._api_key = api_key
        self._client = httpx.AsyncClient(
            base_url=self._base_url,
            timeout=httpx.Timeout(timeout_seconds),
            headers={
                "X-API-Key": self._api_key,
                "Content-Type": "application/json",
            },
        )

    async def create_job_profile(
        self,
        *,
        job_description: str,
        job_id: str | None,
        profile: dict[str, Any],
        idempotency_key: str | None = None,
    ) -> dict[str, Any]:
        key = idempotency_key or str(uuid.uuid4())
        headers = {"Idempotency-Key": key}

        payload: dict[str, Any] = {
            "job_description": job_description,
            "profile": profile,
        }
        if job_id is not None:
            payload["job_id"] = job_id

        response = await self._client.post(
            "/api/v1/internal/job-profiles",
            json=payload,
            headers=headers,
        )
        response.raise_for_status()
        data = response.json()
        return data.get("profile", data)

    async def aclose(self) -> None:
        await self._client.aclose()


def NewGetojobuAPI(
    *,
    base_url: str,
    api_key: str,
) -> JobProfiles:
    """Create a JobProfiles repository backed by the Go API.

    Mirrors NewExperiencesRepository in apps/backend/internal/repository.
    """
    return _getojobuAPIRepository(base_url=base_url, api_key=api_key)
