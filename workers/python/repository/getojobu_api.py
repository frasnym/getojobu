"""Repository for getting job data and updating status via the Go API.

The worker interacts with jobs by calling the Go API
over the internal Docker network.
"""

from __future__ import annotations

from typing import Any, Protocol, runtime_checkable

import httpx


@runtime_checkable
class GetojobuAPI(Protocol):
    """General data access client for the Go API (internal Docker network)."""

    async def get_job(self, slug: str) -> dict[str, Any]: ...

    async def patch_job_status(
        self,
        slug: str,
        status: str,
        *,
        analyze_result: str | None = None,
        failure_message: str | None = None,
    ) -> dict[str, Any]: ...

    async def aclose(self) -> None: ...


class _getojobuAPI:
    """httpx-backed implementation of GetojobuAPI via the Go API."""

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
                "Authorization": f"Bearer {self._api_key}",
                "Content-Type": "application/json",
            },
        )

    async def get_job(self, slug: str) -> dict[str, Any]:
        response = await self._client.get(f"/v1/jobs/{slug}")
        response.raise_for_status()
        raw = response.json()
        return self._unwrap_response(raw, operation=f"get job {slug!r}")

    async def patch_job_status(
        self,
        slug: str,
        status: str,
        *,
        analyze_result: str | None = None,
        failure_message: str | None = None,
    ) -> dict[str, Any]:
        payload: dict[str, Any] = {"status": status}
        if analyze_result is not None:
            payload["analyze_result"] = analyze_result
        if failure_message is not None:
            payload["failure_message"] = failure_message

        response = await self._client.patch(
            f"/v1/jobs/{slug}",
            json=payload,
        )
        response.raise_for_status()
        raw = response.json()
        return self._unwrap_response(raw, operation=f"patch job {slug!r} status")

    def _unwrap_response(self, raw: Any, *, operation: str) -> dict[str, Any]:
        if not isinstance(raw, dict):
            raise RuntimeError(f"{operation}: response is not a valid JSON object: {raw!r}")

        if "success" in raw:
            if not raw.get("success"):
                err = raw.get("error") or {}
                code = err.get("code", "UNKNOWN_ERROR")
                message = err.get("message", "API returned failure")
                raise RuntimeError(f"{operation} failed with API error ({code}): {message}")
            data = raw.get("data")
            if not isinstance(data, dict):
                raise RuntimeError(f"{operation}: 'data' is not an object: {data!r}")
            return data

        return raw

    async def aclose(self) -> None:
        await self._client.aclose()


def NewGetojobuAPI(
    *,
    base_url: str,
    api_key: str,
) -> GetojobuAPI:
    """Create a GetojobuAPI client backed by the Go API."""
    return _getojobuAPI(base_url=base_url, api_key=api_key)
