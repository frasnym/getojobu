"""Unit tests for prompt input sanitizers."""

from utils.prompt_sanitizer import sanitize_evaluation_results, sanitize_job_profile


def test_sanitize_job_profile_strips_internal_metadata():
    raw_job = {
        "metadata": {"model": "gpt-4o-mini", "prompt_version": "extract-job-v1"},
        "title": "Software Engineer",
        "company": "Motorola Solutions",
        "location": "Chicago, IL",
        "work_arrangement": "hybrid",
        "employment_type": "full-time",
        "family": "technology",
        "seniority": "junior",
        "minimum_years_experience": 1,
        "requirements": [
            {
                "original_name": "Golang",
                "normalized_name": "golang",
                "category": "technical_skills",
                "importance": "required",
            }
        ],
        "responsibilities": ["Develop microservices in Go"],
    }

    sanitized = sanitize_job_profile(raw_job)

    # Internal metadata keys must be removed
    assert "metadata" not in sanitized

    # Substantive job details must be preserved
    assert sanitized["title"] == "Software Engineer"
    assert sanitized["company"] == "Motorola Solutions"
    assert sanitized["location"] == "Chicago, IL"
    assert sanitized["work_arrangement"] == "hybrid"
    assert sanitized["family"] == "technology"
    assert sanitized["seniority"] == "junior"
    assert sanitized["minimum_years_experience"] == 1
    assert len(sanitized["requirements"]) == 1
    assert len(sanitized["responsibilities"]) == 1


def test_sanitize_job_profile_handles_empty():
    assert sanitize_job_profile({}) == {}


def test_sanitize_evaluation_results_strips_metadata():
    raw_results = {
        "metadata": {"model": "gpt-4o-mini", "prompt_version": "evaluate-match-v1"},
        "evaluations": [
            {
                "category": "technical_skills",
                "score": 85,
                "reason": "Strong Go skills",
                "strengths": ["Golang"],
                "gaps": [],
                "evidence": [],
            }
        ],
        "mandatory_requirements": [
            {
                "requirement": "Golang",
                "status": "met",
                "reason": "Present in CV",
                "evidence": [],
            }
        ],
    }

    sanitized = sanitize_evaluation_results(raw_results)

    assert "metadata" not in sanitized
    assert len(sanitized["evaluations"]) == 1
    assert len(sanitized["mandatory_requirements"]) == 1


def test_sanitize_evaluation_results_handles_empty():
    assert sanitize_evaluation_results({}) == {}
