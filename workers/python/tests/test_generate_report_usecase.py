"""Unit tests for the Step 4 GenerateReport service."""

from unittest.mock import AsyncMock

import pytest
from pydantic import ValidationError

from models.report import MatchReportOutput
from usecase.generate_report.service import GenerateReport


@pytest.mark.asyncio
async def test_generate_report_success() -> None:
    fake_llm_response = {
        "job_title": "Senior Backend Engineer",
        "summary": "Strong match for the position.",
        "strengths": ["Deep Golang experience", "Distributed systems"],
        "gaps": [
            {
                "requirement": "Kubernetes",
                "status": "not_found_in_cv",
                "suggestion": "Add k8s experience if any",
            }
        ],
        "experience_assessment": "6 years of high scale backend systems.",
        "cv_improvements": ["Highlight Kafka throughput metrics"],
        "learning_recommendations": ["AWS CDK basics"],
        "interview_preparation": ["System design concurrency patterns"],
        "report_prompt_version": "generate-report-v1",
        "recommendation_policy_version": "deterministic-v1",
    }

    mock_llm = AsyncMock()
    mock_llm.generate_match_report.return_value = fake_llm_response

    service = GenerateReport(llm=mock_llm)
    result = await service.generate(
        candidate_profile={"skills": ["golang"]},
        job_profile={"title": "Senior Backend Engineer"},
        evaluation_results={"evaluations": []},
        overall_score=85.0,
        eligibility_status="eligible",
        recommendation_label="worth_applying",
    )

    assert isinstance(result, MatchReportOutput)
    assert result.job_title == "Senior Backend Engineer"
    assert result.summary == "Strong match for the position."
    assert len(result.strengths) == 2
    assert len(result.gaps) == 1
    assert result.gaps[0].requirement == "Kubernetes"
    assert result.gaps[0].status == "not_found_in_cv"
    assert result.report_prompt_version == "generate-report-v1"
    assert result.recommendation_policy_version == "deterministic-v1"
    assert not hasattr(result, "provider")
    assert not hasattr(result, "model")


@pytest.mark.asyncio
async def test_generate_report_missing_inputs() -> None:
    service = GenerateReport(llm=AsyncMock())

    with pytest.raises(ValueError, match="candidate_profile is required"):
        await service.generate(
            candidate_profile={},
            job_profile={"title": "Backend Engineer"},
            evaluation_results={"evaluations": []},
            overall_score=80.0,
            eligibility_status="eligible",
            recommendation_label="worth_applying",
        )

    with pytest.raises(ValueError, match="job_profile is required"):
        await service.generate(
            candidate_profile={"skills": ["golang"]},
            job_profile={},
            evaluation_results={"evaluations": []},
            overall_score=80.0,
            eligibility_status="eligible",
            recommendation_label="worth_applying",
        )

    with pytest.raises(ValueError, match="evaluation_results is required"):
        await service.generate(
            candidate_profile={"skills": ["golang"]},
            job_profile={"title": "Backend Engineer"},
            evaluation_results={},
            overall_score=80.0,
            eligibility_status="eligible",
            recommendation_label="worth_applying",
        )


@pytest.mark.asyncio
async def test_generate_report_invalid_schema() -> None:
    mock_llm = AsyncMock()
    # Missing required 'summary'
    mock_llm.generate_match_report.return_value = {
        "job_title": "Backend Dev",
    }
    service = GenerateReport(llm=mock_llm)

    with pytest.raises(ValidationError):
        await service.generate(
            candidate_profile={"skills": ["golang"]},
            job_profile={"title": "Backend Dev"},
            evaluation_results={"evaluations": []},
            overall_score=80.0,
            eligibility_status="eligible",
            recommendation_label="worth_applying",
        )
