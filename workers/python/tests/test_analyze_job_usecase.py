"""Tests for the analyze_job usecase validation (Step 1B business logic)."""

import pytest

from usecase.analyze_job.service import AnalyzeJob


class _FakeLLM:
    def __init__(self, payload: dict):
        self._payload = payload
        self.calls = 0

    async def extract_job_analysis(self, *, job_description: str, prompt_version: str):
        self.calls += 1
        return dict(self._payload)


class _FakeAPI:
    def __init__(self, job_data: dict | None = None):
        self.job_data = job_data or {
            "id": 1,
            "slug": "job-1",
            "status": "new",
            "description": "We need a Senior Backend Engineer with Golang.",
        }
        self.patched = None

    async def get_job(self, slug: str) -> dict:
        return dict(self.job_data)

    async def patch_job_status(self, slug: str, status: str, **kwargs) -> dict:
        self.patched = {"slug": slug, "status": status, **kwargs}
        return {"slug": slug, "status": status}


@pytest.mark.asyncio
async def test_analyze_rejects_empty_job_description():
    fake_api = _FakeAPI(
        {"id": 1, "slug": "job-1", "status": "new", "description": "   "}
    )
    llm = _FakeLLM({"family": "technology", "seniority": "unknown"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    with pytest.raises(ValueError, match="has no description to analyze"):
        await svc.analyze(slug="job-1")

    assert llm.calls == 0


@pytest.mark.asyncio
async def test_analyze_returns_parsed_analysis():
    fake_api = _FakeAPI(
        {
            "id": 1,
            "slug": "job-1",
            "status": "new",
            "description": "We need a Senior Backend Engineer with Golang.",
        }
    )
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
    svc = AnalyzeJob(api=fake_api, llm=llm)

    analysis = await svc.analyze(slug="job-1")

    assert analysis.family == "technology"
    assert analysis.seniority == "senior"
    assert analysis.title == "Senior Backend Engineer"
    assert analysis.job_id == "1"
    assert len(analysis.requirements) == 1
    assert analysis.requirements[0].original_name == "Golang"
    assert analysis.requirements[0].importance == "required"
    assert llm.calls == 1


@pytest.mark.asyncio
async def test_analyze_sets_metadata_defaults():
    fake_api = _FakeAPI(
        {
            "id": 42,
            "slug": "job-42",
            "status": "new",
            "description": "We need an engineer.",
        }
    )
    llm = _FakeLLM({"family": "technology", "seniority": "mid"})
    svc = AnalyzeJob(
        api=fake_api, llm=llm, model="use-combo", prompt_version="extract-job-v1"
    )

    analysis = await svc.analyze(slug="job-42")

    assert analysis.job_id == "42"
    assert analysis.model == "use-combo"
    assert analysis.prompt_version == "extract-job-v1"
    assert llm.calls == 1


@pytest.mark.asyncio
async def test_analyze_metadata_empty_when_unset():
    fake_api = _FakeAPI(
        {
            "id": 42,
            "slug": "job-42",
            "status": "new",
            "description": "We need an engineer.",
        }
    )
    llm = _FakeLLM({"family": "technology", "seniority": "mid"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    analysis = await svc.analyze(slug="job-42")

    assert analysis.job_id == "42"
    assert analysis.model is None
    assert analysis.prompt_version == "extract-job-v1"
