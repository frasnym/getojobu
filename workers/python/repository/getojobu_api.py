"""Repository for getting job data and updating status via the Go API.

The worker interacts with jobs by calling the Go API
over the internal Docker network.
"""

from __future__ import annotations

from typing import Any, Protocol, cast, runtime_checkable

import httpx

from utils.logging import get_logger

logger = get_logger("repository.getojobu_api")


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

    async def get_active_scoring_config(self, family: str) -> dict[str, Any]: ...

    async def patch_matching_evaluation(
        self,
        evaluation_id: str,
        *,
        evaluation_status: str | None = None,
        status: str | None = None,
        evaluation_data: dict[str, Any] | None = None,
        failure_message: str | None = None,
        provider: str | None = None,
        model: str | None = None,
        **extra: Any,
    ) -> dict[str, Any]: ...

    async def patch_matching_evaluation_report(
        self,
        evaluation_id: str,
        *,
        report_status: str,
        report_data: dict[str, Any] | None = None,
        report_failure_message: str | None = None,
    ) -> dict[str, Any]: ...

    async def patch_matching_evaluation_resume(
        self,
        evaluation_id: str,
        *,
        resume_status: str,
        resume_failure_message: str | None = None,
        resume_pdf_key: str | None = None,
        resume_json_key: str | None = None,
        resume_artifacts: dict[str, Any] | None = None,
    ) -> dict[str, Any]: ...

    async def bulk_generate_resume(
        self, items: list[dict[str, Any]]
    ) -> list[dict[str, Any]]: ...

    async def get_matching_evaluation_report(
        self, evaluation_id: str
    ) -> dict[str, Any]: ...

    async def trigger_bulk_analyze_compatibility(
        self, items: list[dict[str, Any]]
    ) -> list[dict[str, Any]]: ...

    async def post(self, path: str, json: Any) -> Any: ...

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

    def _check_response(self, response: httpx.Response, operation: str) -> None:
        if response.is_error:
            logger.error(
                "API request failed",
                operation=operation,
                status_code=response.status_code,
                response_text=response.text,
            )
        response.raise_for_status()

    async def get_job(self, slug: str) -> dict[str, Any]:
        response = await self._client.get(f"/v1/jobs/{slug}")
        self._check_response(response, operation=f"get job {slug!r}")
        raw = response.json()
        unwrapped = self._unwrap_response(raw, operation=f"get job {slug!r}")
        if not isinstance(unwrapped, dict):
            raise TypeError(f"get job {slug!r}: expected dict, got {type(unwrapped)}")
        return unwrapped

    async def patch_matching_evaluation_resume(
        self,
        evaluation_id: str,
        *,
        resume_status: str,
        resume_failure_message: str | None = None,
        resume_pdf_key: str | None = None,
        resume_json_key: str | None = None,
        resume_artifacts: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        payload: dict[str, Any] = {"resume_status": resume_status}
        if resume_failure_message is not None:
            payload["resume_failure_message"] = resume_failure_message
        if resume_pdf_key is not None:
            payload["resume_pdf_key"] = resume_pdf_key
        elif resume_artifacts and "pdf_key" in resume_artifacts:
            payload["resume_pdf_key"] = resume_artifacts["pdf_key"]
        if resume_json_key is not None:
            payload["resume_json_key"] = resume_json_key
        elif resume_artifacts and "content_key" in resume_artifacts:
            payload["resume_json_key"] = resume_artifacts["content_key"]
        if resume_artifacts is not None:
            payload["resume_artifacts"] = resume_artifacts
        response = await self._client.patch(
            f"/v1/matching-evaluations/{evaluation_id}/resume", json=payload
        )
        self._check_response(
            response, operation=f"patch matching evaluation resume {evaluation_id!r}"
        )
        raw = response.json()
        unwrapped = self._unwrap_response(
            raw, operation=f"patch matching evaluation resume {evaluation_id!r}"
        )
        if not isinstance(unwrapped, dict):
            raise TypeError(
                f"patch matching evaluation resume {evaluation_id!r}: expected dict"
            )
        return unwrapped

    async def bulk_generate_resume(
        self, items: list[dict[str, Any]]
    ) -> list[dict[str, Any]]:
        response = await self._client.post(
            "/v1/matching-evaluations/bulk-generate-resume",
            json=items,
        )
        self._check_response(response, operation="bulk generate resume")
        raw = response.json()
        unwrapped = self._unwrap_response(raw, operation="bulk generate resume")
        if isinstance(unwrapped, list):
            return cast("list[dict[str, Any]]", unwrapped)
        return []

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
        self._check_response(response, operation=f"patch job {slug!r} status")
        raw = response.json()
        unwrapped = self._unwrap_response(raw, operation=f"patch job {slug!r} status")
        if not isinstance(unwrapped, dict):
            raise TypeError(
                f"patch job {slug!r} status: expected dict, got {type(unwrapped)}"
            )
        return unwrapped

    async def get_active_scoring_config(self, family: str) -> dict[str, Any]:
        response = await self._client.get(
            "/v1/scoring-configs/active",
            params={"family": family},
        )
        self._check_response(
            response, operation=f"get active scoring config for {family!r}"
        )
        raw = response.json()
        unwrapped = self._unwrap_response(
            raw, operation=f"get active scoring config for {family!r}"
        )
        if not isinstance(unwrapped, dict):
            raise TypeError(
                f"get active scoring config for {family!r}: expected dict, got {type(unwrapped)}"
            )
        return unwrapped

    async def patch_matching_evaluation(
        self,
        evaluation_id: str,
        *,
        evaluation_status: str | None = None,
        status: str | None = None,
        evaluation_data: dict[str, Any] | None = None,
        failure_message: str | None = None,
        provider: str | None = None,
        model: str | None = None,
        **extra: Any,
    ) -> dict[str, Any]:
        eval_status = evaluation_status or status or ""
        payload: dict[str, Any] = {"evaluation_status": eval_status}
        if evaluation_data is not None:
            eval_copy = dict(evaluation_data)
            if provider and "provider" not in eval_copy:
                eval_copy["provider"] = provider
            if model and "model" not in eval_copy:
                eval_copy["model"] = model
            payload["evaluation_data"] = eval_copy
        if failure_message is not None:
            payload["failure_message"] = failure_message

        response = await self._client.patch(
            f"/v1/matching-evaluations/{evaluation_id}",
            json=payload,
        )
        self._check_response(
            response, operation=f"patch matching evaluation {evaluation_id!r}"
        )
        raw = response.json()
        unwrapped = self._unwrap_response(
            raw, operation=f"patch matching evaluation {evaluation_id!r}"
        )
        if not isinstance(unwrapped, dict):
            raise TypeError(
                f"patch matching evaluation {evaluation_id!r}: expected dict, got {type(unwrapped)}"
            )
        return unwrapped

    async def patch_matching_evaluation_report(
        self,
        evaluation_id: str,
        *,
        report_status: str,
        report_data: dict[str, Any] | None = None,
        report_failure_message: str | None = None,
    ) -> dict[str, Any]:
        payload: dict[str, Any] = {"report_status": report_status}
        if report_data is not None:
            payload["report_data"] = report_data
        if report_failure_message is not None:
            payload["report_failure_message"] = report_failure_message

        response = await self._client.patch(
            f"/v1/matching-evaluations/{evaluation_id}/report",
            json=payload,
        )
        self._check_response(
            response, operation=f"patch matching evaluation report {evaluation_id!r}"
        )
        raw = response.json()
        unwrapped = self._unwrap_response(
            raw, operation=f"patch matching evaluation report {evaluation_id!r}"
        )
        if not isinstance(unwrapped, dict):
            raise TypeError(
                f"patch matching evaluation report {evaluation_id!r}: expected dict, got {type(unwrapped)}"
            )
        return unwrapped

    async def get_matching_evaluation_report(
        self, evaluation_id: str
    ) -> dict[str, Any]:
        response = await self._client.get(
            f"/v1/matching-evaluations/{evaluation_id}/report"
        )
        self._check_response(
            response, operation=f"get matching evaluation report {evaluation_id!r}"
        )
        raw = response.json()
        unwrapped = self._unwrap_response(
            raw, operation=f"get matching evaluation report {evaluation_id!r}"
        )
        if not isinstance(unwrapped, dict):
            raise TypeError(
                f"get matching evaluation report {evaluation_id!r}: expected dict, got {type(unwrapped)}"
            )
        return unwrapped

    async def trigger_bulk_analyze_compatibility(
        self, items: list[dict[str, Any]]
    ) -> list[dict[str, Any]]:
        response = await self._client.post(
            "/v1/jobs/bulk-analyze-compatibility",
            json=items,
        )
        self._check_response(response, operation="trigger bulk analyze compatibility")
        raw = response.json()
        unwrapped = self._unwrap_response(
            raw, operation="trigger bulk analyze compatibility"
        )
        if isinstance(unwrapped, list):
            return cast("list[dict[str, Any]]", unwrapped)
        return []

    async def post(self, path: str, json: Any) -> Any:
        response = await self._client.post(path, json=json)
        self._check_response(response, operation=f"POST {path}")
        raw = response.json()
        return self._unwrap_response(raw, operation=f"POST {path}")

    def _unwrap_response(
        self, raw: object, *, operation: str
    ) -> dict[str, Any] | list[Any] | None:
        if not isinstance(raw, dict):
            raise RuntimeError(  # noqa: TRY004
                f"{operation}: response is not a valid JSON object: {raw!r}"
            )

        raw_dict: dict[str, Any] = cast(dict[str, Any], raw)
        if "success" in raw_dict:
            if not raw_dict.get("success"):
                err = raw_dict.get("error")
                err_dict: dict[str, Any] = (
                    cast(dict[str, Any], err) if isinstance(err, dict) else {}
                )
                code = str(err_dict.get("code") or "UNKNOWN_ERROR")
                message = str(err_dict.get("message") or "API returned failure")
                raise RuntimeError(
                    f"{operation} failed with API error ({code}): {message}"
                )
            data = raw_dict.get("data")
            if data is not None and not isinstance(data, (dict, list)):
                raise RuntimeError(
                    f"{operation}: 'data' is not an object or list: {data!r}"
                )
            return cast("dict[str, Any] | list[Any] | None", data)

        return raw_dict

    async def aclose(self) -> None:
        await self._client.aclose()


def NewGetojobuAPI(
    *,
    base_url: str,
    api_key: str,
) -> GetojobuAPI:
    """Create a GetojobuAPI client backed by the Go API."""
    return _getojobuAPI(base_url=base_url, api_key=api_key)
