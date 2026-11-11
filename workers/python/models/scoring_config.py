"""Domain models for Job Scoring Configurations."""

from __future__ import annotations

from datetime import datetime
from typing import Self

from pydantic import BaseModel, ConfigDict, Field, model_validator


class CategoryConfig(BaseModel):
    """Configuration for a single scoring category."""

    model_config = ConfigDict(frozen=True)

    name: str = Field(..., description="Unique category key (e.g. 'technical_skills')")
    weight: float = Field(..., ge=0.0, le=1.0, description="Weight between 0.0 and 1.0")
    description: str = Field(..., description="Category description and evaluation criteria")


class ScoringConfig(BaseModel):
    """Family-specific versioned scoring configuration."""

    model_config = ConfigDict(frozen=True)

    id: str = Field(..., description="UUID primary key")
    family: str = Field(..., description="Job family key (e.g. 'technology')")
    version: int = Field(..., ge=1, description="Version number")
    categories: list[CategoryConfig] = Field(..., min_length=1, description="Category definitions and weights")
    is_active: bool = Field(..., description="Whether this version is active for the family")
    created_at: datetime = Field(..., description="Record creation timestamp")
    updated_at: datetime = Field(..., description="Record update timestamp")

    @model_validator(mode="after")
    def validate_categories_sum_and_uniqueness(self) -> Self:
        seen: set[str] = set()
        total_weight = 0.0
        for cat in self.categories:
            lower = cat.name.strip().lower()
            if lower in seen:
                raise ValueError(f"duplicate category name: {cat.name}")
            seen.add(lower)
            total_weight += cat.weight

        if not (0.999 <= total_weight <= 1.001):
            raise ValueError(f"weights must sum to 1.0, got {total_weight}")
        return self

