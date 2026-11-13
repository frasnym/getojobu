"""Domain models for Step 4: Match Report Generation.

Mirrors docs/workflows/getojobu-job-matching-implementation-reference.md §9.
Uses pydantic for JSON Schema structured output with OpenAI/OmniRoute.
"""

from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field


class ReportGap(BaseModel):
    """Assessment of a skill or qualification gap with suggestions."""

    requirement: str = Field(..., min_length=1)
    status: Literal["met", "partially_met", "not_found_in_cv"]
    suggestion: str | None = None


class MatchReportOutput(BaseModel):
    """Full candidate-facing match report output."""

    job_title: str = Field(..., min_length=1)
    summary: str = Field(..., min_length=1)
    strengths: list[str] = Field(default_factory=list[str])
    gaps: list[ReportGap] = Field(default_factory=list[ReportGap])
    experience_assessment: str = Field(..., min_length=1)
    cv_improvements: list[str] = Field(default_factory=list[str])
    learning_recommendations: list[str] = Field(default_factory=list[str])
    interview_preparation: list[str] = Field(default_factory=list[str])
    report_prompt_version: str = Field(default="generate-report-v1")
    model: str | None = None
    provider: str | None = None
    recommendation_policy_version: str = Field(default="deterministic-v1")
