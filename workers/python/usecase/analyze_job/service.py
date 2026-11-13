"""Business logic for job analysis (Step 1B).

Flow:
1. Check status via Go API GET /v1/jobs/{slug}.
2. If status is not 'new' or 'failed', reject.
3. Update status to 'analyze_pending' via PATCH /v1/jobs/{slug}.
4. Analyze job description using LLM.
5. Update status based on result:
   - On success: 'analyzed' with JobAnalysis JSON.
   - On failure: 'failed' with error message.
"""

from __future__ import annotations

from typing import Any, Protocol

import httpx

from models.job_analysis import JobAnalysis
from repository.getojobu_api import GetojobuAPI
from utils.logging import get_logger

logger = get_logger("usecase.analyze_job")


class LLMClient(Protocol):
    """Protocol for an LLM client used to extract job analysis."""

    async def extract_job_analysis(
        self, *, job_description: str, prompt_version: str
    ) -> dict[str, Any]: ...


class AnalyzeJob:
    """Service that coordinates job analysis with the Go API and LLM."""

    def __init__(
        self,
        api: GetojobuAPI | None = None,
        llm: LLMClient | None = None,
        *,
        prompt_version: str = "extract-job-v1",
        model: str | None = None,
    ) -> None:
        self._api = api
        self._llm = llm
        self._prompt_version = prompt_version
        self._model = model

    async def analyze(
        self,
        slug: str,
        *,
        job_description: str | None = None,
        job_id: str | None = None,
    ) -> JobAnalysis:
        """Run the complete 5-step job analysis lifecycle for a given job slug."""
        if not slug or not slug.strip():
            raise ValueError("slug is required")

        if self._api is None:
            raise RuntimeError("api client is required for job analysis lifecycle")

        # 1. Check current status from Go API
        job = await self._api.get_job(slug)
        current_status = job.get("status")
        if not current_status:
            raise RuntimeError(f"job {slug!r} response missing 'status' field")

        # 2. Reject if not 'new' or 'failed'
        if current_status not in ("new", "failed"):
            raise ValueError(
                f"job {slug!r} cannot be analyzed: current status is {current_status!r} (must be 'new' or 'failed')"
            )

        # 3. Transition status to 'analyze_pending' and verify response
        pending_resp = await self._api.patch_job_status(
            slug=slug, status="analyze_pending"
        )
        if pending_resp.get("status") != "analyze_pending":
            actual_status = pending_resp.get("status")
            raise RuntimeError(
                f"failed to transition job {slug!r} to 'analyze_pending': API returned status {actual_status!r}"
            )

        # Resolve job description from input or fetched job record
        jd = job_description or job.get("description")
        if not jd or not jd.strip():
            err_msg = f"job {slug!r} has no description to analyze"
            await self._api.patch_job_status(
                slug=slug, status="failed", failure_message=err_msg
            )
            raise ValueError(err_msg)

        resolved_job_id = job_id or str(job.get("id", ""))

        # 4. Perform LLM analysis
        try:
            analysis = await self._call_llm(job_description=jd, job_id=resolved_job_id)
        except Exception as exc:
            # 5b. Transition status to 'failed' on error
            try:
                fail_resp = await self._api.patch_job_status(
                    slug=slug, status="failed", failure_message=str(exc)
                )
                if fail_resp.get("status") != "failed":
                    pass  # keep underlying LLM error as primary exception
            except Exception:  # noqa: BLE001, S110
                pass  # do not swallow the underlying LLM error
            raise

        # 5a. Transition status to 'analyzed' and verify response
        analyzed_resp = await self._api.patch_job_status(
            slug=slug,
            status="analyzed",
            analyze_result=analysis.model_dump_json(),
        )
        if analyzed_resp.get("status") != "analyzed":
            actual_status = analyzed_resp.get("status")
            raise RuntimeError(
                f"failed to transition job {slug!r} to 'analyzed': API returned status {actual_status!r}"
            )

        # 6. Trigger compatibility evaluation (Step 2)
        if hasattr(self._api, "trigger_analyze_compatibility"):
            try:
                await self._api.trigger_analyze_compatibility(slug=slug)
            except (RuntimeError, httpx.HTTPError) as exc:
                logger.warning(
                    "trigger_analyze_compatibility_failed",
                    slug=slug,
                    error=str(exc),
                )

        return analysis

    async def _call_llm(self, *, job_description: str, job_id: str) -> JobAnalysis:
        if self._llm is None:
            raise ValueError("llm client is required")

        raw = await self._llm.extract_job_analysis(
            job_description=job_description,
            prompt_version=self._prompt_version,
        )

        analysis = JobAnalysis.model_validate(raw)
        if job_id:
            analysis.job_id = job_id
        analysis.model = self._model
        analysis.prompt_version = self._prompt_version
        return analysis
