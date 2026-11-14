"""Prompt input composition for resume generation."""

from __future__ import annotations

import json
from typing import Any


def build_resume_content(
    *,
    candidate_profile: dict[str, Any],
    job_profile: dict[str, Any],
    evaluation_results: dict[str, Any],
    facts: dict[str, Any],
    overall_score: float,
    eligibility_status: str,
    recommendation_label: str,
) -> dict[str, str]:
    required = {
        "candidate_profile": candidate_profile,
        "job_profile": job_profile,
        "match_evidence": evaluation_results,
        "facts": facts,
    }
    missing = [name for name, value in required.items() if not value]
    if missing:
        raise ValueError(f"{missing[0]} is required")
    evidence = dict(evaluation_results)
    evidence.update(
        overall_score=overall_score,
        eligibility_status=eligibility_status,
        recommendation_label=recommendation_label,
    )
    return {
        "candidate_profile": json.dumps(candidate_profile, sort_keys=True),
        "job_profile": json.dumps(job_profile, sort_keys=True),
        "match_evidence": json.dumps(evidence, sort_keys=True),
        "facts": json.dumps(facts, sort_keys=True),
    }
