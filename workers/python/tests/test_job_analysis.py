"""Tests for the JobAnalysis domain model (Step 1B output).

Mirrors the backend's model validation patterns.
"""

from models.job_analysis import JobAnalysis, JobRequirement


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
