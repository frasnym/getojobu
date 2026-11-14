"""Structured resume-generation values and deterministic validation."""

from __future__ import annotations

import re
from typing import Any, cast

from pydantic import BaseModel, Field


class ResumeRole(BaseModel):
    company: str | None = None
    role: str | None = None
    period: str | None = None
    bullets: list[str] = Field(default_factory=list)
    selected: bool = False


class ResumeDraft(BaseModel):
    headline: str
    skills: list[str]
    jobs: dict[str, ResumeRole]
    education: str
    contact: str


class ResumeCritique(BaseModel):
    passed: bool
    issues: list[str] = Field(default_factory=list)


def validate_resume_draft(
    draft: ResumeDraft, *, facts: dict[str, Any], match_evidence: dict[str, Any]
) -> None:
    """Reject unsafe or non-ATS content before it reaches a renderer."""
    if not 6 <= len(draft.skills) <= 12:
        raise ValueError("skills must contain between 6 and 12 items")
    raw_roles = facts.get("roles")
    roles: list[object] = (
        cast(list[object], raw_roles) if isinstance(raw_roles, list) else []
    )
    role_ids: set[str] = set()
    for role in roles:
        if isinstance(role, dict):
            role_dict = cast(dict[str, object], role)
            role_id = role_dict.get("id")
            if role_id is not None:
                role_ids.add(str(role_id))
    missing = role_ids - set(draft.jobs)
    if missing:
        raise ValueError(f"resume is missing factual roles: {sorted(missing)}")
    selected = [role for role in draft.jobs.values() if role.selected]
    if len(selected) > 3:
        raise ValueError("resume may select at most 3 roles")
    if any(len(role.bullets) > 6 for role in selected):
        raise ValueError("a selected role may contain at most 6 bullets")

    text = " ".join(
        [draft.headline, draft.education, draft.contact, *draft.skills]
        + [bullet for role in draft.jobs.values() for bullet in role.bullets]
    )
    if any(ord(char) > 127 for char in text):
        raise ValueError("resume contains non-ASCII text")
    if re.search(r"(^|\s)([#*`]|[✓→•])", text):
        raise ValueError("resume contains unsupported ATS formatting")

    for role in draft.jobs.values():
        openings = [
            bullet.split(maxsplit=1)[0].lower() for bullet in role.bullets if bullet
        ]
        if len(openings) != len(set(openings)):
            raise ValueError("resume repeats an opening verb within a role")

    strengths = _strengths(match_evidence)
    if strengths and not any(
        strength.lower() in text.lower() for strength in strengths
    ):
        raise ValueError("resume does not surface a demonstrated match strength")


def _strengths(match_evidence: dict[str, Any]) -> list[str]:
    values: list[str] = []
    raw_strengths = match_evidence.get("strengths")
    strengths_list: list[object] = (
        cast(list[object], raw_strengths) if isinstance(raw_strengths, list) else []
    )
    for value in strengths_list:
        if isinstance(value, str):
            values.append(value)

    raw_categories = match_evidence.get("category_evaluations")
    cat_list: list[object] = (
        cast(list[object], raw_categories) if isinstance(raw_categories, list) else []
    )
    for category in cat_list:
        if isinstance(category, dict):
            cat_dict = cast(dict[str, object], category)
            cat_strengths = cat_dict.get("strengths")
            if isinstance(cat_strengths, list):
                item_list: list[object] = cast(list[object], cat_strengths)
                for item in item_list:
                    if isinstance(item, str):
                        values.append(item)
    return values
