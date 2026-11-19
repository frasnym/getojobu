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

from models.job_analysis import JobAnalysis, JobExtractionOutput
from repository.getojobu_api import GetojobuAPI


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

        # 2. Reject if not 'queued' or 'queued_retry'
        if current_status not in ("queued", "queued_retry"):
            raise ValueError(
                f"job {slug!r} cannot be analyzed: current status is {current_status!r} (must be 'queued' or 'queued_retry')"
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

        # 4. Perform LLM analysis
        try:
            analysis = await self._call_llm(job_description=jd)
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

        # 6. Insert evaluations, then start their workflows. A failed trigger
        # fails the activity so Temporal retries both calls; the insert step
        # is create-if-absent, so a retry cannot duplicate evaluation rows.
        created = await self._api.create_evaluations([{"slug": slug}])
        trigger_items = []
        for item in created:
            if not isinstance(item, dict) or not item.get("id") or item.get("error"):
                continue
            trigger = {"id": item["id"]}
            if item.get("user_id"):
                trigger["user_id"] = item["user_id"]
            trigger_items.append(trigger)
        if trigger_items:
            trigger_result = await self._api.trigger_evaluations(trigger_items)
            # The trigger endpoint answers 202 even when starting a workflow
            # failed; the failure lives in each item's `error`. Fail the
            # activity so Temporal retries both calls instead of orphaning the
            # evaluation at `new`.
            for item in trigger_result:
                if isinstance(item, dict) and item.get("error"):
                    raise RuntimeError(f"trigger evaluations failed: {item['error']}")

        return analysis

    async def _call_llm(self, *, job_description: str) -> JobAnalysis:
        if self._llm is None:
            raise ValueError("llm client is required")

        raw: object = await self._llm.extract_job_analysis(
            job_description=job_description,
            prompt_version=self._prompt_version,
        )

        if not isinstance(raw, dict):
            raise ValueError(
                f"LLM returned a {type(raw).__name__} instead of a JSON object matching JobExtractionOutput"
            )

        extraction = JobExtractionOutput.model_validate(raw)
        model = self._model if isinstance(self._model, str) else None
        if not model:
            val = getattr(self._llm, "_model", None)
            if isinstance(val, str):
                model = val

        return JobAnalysis(
            **extraction.model_dump(),
            metadata={"model": model, "prompt_version": self._prompt_version},
        )
