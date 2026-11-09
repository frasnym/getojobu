"""Usecase layer contract.

Mirrors apps/backend/internal/contract/usecase/contract.go: aggregates
usecase services behind a single container.
"""

from __future__ import annotations

from usecase.analyze_job.llm_client import OpenAIClient
from usecase.analyze_job.service import AnalyzeJob


class Contract:
    def __init__(self, analyze_job: AnalyzeJob) -> None:
        self.analyze_job = analyze_job


def new(repository, llm_provider: str, llm_model: str, llm_api_key: str) -> Contract:
    """Build the usecase contract from the repository contract.

    Wires an OpenAI LLM client into the analyze_job usecase.
    """
    llm = OpenAIClient(api_key=llm_api_key, model=llm_model)
    return Contract(analyze_job=AnalyzeJob(repo=repository.job_profiles, llm=llm))
