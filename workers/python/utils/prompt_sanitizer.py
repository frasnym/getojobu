"""Sanitizers for prompt input payloads.

Removes internal database entity identifiers, version tags, timestamps,
and pipeline metadata before serializing data blocks into prompt templates.
"""

from __future__ import annotations

from typing import Any

# Metadata keys that should never be sent to the LLM in job_profile
_INTERNAL_JOB_KEYS: set[str] = {
    "id",
    "job_id",
    "version",
    "schema_version",
    "profile_data",
    "prompt_version",
    "model",
    "created_at",
}

# Metadata keys that should never be sent to the LLM in evaluation_results
_INTERNAL_EVALUATION_KEYS: set[str] = {
    "prompt_version",
    "model",
    "provider",
}


def sanitize_job_profile(job_profile: dict[str, Any]) -> dict[str, Any]:
    """Return a copy of job_profile stripped of database and pipeline metadata."""
    sanitized: dict[str, Any] = {}
    k: str
    v: Any
    for k, v in job_profile.items():
        if k not in _INTERNAL_JOB_KEYS:
            sanitized[k] = v
    return sanitized


def sanitize_evaluation_results(evaluation_results: dict[str, Any]) -> dict[str, Any]:
    """Return a copy of evaluation_results stripped of service metadata."""
    sanitized: dict[str, Any] = {}
    k: str
    v: Any
    for k, v in evaluation_results.items():
        if k not in _INTERNAL_EVALUATION_KEYS:
            sanitized[k] = v
    return sanitized
