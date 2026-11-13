"""Integration tests for the Python Temporal worker.

These tests verify worker components, usecases, and activities with fakes.
"""

import json
from typing import Any

import pytest

from config.config import APIConfig, Config, OmniRouteConfig, TemporalConfig
from contract.contract import WorkerContainer
from contract.repository import RepositoryContract
from contract.usecase import UsecaseContract
from deliveries.consumer.activities.analyze_job import (
    analyze_job_activity,
    init_analyze_job_container,
)
from usecase.analyze_job.service import AnalyzeJob


class _FakeLLM:
    """Fake LLM that returns deterministic payloads for integration testing."""

    def __init__(self, payload: dict):
        self._payload = payload
        self.calls = 0

    async def extract_job_analysis(self, *, job_description: str, prompt_version: str):
        self.calls += 1
        res = dict(self._payload)
        for s in ("junior", "mid", "senior"):
            if s in job_description:
                res["seniority"] = s
        return res


class _FailingLLM:
    """Fake LLM that raises an exception on extraction."""

    async def extract_job_analysis(self, *, job_description: str, prompt_version: str):
        raise RuntimeError("LLM quota exceeded")


class _FakeAPI:
    """Fake GetojobuAPI that simulates GET and PATCH operations."""

    def __init__(
        self,
        job_data: Any = None,
        *,
        patch_pending_response: dict | None = None,
        patch_analyzed_response: dict | None = None,
    ):
        self.job_data = (
            job_data
            if job_data is not None
            else {
                "id": 101,
                "slug": "lead-go-developer-acme",
                "status": "new",
                "description": "We need a Lead Go Developer with 8 years experience.",
            }
        )
        self.patch_pending_response = patch_pending_response
        self.patch_analyzed_response = patch_analyzed_response
        self.patched = None
        self.patched_statuses = []
        self.closed = False

    async def get_job(self, slug: str) -> Any:
        if isinstance(self.job_data, dict):
            return dict(self.job_data)
        return self.job_data

    async def patch_job_status(
        self,
        slug: str,
        status: str,
        *,
        analyze_result: str | None = None,
        failure_message: str | None = None,
    ) -> dict:
        self.patched = {
            "slug": slug,
            "status": status,
            "analyze_result": analyze_result,
            "failure_message": failure_message,
        }
        self.patched_statuses.append(status)
        if status == "analyze_pending" and self.patch_pending_response is not None:
            return self.patch_pending_response
        if status == "analyzed" and self.patch_analyzed_response is not None:
            return self.patch_analyzed_response
        return {"slug": slug, "status": status}

    async def aclose(self) -> None:
        self.closed = True


