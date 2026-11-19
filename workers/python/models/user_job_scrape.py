"""Structured fields extracted from a user-supplied job page."""

from __future__ import annotations

from pydantic import BaseModel, Field


class ScrapedJobFields(BaseModel):
    """Job fields returned by the URL extraction LLM."""

    title: str = Field(default="", description="Job title")
    company: str = Field(default="", description="Company name")
    location: str = Field(default="", description="Job location")
    description: str = Field(default="", description="Job description")
    apply_url: str | None = None
    posted_date: str | None = None
    is_remote: bool | None = None
    visa_relocation: bool | None = None
