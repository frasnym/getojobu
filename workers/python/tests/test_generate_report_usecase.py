"""Unit tests for the Step 4 GenerateReport service."""

from unittest.mock import AsyncMock

import pytest
from models.report import MatchReportExtraction, MatchReportOutput
from pydantic import ValidationError
from usecase.generate_report.service import GenerateReport


@pytest.mark.asyncio
async def test_generate_report_success() -> None:
    fake_llm_response = {
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
    assert result.summary == "Strong match for the position."
    assert len(result.strengths) == 2
    assert len(result.gaps) == 1
    assert result.gaps[0].requirement == "Kubernetes"
    assert result.gaps[0].status == "not_found_in_cv"
    assert result.metadata["model"] is None
    assert result.metadata["prompt_version"] == "generate-report-v1"
    assert result.metadata["recommendation_policy_version"] == "deterministic-v1"
    assert not hasattr(result, "provider")
    assert not hasattr(result, "model")


@pytest.mark.asyncio
async def test_generate_report_metadata_is_worker_owned() -> None:
    mock_llm = AsyncMock()
    mock_llm.generate_match_report.return_value = {
        "summary": "Strong match for the position.",
        "experience_assessment": "Relevant backend experience.",
        "metadata": {
            "model": "llm-chosen",
            "prompt_version": "llm-chosen",
            "recommendation_policy_version": "llm-chosen",
        },
    }
    service = GenerateReport(llm=mock_llm, model="worker-model")

    result = await service.generate(
        candidate_profile={"skills": ["golang"]},
        job_profile={"title": "Senior Backend Engineer"},
        evaluation_results={"evaluations": []},
        overall_score=85.0,
        eligibility_status="eligible",
        recommendation_label="worth_applying",
    )

    assert result.metadata == {
        "model": "worker-model",
        "prompt_version": "generate-report-v1",
        "recommendation_policy_version": "deterministic-v1",
    }


def test_match_report_extraction_schema_excludes_metadata() -> None:
    schema = MatchReportExtraction.model_json_schema()
    props = schema["properties"]
    assert "job_title" not in props
    assert "summary" in props
    assert "strengths" in props
    assert "gaps" in props
    assert "report_prompt_version" not in props
    assert "recommendation_policy_version" not in props


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
    mock_llm.generate_match_report.return_value = {}
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
