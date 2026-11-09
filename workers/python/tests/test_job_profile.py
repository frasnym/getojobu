"""Tests for the JobProfile domain model (Step 1B output).

Mirrors the backend's model validation patterns.
"""

from models.job_profile import JobProfile, JobRequirement


def test_job_profile_minimal():
    profile = JobProfile(
        family="technology",
        seniority="senior",
        title="Senior Backend Engineer",
        company="Acme",
    )
    assert profile.family == "technology"
    assert profile.seniority == "senior"
    assert profile.version == 1
    assert profile.requirements == []


def test_job_profile_with_requirements():
    profile = JobProfile(
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
    assert len(profile.requirements) == 1
    assert profile.requirements[0].normalized_name == "golang"
    assert profile.requirements[0].importance == "required"


def test_job_profile_rejects_invalid_family():
    try:
        JobProfile(family="not_a_family", seniority="unknown")
        assert False, "expected ValueError"
    except ValueError:
        pass


def test_job_profile_rejects_invalid_importance():
    try:
        JobProfile(
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
