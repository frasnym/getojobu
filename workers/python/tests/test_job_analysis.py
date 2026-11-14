"""Tests for the JobAnalysis domain model (Step 1B output).

Mirrors the backend's model validation patterns.
"""

import pytest
from models.job_analysis import JobAnalysis, JobExtractionOutput, JobRequirement
from pydantic import ValidationError


def test_job_analysis_minimal():
    analysis = JobAnalysis(
        family="technology",
        seniority="senior",
        title="Senior Backend Engineer",
        company="Acme",
    )
    assert analysis.family == "technology"
    assert analysis.seniority == "senior"
    assert analysis.version == 1
    assert analysis.requirements == []


def test_job_extraction_output_schema_excludes_metadata():
    schema = JobExtractionOutput.model_json_schema()
    props = schema["properties"]
    assert "id" not in props
    assert "job_id" not in props
    assert "version" not in props
    assert "schema_version" not in props
    assert "profile_data" not in props
    assert "prompt_version" not in props
    assert "model" not in props
    assert "created_at" not in props

    assert "family" in props
    assert "seniority" in props
    assert "requirements" in props
    assert "responsibilities" in props


def test_job_analysis_accepts_slug_or_custom_string_id():
    raw = {
        "id": "job-motorola-sw-newgrad",
        "family": "technology",
        "seniority": "entry",
        "title": "Software Engineer New Grad",
        "company": "Motorola Solutions",
    }
    analysis = JobAnalysis.model_validate(raw)
    assert analysis.id == "job-motorola-sw-newgrad"
    assert analysis.family == "technology"
    assert analysis.seniority == "entry"


def test_job_analysis_rejects_none_id():
    raw_none = {
        "id": None,
        "family": "technology",
        "seniority": "mid",
    }
    with pytest.raises(ValidationError):
        JobAnalysis.model_validate(raw_none)


def test_job_extraction_output_rejects_none_requirements():
    raw = {
        "family": "technology",
        "seniority": "mid",
        "requirements": None,
    }
    with pytest.raises(ValidationError):
        JobExtractionOutput.model_validate(raw)


def test_job_analysis_with_requirements():
    analysis = JobAnalysis(
        family="technology",
        seniority="mid",
        requirements=[
            JobRequirement(
                original_name="Golang",
                normalized_name="golang",
                category="technical_skills",
                importance="required",
                source_excerpt="Strong Golang experience.",
            )
        ],
    )
    assert len(analysis.requirements) == 1
    assert analysis.requirements[0].normalized_name == "golang"
    assert analysis.requirements[0].importance == "required"


def test_job_analysis_rejects_invalid_family():
    try:
        JobAnalysis(family="not_a_family", seniority="unknown")
        assert False, "expected ValueError"
    except ValueError:
        pass


def test_job_analysis_rejects_invalid_importance():
    try:
        JobAnalysis(
            family="technology",
            seniority="unknown",
            requirements=[
                JobRequirement(
                    original_name="X",
                    normalized_name="x",
                    category="c",
                    importance="not_a_level",
                )
            ],
        )
        assert False, "expected ValueError"
    except ValueError:
        pass
