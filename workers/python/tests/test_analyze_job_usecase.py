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
            "status": "queued",
            "description": "We need a Senior Backend Engineer with Golang.",
        }
        self.patched = None
        self.created = None
        self.triggered = None

    async def get_job(self, slug: str) -> dict:
        return dict(self.job_data)

    async def patch_job_status(self, slug: str, status: str, **kwargs) -> dict:
        self.patched = {"slug": slug, "status": status, **kwargs}
        return {"slug": slug, "status": status}

    async def create_evaluations(self, items: list[dict]) -> list[dict]:
        self.created = items
        return [
            {
                "id": "eval-1",
                "slug": "job-1",
                "user_id": "user-1",
                "evaluation_status": "new",
            }
        ]

    async def trigger_evaluations(self, items: list[dict]) -> list[dict]:
        self.triggered = items
        return [{"id": "eval-1", "evaluation_status": "queued"}]


@pytest.mark.asyncio
async def test_analyze_rejects_empty_job_description():
    fake_api = _FakeAPI(
        {"id": 1, "slug": "job-1", "status": "queued", "description": "   "}
    )
    llm = _FakeLLM({"family": "technology", "seniority": "unknown"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    with pytest.raises(ValueError, match="has no description to analyze"):
        await svc.analyze(slug="job-1")

    assert llm.calls == 0


@pytest.mark.asyncio
async def test_analyze_rejects_new_status():
    fake_api = _FakeAPI(
        {"id": 1, "slug": "job-1", "status": "new", "description": "Some description"}
    )
    llm = _FakeLLM({"family": "technology", "seniority": "unknown"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    with pytest.raises(ValueError, match="must be 'queued' or 'queued_retry'"):
        await svc.analyze(slug="job-1")

    assert llm.calls == 0


@pytest.mark.asyncio
async def test_analyze_returns_parsed_analysis():
    fake_api = _FakeAPI(
        {
            "id": 1,
            "slug": "job-1",
            "status": "queued",
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
            "status": "queued",
            "description": "We need an engineer.",
        }
    )
    llm = _FakeLLM({"family": "technology", "seniority": "mid"})
    svc = AnalyzeJob(
        api=fake_api, llm=llm, model="use-combo", prompt_version="extract-job-v1"
    )

    analysis = await svc.analyze(slug="job-42")

    assert analysis.metadata["model"] == "use-combo"
    assert analysis.metadata["prompt_version"] == "extract-job-v1"
    assert llm.calls == 1


@pytest.mark.asyncio
async def test_analyze_metadata_empty_when_unset():
    fake_api = _FakeAPI(
        {
            "id": 42,
            "slug": "job-42",
            "status": "queued",
            "description": "We need an engineer.",
        }
    )
    llm = _FakeLLM({"family": "technology", "seniority": "mid"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    analysis = await svc.analyze(slug="job-42")

    assert analysis.metadata["model"] is None
    assert analysis.metadata["prompt_version"] == "extract-job-v1"


class _ListReturningLLM:
    """Fake LLM that returns a JSON array instead of an object."""

    async def extract_job_analysis(self, *, job_description: str, prompt_version: str):
        return [{"original_name": "Bachelor's Degree Learning Familiarity"}]


@pytest.mark.asyncio
async def test_analyze_rejects_llm_returning_a_list():
    """An LLM that returns a JSON array instead of an object must be rejected,
    and the job should transition to 'failed'."""
    fake_api = _FakeAPI(
        {
            "id": 1,
            "slug": "job-1",
            "status": "queued",
            "description": "We need a Senior Backend Engineer with Golang.",
        }
    )
    svc = AnalyzeJob(api=fake_api, llm=_ListReturningLLM())

    with pytest.raises(ValueError, match="instead of a JSON object"):
        await svc.analyze(slug="job-1")

    assert fake_api.patched is not None
    assert fake_api.patched["status"] == "failed"
    assert "instead of a JSON object" in fake_api.patched["failure_message"]


@pytest.mark.asyncio
async def test_analyze_inserts_then_triggers_evaluations():
    fake_api = _FakeAPI()
    llm = _FakeLLM({"family": "technology", "seniority": "senior"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    await svc.analyze(slug="job-1")

    assert fake_api.created == [{"slug": "job-1"}]
    assert fake_api.triggered == [{"id": "eval-1", "user_id": "user-1"}]


@pytest.mark.asyncio
async def test_analyze_propagates_trigger_failure():
    class _FailingTriggerAPI(_FakeAPI):
        async def trigger_evaluations(self, items: list[dict]) -> list[dict]:
            self.triggered = items
            raise RuntimeError("trigger unavailable")

    fake_api = _FailingTriggerAPI()
    llm = _FakeLLM({"family": "technology", "seniority": "senior"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    with pytest.raises(RuntimeError, match="trigger unavailable"):
        await svc.analyze(slug="job-1")

    assert fake_api.created == [{"slug": "job-1"}]
    assert fake_api.triggered == [{"id": "eval-1", "user_id": "user-1"}]
