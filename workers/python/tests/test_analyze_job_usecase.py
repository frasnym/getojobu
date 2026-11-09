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


class _FakeAPIClient:
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
    api = _FakeAPIClient()
    llm = _FakeLLM({"family": "technology", "seniority": "unknown"})
    svc = AnalyzeJob(api_client=api, llm=llm)

    with pytest.raises(ValueError, match="job_description is required"):
        await svc.execute(job_description="   ", job_id="job-1")

    assert api.created is None
    assert llm.calls == 0


@pytest.mark.asyncio
async def test_execute_calls_api_with_profile():
    api = _FakeAPIClient()
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
    svc = AnalyzeJob(api_client=api, llm=llm)

    profile = await svc.execute(
        job_description="We need a Senior Backend Engineer with Golang.",
        job_id="job-1",
        idempotency_key="idem-key-abc",
    )

    assert api.created is not None
    assert api.created["job_id"] == "job-1"
    assert api.created["idempotency_key"] == "idem-key-abc"
    assert profile.family == "technology"
    assert profile.job_id == "job-1"
    assert llm.calls == 1


@pytest.mark.asyncio
async def test_execute_generates_idempotency_key_when_not_provided():
    api = _FakeAPIClient()
    llm = _FakeLLM({"family": "technology", "seniority": "unknown"})
    svc = AnalyzeJob(api_client=api, llm=llm)

    await svc.execute(job_description="We need an engineer.")

    assert api.created is not None
    assert api.created["idempotency_key"] is not None
    assert len(api.created["idempotency_key"]) > 0
