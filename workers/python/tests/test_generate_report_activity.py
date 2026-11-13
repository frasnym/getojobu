"""Tests for Step 4 Match Report Generation activities."""

from unittest.mock import AsyncMock, MagicMock

import pytest

from deliveries.consumer.activities.report_generation import (
    generate_report_activity,
    init_report_generation_container,
    update_report_status_activity,
)
from models.report import MatchReportOutput


@pytest.mark.asyncio
async def test_generate_report_activity_success():
    fake_report = MatchReportOutput(
        job_title="Senior Backend Engineer",
        summary="Excellent backend match",
        strengths=["Go", "PostgreSQL"],
        gaps=[],
        experience_assessment="Solid backend background",
        cv_improvements=[],
        learning_recommendations=[],
        interview_preparation=[],
    )

    mock_container = MagicMock()
    mock_container.usecase.generate_report.generate = AsyncMock(return_value=fake_report)
    init_report_generation_container(mock_container)

    res = await generate_report_activity(
        {
            "candidate_profile": {"skills": ["go"]},
            "job_profile": {"title": "Senior Backend Engineer"},
            "evaluation_results": {"evaluations": []},
            "overall_score": 88.5,
            "eligibility_status": "eligible",
            "recommendation_label": "worth_applying",
        }
    )

    assert res["job_title"] == "Senior Backend Engineer"
    assert res["summary"] == "Excellent backend match"
    mock_container.usecase.generate_report.generate.assert_awaited_once()


@pytest.mark.asyncio
async def test_generate_report_activity_missing_inputs():
    with pytest.raises(ValueError, match="candidate_profile is required"):
        await generate_report_activity(
            {
                "candidate_profile": {},
                "job_profile": {"title": "Dev"},
                "evaluation_results": {"evaluations": []},
                "overall_score": 80.0,
                "eligibility_status": "eligible",
                "recommendation_label": "worth_applying",
            }
        )

    with pytest.raises(ValueError, match="job_profile is required"):
        await generate_report_activity(
            {
                "candidate_profile": {"skills": ["go"]},
                "job_profile": {},
                "evaluation_results": {"evaluations": []},
                "overall_score": 80.0,
                "eligibility_status": "eligible",
                "recommendation_label": "worth_applying",
            }
        )

    with pytest.raises(ValueError, match="overall_score is required"):
        await generate_report_activity(
            {
                "candidate_profile": {"skills": ["go"]},
                "job_profile": {"title": "Dev"},
                "evaluation_results": {"evaluations": []},
                "eligibility_status": "eligible",
                "recommendation_label": "worth_applying",
            }
        )


@pytest.mark.asyncio
async def test_update_report_status_activity_success():
    mock_container = MagicMock()
    mock_container.repository.getojobu_api.patch_matching_evaluation_report = AsyncMock(
        return_value={"id": "eval-1", "report_status": "generated"}
    )
    init_report_generation_container(mock_container)

    res = await update_report_status_activity(
        {
            "evaluation_id": "eval-1",
            "report_status": "generated",
            "report_data": {"job_title": "Backend Dev"},
        }
    )

    assert res["report_status"] == "generated"
    mock_container.repository.getojobu_api.patch_matching_evaluation_report.assert_awaited_once_with(
        evaluation_id="eval-1",
        report_status="generated",
        report_data={"job_title": "Backend Dev"},
        report_failure_message=None,
    )


@pytest.mark.asyncio
async def test_update_report_status_activity_failure():
    mock_container = MagicMock()
    mock_container.repository.getojobu_api.patch_matching_evaluation_report = AsyncMock(
        return_value={"id": "eval-1", "report_status": "failed"}
    )
    init_report_generation_container(mock_container)

    res = await update_report_status_activity(
        {
            "evaluation_id": "eval-1",
            "report_status": "failed",
            "report_failure_message": "LLM timeout",
        }
    )

    assert res["report_status"] == "failed"
    mock_container.repository.getojobu_api.patch_matching_evaluation_report.assert_awaited_once_with(
        evaluation_id="eval-1",
        report_status="failed",
        report_data=None,
        report_failure_message="LLM timeout",
    )
