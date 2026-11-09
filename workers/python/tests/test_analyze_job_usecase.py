"""Tests for the analyze_job usecase validation (Step 1B business logic).

Mirrors the backend's usecase test patterns.
"""

import pytest

from usecase.analyze_job.service import AnalyzeJob


class _FakeLLM:
    def __init__(self, payload: dict):
        self._payload = payload
        self.calls = 0

    async def extract_job_profile(self, *, job_description: str, prompt_version: str):
        self.calls += 1
        return dict(self._payload)


class _FakeRepo:
    def __init__(self):
        self.created = None

    async def create(self, profile):
        self.created = profile
        return profile

    async def find_by_job_id(self, job_id):
        return None


@pytest.mark.asyncio
async def test_execute_rejects_empty_job_description():
    repo = _FakeRepo()
    llm = _FakeLLM({"family": "technology", "seniority": "unknown"})
    svc = AnalyzeJob(repo=repo, llm=llm)

    with pytest.raises(ValueError, match="job_description is required"):
        await svc.execute(job_description="   ", job_id="job-1")


@pytest.mark.asyncio
async def test_execute_persists_profile():
    repo = _FakeRepo()
    llm = _FakeLLM(
        {
            "family": "technology",
            "seniority": "senior",
            "title": "Senior Backend Engineer",
            "requirements": [
                {
                    "original_name": "Golang",
                    "normalized_name": "golang",
                    "category": "technical_skills",
                    "importance": "required",
                }
            ],
        }
    )
    svc = AnalyzeJob(repo=repo, llm=llm)

    profile = await svc.execute(
        job_description="We need a Senior Backend Engineer with Golang.",
        job_id="job-1",
    )

    assert repo.created is not None
    assert repo.created.job_id == "job-1"
    assert profile.family == "technology"
    assert llm.calls == 1
