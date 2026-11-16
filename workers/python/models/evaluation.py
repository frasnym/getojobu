"""Domain models for Step 2: Compatibility Evaluation.

Mirrors apps/backend/internal/models/: plain data structures with validation.
Uses pydantic for JSON Schema structured output with OpenAI/OmniRoute.
"""

from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, Field


class CategoryEvidence(BaseModel):
    """Supporting evidence linking candidate details to a job requirement."""

    candidate_evidence: str = Field(..., min_length=1)
    job_requirement: str = Field(..., min_length=1)
    relationship: Literal["direct", "transferable"]


class CategoryEvaluation(BaseModel):
    """Evaluation of a single scoring category."""

    category: str = Field(..., min_length=1)
    score: int = Field(..., ge=0, le=100)
    reason: str = Field(..., min_length=1)
    strengths: list[str] = Field(default_factory=list[str])
    gaps: list[str] = Field(default_factory=list[str])
    evidence: list[CategoryEvidence] = Field(default_factory=list[CategoryEvidence])


class MandatoryRequirementAssessment(BaseModel):
    """Assessment of a single mandatory (required) job requirement."""

    requirement: str = Field(..., min_length=1)
    status: Literal["met", "not_met", "uncertain"]
    reason: str = Field(..., min_length=1)
    evidence: list[str] = Field(default_factory=list[str])


class MatchEvaluationExtraction(BaseModel):
    """Pure LLM structured output for Step 2 Compatibility Evaluation."""

    evaluations: list[CategoryEvaluation] = Field(
        default_factory=list[CategoryEvaluation]
    )
    mandatory_requirements: list[MandatoryRequirementAssessment] = Field(
        default_factory=list[MandatoryRequirementAssessment]
    )


class MatchEvaluationOutput(MatchEvaluationExtraction):
    """Full domain model for Step 2 compatibility evaluation including execution metadata.

    ``model`` and ``prompt_version`` are nested under ``metadata`` so they do not
    collide with LLM-structured output fields and can be stripped from API
    responses separately from the evaluation results.
    """

    metadata: dict[str, Any] = Field(
        default_factory=lambda: {"model": None, "prompt_version": "evaluate-match-v1"}
    )
