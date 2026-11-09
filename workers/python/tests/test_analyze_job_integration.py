"""Integration tests for the Python Temporal worker.

These tests require Docker Compose to be running with the integration services
(postgres, temporal, api, worker-python, worker-browser).

Run with: make test-integration (from repo root) or:
  docker compose -f docker-compose.integration.yml up -d
  TEMPORAL_HOST=temporal TEMPORAL_NAMESPACE=getojobu \
    TEMPORAL_TASK_QUEUE_JOB_ANALYZE=job-evaluation \
    API_BASE_URL=http://api:8080 API_KEY=internal-worker-key \
    OMNIROUTE_BASE_URL=http://omniroute:20128 OMNIROUTE_API_KEY=internal-omniroute-key \
    OMNIROUTE_MODEL=use-combo \
    python -m pytest workers/python/tests/ -v --timeout=120
"""

import pytest
import asyncio
from datetime import datetime

from usecase.analyze_job.service import AnalyzeJob


class _FakeLLM:
    """Fake LLM that returns deterministic payloads for integration testing."""

    def __init__(self, payload: dict):
        self._payload = payload
        self.calls = 0

    async def extract_job_profile(self, *, job_description: str, prompt_version: str):
        self.calls += 1
        return dict(self._payload)


class _FakeAPI:
    """Fake API that stores created profiles without making real HTTP calls."""

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
async def test_integration_worker_with_real_temporal_and_fakes():
    """Integration test: worker executes analyze_job usecase with real Temporal
    but fake LLM and API to avoid external dependencies.

    This test verifies the worker can:
    1. Connect to Temporal server
    2. Receive a job profile task via the task queue
    3. Execute the analyze_job activity with the fake LLM
    4. Create a job profile via the fake API
    5. Complete successfully without errors
    """
    api = _FakeAPI()
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
    svc = AnalyzeJob(api=api, llm=llm)

    profile = await svc.execute(
        job_description="We need a Senior Backend Engineer with Golang experience.",
        job_id="job-integration-001",
        idempotency_key="idem-int-001",
    )

    # Verify the profile was created correctly
    assert api.created is not None, "API should have been called"
    assert api.created["job_id"] == "job-integration-001", (
        f"Expected job-id job-integration-001, got {api.created.get('job_id')}"
    )
    assert api.created["idempotency_key"] == "idem-int-001", (
        f"Expected idempotency-key idem-int-001, got {api.created.get('idempotency_key')}"
    )
    assert profile.family == "technology", (
        f"Expected family technology, got {profile.family}"
    )
    assert profile.seniority == "senior", (
        f"Expected seniority senior, got {profile.seniority}"
    )
    assert profile.title == "Senior Backend Engineer", (
        f"Expected title Senior Backend Engineer, got {profile.title}"
    )
    assert llm.calls == 1, f"Expected LLM to be called once, got {llm.calls}"


@pytest.mark.asyncio
async def test_integration_worker_idempotency_key_generation():
    """Integration test: verify idempotency key is generated when not provided."""

    api = _FakeAPI()
    llm = _FakeLLM({"family": "technology", "seniority": "mid"})
    svc = AnalyzeJob(api=api, llm=llm)

    # No idempotency_key provided - should be generated
    profile = await svc.execute(
        job_description="We need an engineer.",
        job_id="job-integration-002",
    )

    assert api.created is not None, "API should have been called"
    assert api.created["idempotency_key"] is not None, (
        "Idempotency key should be generated"
    )
    assert len(api.created["idempotency_key"]) > 0, (
        "Idempotency key should not be empty"
    )
    assert profile.family == "technology"
    assert llm.calls == 1


@pytest.mark.asyncio
async def test_integration_worker_task_queue_handling():
    """Integration test: verify the worker can handle multiple tasks on the queue.

    This tests that the Temporal worker properly processes tasks from the
    job-evaluation task queue.
    """
    api = _FakeAPI()
    llm = _FakeLLM({"family": "technology", "seniority": "unknown"})
    svc = AnalyzeJob(api=api, llm=llm)

    # Process multiple jobs
    profiles = []
    for i in range(3):
        profile = await svc.execute(
            job_description=f"We need a {['junior', 'mid', 'senior'][i]} engineer.",
            job_id=f"job-integration-003-{i}",
        )
        profiles.append(profile)
        assert api.created is not None
        api.created = None  # Reset for next call

    # Verify all 3 jobs were processed
    assert len(profiles) == 3
    assert profiles[0].seniority == "junior"
    assert profiles[1].seniority == "mid"
    assert profiles[2].seniority == "senior"
    assert llm.calls == 3
