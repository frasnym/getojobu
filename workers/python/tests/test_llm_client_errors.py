"""Provider failures surface as a clear LLM error in every LLM client."""

from unittest.mock import AsyncMock

import pytest

from usecase.analyze_job.llm_client import OpenAIClient as AnalyzeJobLLMClient
from usecase.generate_report.llm_client import OpenAIReportClient
from usecase.generate_resume.llm_client import OpenAIResumeClient
from utils.llm_errors import LLMError

PROVIDER_ERROR = "Error code: 499 - Request aborted"


@pytest.mark.asyncio
async def test_job_analysis_wraps_provider_error() -> None:
    client = AnalyzeJobLLMClient(
        api_key="test-key", base_url="http://mock", model="test-model"
    )
    client._client.chat.completions.create = AsyncMock(  # type: ignore[method-assign]
        side_effect=Exception(PROVIDER_ERROR)
    )

    with pytest.raises(LLMError) as exc_info:
        await client.extract_job_analysis(
            job_description="Backend engineer", prompt_version="extract-job-v1"
        )

    assert exc_info.value.message == "LLM error during job analysis"
    assert exc_info.value.detail == PROVIDER_ERROR


@pytest.mark.asyncio
async def test_report_generation_wraps_provider_error() -> None:
    client = OpenAIReportClient(
        api_key="test-key", base_url="http://mock", model="test-model"
    )
    client._client.chat.completions.create = AsyncMock(  # type: ignore[method-assign]
        side_effect=Exception(PROVIDER_ERROR)
    )

    with pytest.raises(LLMError) as exc_info:
        await client.generate_match_report(
            candidate_profile={},
            job_profile={},
            evaluation_results={},
            overall_score=0.5,
            eligibility_status="eligible",
            recommendation_label="worth_applying",
        )

    assert exc_info.value.message == "LLM error during report generation"
    assert exc_info.value.detail == PROVIDER_ERROR


@pytest.mark.asyncio
async def test_resume_generation_wraps_provider_error() -> None:
    client = OpenAIResumeClient(
        api_key="test-key", base_url="http://mock", model="test-model"
    )
    client._client.chat.completions.create = AsyncMock(  # type: ignore[method-assign]
        side_effect=Exception(PROVIDER_ERROR)
    )

    with pytest.raises(LLMError) as exc_info:
        await client.generate_resume(prompt_version="generate-resume-v1", resume="CV")

    assert exc_info.value.message == "LLM error during resume generation"
    assert exc_info.value.detail == PROVIDER_ERROR
