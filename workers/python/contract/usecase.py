"""Usecase layer contract.

Mirrors apps/backend/internal/contract/usecase/contract.go: aggregates
usecase services behind a single container.
"""

from __future__ import annotations

from repository.getojobu_api import JobProfiles
from usecase.analyze_job.service import AnalyzeJob


class UsecaseContract:
    def __init__(self, analyze_job: AnalyzeJob) -> None:
        self.analyze_job = analyze_job


def new(
    job_profiles: JobProfiles,
    omni_base_url: str,
    omni_api_key: str,
    omni_model: str,
) -> UsecaseContract:
    """Build the usecase contract from the API client.

    Wires an OmniRoute-backed LLM client into the analyze_job usecase.
    """
    from usecase.analyze_job.llm_client import OpenAIClient

    llm = OpenAIClient(api_key=omni_api_key, base_url=omni_base_url, model=omni_model)
    return UsecaseContract(analyze_job=AnalyzeJob(job_profiles=job_profiles, llm=llm))
