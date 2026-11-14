"""Usecase layer contract.

Mirrors apps/backend/internal/contract/usecase/contract.go: aggregates
usecase services behind a single container.
"""

from __future__ import annotations

from config.config import ScrapeConfig
from repository.getojobu_api import GetojobuAPI
from repository.scraper.base import ScraperRepository
from usecase.analyze_job.service import AnalyzeJob
from usecase.evaluate_match.service import EvaluateMatch
from usecase.generate_report.service import GenerateReport
from usecase.generate_resume.llm_client import OpenAIResumeClient
from usecase.generate_resume.service import GenerateResume
from usecase.scraper.service import ScraperService


class UsecaseContract:
    def __init__(
        self,
        analyze_job: AnalyzeJob,
        evaluate_match: EvaluateMatch | None = None,
        generate_report: GenerateReport | None = None,
        generate_resume: GenerateResume | None = None,
        scraper: ScraperService | None = None,
    ) -> None:
        self.analyze_job = analyze_job
        self.evaluate_match = evaluate_match
        self.generate_report = generate_report
        self.generate_resume = generate_resume
        self.scraper = scraper


def new(
    api: GetojobuAPI,
    omni_base_url: str,
    omni_api_key: str,
    omni_model: str | None = None,
    scrape_config: ScrapeConfig | None = None,
    scrapers: dict[str, ScraperRepository] | None = None,
) -> UsecaseContract:
    """Build the usecase contract from the Go API client and configuration."""
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
    resume_llm = OpenAIResumeClient(
        api_key=omni_api_key, base_url=omni_base_url, model=omni_model or ""
    )

    scraper_service: ScraperService | None = None
    if scrape_config is not None and scrapers is not None:
        scraper_service = ScraperService(
            configured_methods=scrape_config.vendor_methods,
            scrapers=scrapers,
        )
        scraper_service.validate_configured_methods()

    return UsecaseContract(
        analyze_job=AnalyzeJob(
            api=api,
            llm=analyze_job_llm,
        ),
        evaluate_match=EvaluateMatch(
            llm=match_llm,
        ),
        generate_report=GenerateReport(
            llm=report_llm,
        ),
        generate_resume=GenerateResume(llm=resume_llm),
        scraper=scraper_service,
    )
