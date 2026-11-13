"""Tests for Step 2 evaluation domain models."""

import pytest
from pydantic import ValidationError

from models.evaluation import (
    CategoryEvaluation,
    CategoryEvidence,
    MandatoryRequirementAssessment,
    MatchEvaluationOutput,
)


def test_category_evidence_valid() -> None:
    ev = CategoryEvidence(
        candidate_evidence="6 years in Go",
        job_requirement="Strong Go experience",
        relationship="direct",
    )
    assert ev.relationship == "direct"
    assert ev.candidate_evidence == "6 years in Go"


def test_category_evidence_invalid_relationship() -> None:
    with pytest.raises(ValidationError):
        CategoryEvidence(
            candidate_evidence="6 years in Go",
            job_requirement="Strong Go experience",
            relationship="unrelated",  # must be direct or transferable
        )


def test_category_evaluation_valid() -> None:
    eval_item = CategoryEvaluation(
        category="technical_skills",
        score=90,
        reason="Demonstrates required backend skills",
        strengths=["Golang", "PostgreSQL"],
        gaps=["No Kubernetes"],
        evidence=[
            CategoryEvidence(
                candidate_evidence="Built payment service in Go",
                job_requirement="Go backend service",
                relationship="direct",
            )
        ],
    )
    assert eval_item.category == "technical_skills"
    assert eval_item.score == 90


@pytest.mark.parametrize("invalid_score", [-1, 101])
def test_category_evaluation_invalid_score(invalid_score: int) -> None:
    with pytest.raises(ValidationError):
        CategoryEvaluation(
            category="technical_skills",
            score=invalid_score,
            reason="out of bounds",
        )


def test_mandatory_requirement_assessment_valid() -> None:
    m = MandatoryRequirementAssessment(
        requirement="5+ years experience",
        status="met",
        reason="Candidate has 6 years experience",
        evidence=["6 years listed on CV"],
    )
    assert m.status == "met"


def test_mandatory_requirement_assessment_invalid_status() -> None:
    with pytest.raises(ValidationError):
        MandatoryRequirementAssessment(
            requirement="5+ years experience",
            status="unknown_status",  # allowed: met, not_met, uncertain
            reason="bad",
        )


def test_match_evaluation_output_roundtrip() -> None:
    output = MatchEvaluationOutput(
        evaluations=[
            CategoryEvaluation(
                category="technical_skills",
                score=85,
                reason="Good fit",
            )
        ],
        mandatory_requirements=[
            MandatoryRequirementAssessment(
                requirement="Go",
                status="met",
                reason="Explicitly mentioned",
            )
        ],
    )
    dumped = output.model_dump()
    assert "evaluations" in dumped
    assert "mandatory_requirements" in dumped

    # Ensure JSON schema generation works (needed by OpenAI structured output)
    schema = MatchEvaluationOutput.model_json_schema()
    assert "properties" in schema
    assert "evaluations" in schema["properties"]
    assert "mandatory_requirements" in schema["properties"]
