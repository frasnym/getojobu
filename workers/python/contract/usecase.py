"""Usecase layer contract.

Mirrors apps/backend/internal/contract/usecase/contract.go: aggregates
usecase services behind a single container.
"""

from __future__ import annotations

from repository.getojobu_api import GetojobuAPI
from usecase.analyze_job.service import AnalyzeJob
from usecase.evaluate_match.service import EvaluateMatch
from usecase.generate_report.service import GenerateReport


class UsecaseContract:
    def __init__(
        self,
        analyze_job: AnalyzeJob,
        evaluate_match: EvaluateMatch | None = None,
        generate_report: GenerateReport | None = None,
    ) -> None:
        self.analyze_job = analyze_job
        self.evaluate_match = evaluate_match
        self.generate_report = generate_report


def new(
    api: GetojobuAPI,
    omni_base_url: str,
    omni_api_key: str,
    omni_model: str | None = None,
) -> UsecaseContract:
    """Build the usecase contract from the Go API client.

    Wires an OmniRoute-backed LLM client into the analyze_job, evaluate_match, and generate_report usecases.
    """
    from usecase.analyze_job.llm_client import OpenAIClient as AnalyzeJobLLMClient
    from usecase.evaluate_match.llm_client import OpenAIClient as EvaluateMatchLLMClient
    from usecase.generate_report.llm_client import OpenAIReportClient

    analyze_job_llm = AnalyzeJobLLMClient(
        api_key=omni_api_key, base_url=omni_base_url, model=omni_model or ""
    )
    match_llm = EvaluateMatchLLMClient(
        api_key=omni_api_key, base_url=omni_base_url, model=omni_model or ""
    )
    report_llm = OpenAIReportClient(
        api_key=omni_api_key, base_url=omni_base_url, model=omni_model or ""
    )
    return UsecaseContract(
        analyze_job=AnalyzeJob(
            api=api,
            llm=analyze_job_llm,
            model=omni_model,
        ),
        evaluate_match=EvaluateMatch(
            llm=match_llm,
            model=omni_model,
        ),
        generate_report=GenerateReport(
            llm=report_llm,
            model=omni_model,
        ),
    )
