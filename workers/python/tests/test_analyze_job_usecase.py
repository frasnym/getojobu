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


class _FakeJobProfiles:
    def __init__(self):
        self.created = None
        self.idempotency_keys = []
        self.closed = False

    async def create_job_profile(
        self,
        *,
        job_description: str,
        job_id: str | None,
        profile: dict,
        idempotency_key: str | None = None,
    ) -> dict:
        self.created = {
            "job_description": job_description,
            "job_id": job_id,
            "profile": profile,
            "idempotency_key": idempotency_key,
        }
        self.idempotency_keys.append(idempotency_key)
        return {"id": "stored-id-123", **profile}

    async def aclose(self) -> None:
        self.closed = True


@pytest.mark.asyncio
async def test_execute_rejects_empty_job_description():
    job_profiles = _FakeJobProfiles()
    llm = _FakeLLM({"family": "technology", "seniority": "unknown"})
    svc = AnalyzeJob(job_profiles=job_profiles, llm=llm)

    with pytest.raises(ValueError, match="job_description is required"):
        await svc.execute(job_description="   ", job_id="job-1")

    assert job_profiles.created is None
    assert llm.calls == 0


@pytest.mark.asyncio
async def test_execute_calls_api_with_profile():
    job_profiles = _FakeJobProfiles()
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
    svc = AnalyzeJob(job_profiles=job_profiles, llm=llm)

    profile = await svc.execute(
        job_description="We need a Senior Backend Engineer with Golang.",
        job_id="job-1",
        idempotency_key="idem-key-abc",
    )

    assert job_profiles.created is not None
    assert job_profiles.created["job_id"] == "job-1"
    assert job_profiles.created["idempotency_key"] == "idem-key-abc"
    assert profile.family == "technology"
    assert profile.job_id == "job-1"
    assert llm.calls == 1


@pytest.mark.asyncio
async def test_execute_generates_idempotency_key_when_not_provided():
    job_profiles = _FakeJobProfiles()
    llm = _FakeLLM({"family": "technology", "seniority": "unknown"})
    svc = AnalyzeJob(job_profiles=job_profiles, llm=llm)

    await svc.execute(job_description="We need an engineer.")

    assert job_profiles.created is not None
    assert job_profiles.created["idempotency_key"] is not None
    assert len(job_profiles.created["idempotency_key"]) > 0
