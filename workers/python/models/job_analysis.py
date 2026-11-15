"""Domain models for job analysis extraction (Step 1B).

Mirrors apps/backend/internal/models/: plain data structures with
validation. Uses pydantic so LLM JSON output can be parsed and validated
in one step.
"""

from pydantic import BaseModel, Field, field_validator


class JobRequirement(BaseModel):
    """A single requirement extracted from a job description."""

    original_name: str = Field(..., min_length=1)
    normalized_name: str | None = None
    category: str = Field(..., min_length=1)
    importance: str  # required | preferred | optional | unknown
    source_excerpt: str | None = None

    @field_validator("importance")
    @classmethod
    def _validate_importance(cls, v: str) -> str:
        allowed = {"required", "preferred", "optional", "unknown"}
        if v not in allowed:
            raise ValueError(f"importance must be one of {sorted(allowed)}")
        return v


class JobExtractionOutput(BaseModel):
    """Normalized job description output extracted by LLM in Step 1B (extract-job-v1)."""

    family: str  # technology | administration | ... | other
    work_arrangement: str | None = None  # remote | hybrid | onsite | unknown
    employment_type: str | None = None
    seniority: (
        str  # entry | junior | mid | senior | lead | manager | executive | unknown
    )
    minimum_years_experience: int | None = Field(default=None, ge=0)
    requirements: list[JobRequirement] = []
    responsibilities: list[str] = []

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


class JobAnalysis(JobExtractionOutput):
    """Full domain model for job analysis including database/workflow metadata."""

    prompt_version: str = "extract-job-v1"
    model: str | None = None
