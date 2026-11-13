"""Unit tests for the Step 2 EvaluateMatch service."""

from unittest.mock import AsyncMock

import pytest
from pydantic import ValidationError

from models.evaluation import MatchEvaluationOutput
from usecase.evaluate_match.service import EvaluateMatch


@pytest.mark.asyncio
async def test_evaluate_match_success() -> None:
    fake_llm_response = {
        "evaluations": [
            {
                "category": "technical_skills",
                "score": 85,
                "reason": "Good match",
                "strengths": ["Golang"],
                "gaps": [],
                "evidence": [],
            }
        ],
        "mandatory_requirements": [
            {
                "requirement": "5+ years",
                "status": "met",
                "reason": "6 years shown",
                "evidence": ["Experience section"],
            }
        ],
    }

    mock_llm = AsyncMock()
    mock_llm.extract_match_evaluation.return_value = fake_llm_response

    service = EvaluateMatch(llm=mock_llm)
    result = await service.evaluate(
        candidate_profile={"skills": ["golang"]},
        job_profile={"title": "Backend Engineer"},
        evaluation_config={"family": "technology"},
    )

    assert isinstance(result, MatchEvaluationOutput)
    assert len(result.evaluations) == 1
    assert result.evaluations[0].score == 85
    assert len(result.mandatory_requirements) == 1
    assert result.mandatory_requirements[0].status == "met"


@pytest.mark.asyncio
async def test_evaluate_match_missing_inputs() -> None:
    service = EvaluateMatch(llm=AsyncMock())

    with pytest.raises(ValueError, match="candidate_profile is required"):
        await service.evaluate(
            candidate_profile={},
            job_profile={"title": "Backend Engineer"},
            evaluation_config={"family": "technology"},
        )

    with pytest.raises(ValueError, match="job_profile is required"):
        await service.evaluate(
            candidate_profile={"skills": ["golang"]},
            job_profile={},
            evaluation_config={"family": "technology"},
        )

    with pytest.raises(ValueError, match="evaluation_config is required"):
        await service.evaluate(
            candidate_profile={"skills": ["golang"]},
            job_profile={"title": "Backend Engineer"},
            evaluation_config={},
        )


@pytest.mark.asyncio
async def test_evaluate_match_invalid_llm_output() -> None:
    # Invalid score (> 100) from LLM
    fake_invalid_response = {
        "evaluations": [
            {
                "category": "technical_skills",
                "score": 150,  # Invalid
                "reason": "Invalid score",
            }
        ],
        "mandatory_requirements": [],
    }

    mock_llm = AsyncMock()
    mock_llm.extract_match_evaluation.return_value = fake_invalid_response

    service = EvaluateMatch(llm=mock_llm)
    with pytest.raises(ValidationError):
        await service.evaluate(
            candidate_profile={"skills": ["golang"]},
            job_profile={"title": "Backend Engineer"},
            evaluation_config={"family": "technology"},
        )
