"""HTTP client for calling the Go API from the Python worker.

The worker does not access Neon directly. It sends extracted JobProfile
data to the Go API's internal endpoint over the Docker network, which
handles validation, authorization, and persistence.

Endpoints called:
  POST {base_url}/api/v1/internal/job-profiles

Auth:
  X-API-Key: {static worker key}

Idempotency:
  Idempotency-Key: {unique per evaluation attempt}
"""

from __future__ import annotations

import uuid
from typing import Any, Protocol, runtime_checkable

import httpx


@runtime_checkable
class APIClient(Protocol):
    """Protocol for the Go API client used by the analyze_job usecase."""

    async def create_job_profile(
        self,
        *,
        job_description: str,
        job_id: str | None,
        profile: dict[str, Any],
        idempotency_key: str | None = None,
    ) -> dict[str, Any]:
        """Persist a normalized job profile via the Go API.

        Returns the stored profile as a dict.
        """
        ...


class HTTPAPIClient:
    """httpx-backed implementation of APIClient."""

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
