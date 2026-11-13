"""Domain models for Step 2: Compatibility Evaluation.

Mirrors apps/backend/internal/models/: plain data structures with validation.
Uses pydantic for JSON Schema structured output with OpenAI/OmniRoute.
"""

from __future__ import annotations

from typing import Literal

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


class MatchEvaluationOutput(BaseModel):
    """Full structured output of the Step 2 LLM compatibility evaluation."""

    evaluations: list[CategoryEvaluation] = Field(
        default_factory=list[CategoryEvaluation]
    )
    mandatory_requirements: list[MandatoryRequirementAssessment] = Field(
        default_factory=list[MandatoryRequirementAssessment]
    )
