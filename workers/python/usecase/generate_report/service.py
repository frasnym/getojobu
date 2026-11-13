"""Business logic for Step 4: Match Report Generation."""

from __future__ import annotations

from typing import Any, Protocol

from models.report import MatchReportOutput


class ReportLLMClient(Protocol):
    """Protocol for an LLM client used to generate match report."""

    async def generate_match_report(
        self,
        *,
        candidate_profile: dict[str, Any],
        job_profile: dict[str, Any],
        evaluation_results: dict[str, Any],
        overall_score: float,
        eligibility_status: str,
        recommendation_label: str,
        prompt_version: str = "generate-report-v1",
    ) -> dict[str, Any]: ...


class GenerateReport:
    """Service that coordinates match report generation via LLM."""

    def __init__(
        self,
        llm: ReportLLMClient | None = None,
        *,
        prompt_version: str = "generate-report-v1",
    ) -> None:
        self._llm = llm
        self._prompt_version = prompt_version

    async def generate(
        self,
        *,
        candidate_profile: dict[str, Any],
        job_profile: dict[str, Any],
        evaluation_results: dict[str, Any],
        overall_score: float,
        eligibility_status: str,
        recommendation_label: str,
    ) -> MatchReportOutput:
        """Run report generation using candidate, job, evaluation, and authoritative facts."""
        if not candidate_profile:
            raise ValueError("candidate_profile is required")
        if not job_profile:
            raise ValueError("job_profile is required")
        if not evaluation_results:
            raise ValueError("evaluation_results is required")

        if self._llm is None:
            raise RuntimeError("llm client is required for report generation")

        raw_output = await self._llm.generate_match_report(
            candidate_profile=candidate_profile,
            job_profile=job_profile,
            evaluation_results=evaluation_results,
            overall_score=overall_score,
            eligibility_status=eligibility_status,
            recommendation_label=recommendation_label,
            prompt_version=self._prompt_version,
        )

        # Ensure metadata fields are populated
        raw_output.setdefault("report_prompt_version", self._prompt_version)
        raw_output.setdefault("recommendation_policy_version", "deterministic-v1")

        return MatchReportOutput.model_validate(raw_output)
