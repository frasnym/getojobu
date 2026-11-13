"""Tests for Step 2 MatchJobWorkflow and its Temporal activities."""

from typing import Any
from unittest.mock import AsyncMock, MagicMock

import pytest
from temporalio import activity

from deliveries.consumer.activities.evaluate_match import (
    evaluate_match_activity,
    init_evaluate_match_container,
    update_evaluation_status_activity,
)
from models.evaluation import CategoryEvaluation, MatchEvaluationOutput
from workflows.match_job import MatchJobWorkflow


@pytest.mark.asyncio
async def test_evaluate_match_activity_success():
    fake_eval = MatchEvaluationOutput(
        evaluations=[
            CategoryEvaluation(
                category="technical_skills",
                score=90,
                reason="Strong fit",
            )
        ],
        mandatory_requirements=[],
    )

    mock_container = MagicMock()
    mock_container.usecase.evaluate_match.evaluate = AsyncMock(return_value=fake_eval)
    init_evaluate_match_container(mock_container)

    res = await evaluate_match_activity(
        {
            "candidate_profile": {"skills": ["golang"]},
            "job_profile": {"title": "Go Engineer"},
            "evaluation_config": {"family": "technology"},
        }
    )

    assert "evaluations" in res
    assert res["evaluations"][0]["score"] == 90
    mock_container.usecase.evaluate_match.evaluate.assert_awaited_once()


@pytest.mark.asyncio
async def test_evaluate_match_activity_missing_inputs():
    with pytest.raises(ValueError, match="candidate_profile is required"):
        await evaluate_match_activity(
            {
                "candidate_profile": {},
                "job_profile": {"title": "Go Engineer"},
                "evaluation_config": {"family": "technology"},
            }
        )


@pytest.mark.asyncio
async def test_evaluate_match_activity_uninitialized_usecase():
    mock_container = MagicMock()
    mock_container.usecase.evaluate_match = None
    init_evaluate_match_container(mock_container)

    with pytest.raises(RuntimeError, match="evaluate_match usecase is not initialized"):
        await evaluate_match_activity(
            {
                "candidate_profile": {"skills": ["golang"]},
                "job_profile": {"title": "Go Engineer"},
                "evaluation_config": {"family": "technology"},
            }
        )


@pytest.mark.asyncio
async def test_update_evaluation_status_activity_success():
    mock_container = MagicMock()
    mock_container.repository.getojobu_api.patch_matching_evaluation = AsyncMock(
        return_value={"id": "eval-1", "status": "pending"}
    )
    init_evaluate_match_container(mock_container)

    res = await update_evaluation_status_activity(
        {
            "evaluation_id": "eval-1",
            "status": "pending",
        }
    )

    assert res["status"] == "pending"
    mock_container.repository.getojobu_api.patch_matching_evaluation.assert_awaited_once()


@pytest.mark.asyncio
async def test_match_job_workflow_execution():
    from temporalio.testing import WorkflowEnvironment
    from temporalio.worker import Worker

    try:
        env = await WorkflowEnvironment.start_time_skipping()
    except Exception as exc:  # noqa: BLE001
        pytest.skip(f"Ephemeral temporal server unavailable: {exc}")
        return

    async with env:
        task_queue = "test-match-queue"

        @activity.defn(name="evaluate_match_activity")
        async def fake_evaluate_match(input: dict[str, Any]) -> dict[str, Any]:
            return {
                "evaluations": [
                    {"category": "technical_skills", "score": 88, "reason": "ok"}
                ],
                "mandatory_requirements": [],
            }

        status_updates = []

        @activity.defn(name="update_evaluation_status_activity")
        async def fake_update_status(input: dict[str, Any]) -> dict[str, Any]:
            status_updates.append(input.get("status"))
            return {"status": input.get("status")}

        async with Worker(
            env.client,
            task_queue=task_queue,
            workflows=[MatchJobWorkflow],
            activities=[fake_evaluate_match, fake_update_status],
        ):
            handle = await env.client.start_workflow(
                MatchJobWorkflow.run,
                {
                    "evaluation_id": "eval-123",
                    "slug": "test-job",
                    "candidate_profile": {"skills": ["go"]},
                    "job_profile": {"family": "technology"},
                    "evaluation_config": {"family": "technology"},
                },
                id="test-match-wf-1",
                task_queue=task_queue,
            )
            result = await handle.result()

            assert result["status"] == "success"
            assert result["evaluation_id"] == "eval-123"
            # Verify status updates transition through pending then success
            assert status_updates == ["pending", "success"]


@pytest.mark.asyncio
async def test_match_job_workflow_marks_failed_when_step_3_fails():
    from temporalio.client import WorkflowFailureError
    from temporalio.testing import WorkflowEnvironment
    from temporalio.worker import Worker

    try:
        env = await WorkflowEnvironment.start_time_skipping()
    except Exception as exc:  # noqa: BLE001
        pytest.skip(f"Ephemeral temporal server unavailable: {exc}")
        return

    async with env:
        task_queue = "test-match-queue-fail-3"

        @activity.defn(name="evaluate_match_activity")
        async def fake_evaluate_match(input: dict[str, Any]) -> dict[str, Any]:
            return {
                "evaluations": [
                    {"category": "technical_skills", "score": 88, "reason": "ok"}
                ],
                "mandatory_requirements": [],
            }

        status_updates = []

        @activity.defn(name="update_evaluation_status_activity")
        async def fake_update_status(input: dict[str, Any]) -> dict[str, Any]:
            status = input.get("status")
            status_updates.append(status)
            if status == "success":
                raise RuntimeError("Go backend rejected invalid evaluation data")
            return {"status": status}

        async with Worker(
            env.client,
            task_queue=task_queue,
            workflows=[MatchJobWorkflow],
            activities=[fake_evaluate_match, fake_update_status],
        ):
            handle = await env.client.start_workflow(
                MatchJobWorkflow.run,
                {
                    "evaluation_id": "eval-fail-3",
                    "slug": "test-job",
                    "candidate_profile": {"skills": ["go"]},
                    "job_profile": {"family": "technology"},
                    "evaluation_config": {"family": "technology"},
                },
                id="test-match-wf-fail-3",
                task_queue=task_queue,
            )
            with pytest.raises(WorkflowFailureError):
                await handle.result()

            # Status transitions: pending -> success (fails) -> failed
            assert "failed" in status_updates
