"""Domain models for job profile extraction (Step 1B).

Mirrors apps/backend/internal/models/: plain data structures with
validation. Uses pydantic so LLM JSON output can be parsed and validated
in one step.
"""

from datetime import datetime
from typing import List, Optional
from uuid import UUID, uuid4

from pydantic import BaseModel, Field, field_validator


class JobRequirement(BaseModel):
    """A single requirement extracted from a job description."""

    original_name: str = Field(..., min_length=1)
    normalized_name: Optional[str] = None
    category: str = Field(..., min_length=1)
    importance: str  # required | preferred | optional | unknown
    source_excerpt: Optional[str] = None

    @field_validator("importance")
    @classmethod
    def _validate_importance(cls, v: str) -> str:
        allowed = {"required", "preferred", "optional", "unknown"}
        if v not in allowed:
            raise ValueError(f"importance must be one of {sorted(allowed)}")
        return v


class JobProfile(BaseModel):
    """Normalized job description output of Step 1B (extract-job-v1)."""

    id: UUID = Field(default_factory=uuid4)
    job_id: Optional[str] = None  # reference to shared jobs table slug/id
    version: int = 1
    schema_version: str = "1.0"
    family: str  # technology | administration | ... | other
    title: Optional[str] = None
    company: Optional[str] = None
    location: Optional[str] = None
    work_arrangement: Optional[str] = None  # remote | hybrid | onsite | unknown
    employment_type: Optional[str] = None
    seniority: (
        str  # entry | junior | mid | senior | lead | manager | executive | unknown
    )
    minimum_years_experience: Optional[int] = Field(default=None, ge=0)
    requirements: List[JobRequirement] = Field(default_factory=list)
    responsibilities: List[str] = Field(default_factory=list)
    profile_data: dict = Field(default_factory=dict)  # raw normalized JSONB payload
    prompt_version: str = "extract-job-v1"
    provider: str = "openai"
    model: str = "gpt-4o-mini"
    created_at: Optional[datetime] = None

    @field_validator("family")
    @classmethod
    def _validate_family(cls, v: str) -> str:
        allowed = {
            "technology",
            "administration",
            "customer_operations",
            "transportation",
            "sales",
            "marketing",
            "finance",
            "human_resources",
            "engineering",
            "healthcare",
            "hospitality",
            "education",
            "other",
        }
        if v not in allowed:
            raise ValueError(f"family must be one of {sorted(allowed)}")
        return v

    @field_validator("seniority")
    @classmethod
    def _validate_seniority(cls, v: str) -> str:
        allowed = {
            "entry",
            "junior",
            "mid",
            "senior",
            "lead",
            "manager",
            "executive",
            "unknown",
        }
        if v not in allowed:
            raise ValueError(f"seniority must be one of {sorted(allowed)}")
        return v