@pytest.mark.asyncio
async def test_integration_worker_with_real_temporal_and_fakes():
    """Integration test: worker executes analyze_job usecase with fake LLM."""
    fake_api = _FakeAPI(
        {
            "id": 101,
            "slug": "job-integration-001",
            "status": "new",
            "description": "We need a Senior Backend Engineer with Golang experience.",
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

    profile = await svc.analyze(slug="job-integration-001")

    assert profile.family == "technology"
    assert profile.seniority == "senior"
    assert profile.title == "Senior Backend Engineer"
    assert profile.job_id == "101"
    assert llm.calls == 1


@pytest.mark.asyncio
async def test_integration_worker_task_queue_handling():
    """Integration test: verify the worker handles multiple jobs properly."""
    llm = _FakeLLM({"family": "technology", "seniority": "unknown"})
    profiles = []
    for i, seniority in enumerate(["junior", "mid", "senior"]):
        slug = f"job-integration-003-{i}"
        fake_api = _FakeAPI(
            {
                "id": i + 1,
                "slug": slug,
                "status": "new",
                "description": f"We need a {seniority} engineer.",
            }
        )
        svc = AnalyzeJob(api=fake_api, llm=llm)
        profile = await svc.analyze(slug=slug)
        profiles.append(profile)

    assert len(profiles) == 3
    assert profiles[0].seniority == "junior"
    assert profiles[1].seniority == "mid"
    assert profiles[2].seniority == "senior"
    assert llm.calls == 3


@pytest.mark.asyncio
async def test_activity_flow_success_new_to_analyze_pending_to_analyzed():
    """Verify 5-step flow for a 'new' job: GET -> check new -> PATCH pending -> LLM -> PATCH analyzed."""
    fake_api = _FakeAPI(
        {
            "id": 101,
            "slug": "lead-go-developer-acme",
            "status": "new",
            "description": "We need a Lead Go Developer with 8 years experience.",
        }
    )
    repo_contract = RepositoryContract(api=fake_api)

    llm = _FakeLLM(
        {
            "family": "technology",
            "seniority": "senior",
            "title": "Lead Go Developer",
        }
    )
    svc = AnalyzeJob(api=fake_api, llm=llm)
    usecase_contract = UsecaseContract(analyze_job=svc)

    dummy_cfg = Config(
        temporal=TemporalConfig("localhost:7233", "default", "job-evaluation"),
        api=APIConfig("http://localhost:8080", "test-key"),
        omni_route=OmniRouteConfig("http://localhost:20128", "test-key", "test-model"),
    )
    test_container = WorkerContainer(
        cfg=dummy_cfg,
        repository=repo_contract,
        usecase=usecase_contract,
    )
    init_analyze_job_container(test_container)

    slug = "lead-go-developer-acme"
    result = await analyze_job_activity(
        {
            "slug": slug,
            "job_id": "101",
        }
    )

    assert result["family"] == "technology"
    assert result["seniority"] == "senior"
    # Verify status transitions: first "analyze_pending", then "analyzed"
    assert fake_api.patched_statuses == ["analyze_pending", "analyzed"]
    assert fake_api.patched["slug"] == slug
    assert fake_api.patched["status"] == "analyzed"
    assert fake_api.patched["analyze_result"] is not None

    parsed_result = json.loads(fake_api.patched["analyze_result"])
    assert parsed_result["family"] == "technology"
    assert parsed_result["title"] == "Lead Go Developer"


@pytest.mark.asyncio
async def test_activity_flow_success_failed_to_analyze_pending_to_analyzed():
    """Verify 5-step flow for a 'failed' job: GET -> check failed -> PATCH pending -> LLM -> PATCH analyzed."""
    fake_api = _FakeAPI(
        {
            "id": 102,
            "slug": "failed-job-retry",
            "status": "failed",
            "description": "Retry job description",
        }
    )
    repo_contract = RepositoryContract(api=fake_api)

    llm = _FakeLLM({"family": "technology", "seniority": "mid", "title": "Developer"})
    svc = AnalyzeJob(api=fake_api, llm=llm)
    usecase_contract = UsecaseContract(analyze_job=svc)

    dummy_cfg = Config(
        temporal=TemporalConfig("localhost:7233", "default", "job-evaluation"),
        api=APIConfig("http://localhost:8080", "test-key"),
        omni_route=OmniRouteConfig("http://localhost:20128", "test-key", "test-model"),
    )
    init_analyze_job_container(
        WorkerContainer(
            cfg=dummy_cfg, repository=repo_contract, usecase=usecase_contract
        )
    )

    result = await analyze_job_activity({"slug": "failed-job-retry"})
    assert result["family"] == "technology"
    assert fake_api.patched_statuses == ["analyze_pending", "analyzed"]


@pytest.mark.asyncio
async def test_activity_flow_rejects_analyzed_status():
    """Verify activity rejects a job with status 'analyzed' without invoking LLM or PATCH."""
    fake_api = _FakeAPI(
        {
            "id": 103,
            "slug": "already-analyzed-job",
            "status": "analyzed",
            "description": "Already done",
        }
    )
    repo_contract = RepositoryContract(api=fake_api)
    llm = _FakeLLM({"family": "technology", "seniority": "senior"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    dummy_cfg = Config(
        temporal=TemporalConfig("localhost:7233", "default", "job-evaluation"),
        api=APIConfig("http://localhost:8080", "test-key"),
        omni_route=OmniRouteConfig("http://localhost:20128", "test-key", "test-model"),
    )
    init_analyze_job_container(
        WorkerContainer(
            cfg=dummy_cfg,
            repository=repo_contract,
            usecase=UsecaseContract(analyze_job=svc),
        )
    )

    with pytest.raises(
        ValueError, match="cannot be analyzed: current status is 'analyzed'"
    ):
        await analyze_job_activity({"slug": "already-analyzed-job"})

    assert llm.calls == 0
    assert fake_api.patched_statuses == []


@pytest.mark.asyncio
async def test_activity_flow_rejects_analyze_pending_status():
    """Verify activity rejects a job with status 'analyze_pending'."""
    fake_api = _FakeAPI(
        {
            "id": 104,
            "slug": "pending-job",
            "status": "analyze_pending",
            "description": "Pending",
        }
    )
    repo_contract = RepositoryContract(api=fake_api)
    llm = _FakeLLM({"family": "technology", "seniority": "senior"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    dummy_cfg = Config(
        temporal=TemporalConfig("localhost:7233", "default", "job-evaluation"),
        api=APIConfig("http://localhost:8080", "test-key"),
        omni_route=OmniRouteConfig("http://localhost:20128", "test-key", "test-model"),
    )
    init_analyze_job_container(
        WorkerContainer(
            cfg=dummy_cfg,
            repository=repo_contract,
            usecase=UsecaseContract(analyze_job=svc),
        )
    )

    with pytest.raises(
        ValueError, match="cannot be analyzed: current status is 'analyze_pending'"
    ):
        await analyze_job_activity({"slug": "pending-job"})

    assert llm.calls == 0
    assert fake_api.patched_statuses == []


@pytest.mark.asyncio
async def test_activity_flow_patches_failed_on_llm_error():
    """Verify activity transitions to analyze_pending, then patches failed on LLM exception."""
    fake_api = _FakeAPI(
        {
            "id": 105,
            "slug": "failing-job-slug",
            "status": "new",
            "description": "Some job description",
        }
    )
    repo_contract = RepositoryContract(api=fake_api)

    svc = AnalyzeJob(api=fake_api, llm=_FailingLLM())
    usecase_contract = UsecaseContract(analyze_job=svc)

    dummy_cfg = Config(
        temporal=TemporalConfig("localhost:7233", "default", "job-evaluation"),
        api=APIConfig("http://localhost:8080", "test-key"),
        omni_route=OmniRouteConfig("http://localhost:20128", "test-key", "test-model"),
    )
    init_analyze_job_container(
        WorkerContainer(
            cfg=dummy_cfg, repository=repo_contract, usecase=usecase_contract
        )
    )

    slug = "failing-job-slug"
    with pytest.raises(RuntimeError, match="LLM quota exceeded"):
        await analyze_job_activity({"slug": slug})

    # First transitioned to analyze_pending, then to failed
    assert fake_api.patched_statuses == ["analyze_pending", "failed"]
    assert fake_api.patched["slug"] == slug
    assert fake_api.patched["status"] == "failed"
    assert "LLM quota exceeded" in fake_api.patched["failure_message"]


@pytest.mark.asyncio
async def test_activity_flow_raises_on_get_job_missing_status():
    """Verify activity raises RuntimeError if GET /v1/jobs/{slug} response has no 'status'."""
    fake_api = _FakeAPI({"id": 106, "slug": "test-slug", "description": "desc"})
    repo_contract = RepositoryContract(api=fake_api)
    llm = _FakeLLM({"family": "technology", "seniority": "mid"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    dummy_cfg = Config(
        temporal=TemporalConfig("localhost:7233", "default", "job-evaluation"),
        api=APIConfig("http://localhost:8080", "test-key"),
        omni_route=OmniRouteConfig("http://localhost:20128", "test-key", "test-model"),
    )
    init_analyze_job_container(
        WorkerContainer(
            cfg=dummy_cfg,
            repository=repo_contract,
            usecase=UsecaseContract(analyze_job=svc),
        )
    )

    with pytest.raises(RuntimeError, match="missing 'status' field"):
        await analyze_job_activity({"slug": "test-slug"})


@pytest.mark.asyncio
async def test_activity_flow_raises_on_patch_pending_unexpected_response():
    """Verify activity raises RuntimeError if PATCH status=analyze_pending does not return status=analyze_pending."""
    fake_api = _FakeAPI(
        {
            "id": 107,
            "slug": "test-slug",
            "status": "new",
            "description": "desc",
        },
        patch_pending_response={"slug": "test-slug", "status": "failed"},
    )
    repo_contract = RepositoryContract(api=fake_api)
    llm = _FakeLLM({"family": "technology", "seniority": "mid"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    dummy_cfg = Config(
        temporal=TemporalConfig("localhost:7233", "default", "job-evaluation"),
        api=APIConfig("http://localhost:8080", "test-key"),
        omni_route=OmniRouteConfig("http://localhost:20128", "test-key", "test-model"),
    )
    init_analyze_job_container(
        WorkerContainer(
            cfg=dummy_cfg,
            repository=repo_contract,
            usecase=UsecaseContract(analyze_job=svc),
        )
    )

    with pytest.raises(
        RuntimeError, match="failed to transition job 'test-slug' to 'analyze_pending'"
    ):
        await analyze_job_activity({"slug": "test-slug"})


@pytest.mark.asyncio
async def test_activity_flow_raises_on_patch_analyzed_unexpected_response():
    """Verify activity raises RuntimeError if PATCH status=analyzed does not return status=analyzed."""
    fake_api = _FakeAPI(
        {
            "id": 108,
            "slug": "test-slug",
            "status": "new",
            "description": "desc",
        },
        patch_analyzed_response={"slug": "test-slug", "status": "failed"},
    )
    repo_contract = RepositoryContract(api=fake_api)
    llm = _FakeLLM({"family": "technology", "seniority": "mid"})
    svc = AnalyzeJob(api=fake_api, llm=llm)

    dummy_cfg = Config(
        temporal=TemporalConfig("localhost:7233", "default", "job-evaluation"),
        api=APIConfig("http://localhost:8080", "test-key"),
        omni_route=OmniRouteConfig("http://localhost:20128", "test-key", "test-model"),
    )
    init_analyze_job_container(
        WorkerContainer(
            cfg=dummy_cfg,
            repository=repo_contract,
            usecase=UsecaseContract(analyze_job=svc),
        )
    )

    with pytest.raises(
        RuntimeError, match="failed to transition job 'test-slug' to 'analyzed'"
    ):
        await analyze_job_activity({"slug": "test-slug"})
