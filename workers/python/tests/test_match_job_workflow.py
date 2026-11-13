"""Tests for Step 2 MatchJobWorkflow and its Temporal activities."""

from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch

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
    mock_container.cfg.omni_route.model = "use-combo"
    init_evaluate_match_container(mock_container)

    res = await update_evaluation_status_activity(
        {
            "evaluation_id": "eval-1",
            "status": "pending",
        }
    )

    assert res["status"] == "pending"
    mock_container.repository.getojobu_api.patch_matching_evaluation.assert_awaited_once_with(
        evaluation_id="eval-1",
        status="pending",
        evaluation_data=None,
        failure_message=None,
        provider=None,
        model="use-combo",
    )


@pytest.mark.asyncio
async def test_update_evaluation_status_activity_explicit_model():
    mock_container = MagicMock()
    mock_container.repository.getojobu_api.patch_matching_evaluation = AsyncMock(
        return_value={"id": "eval-1", "status": "success"}
    )
    mock_container.cfg.omni_route.model = "use-combo"
    init_evaluate_match_container(mock_container)

    res = await update_evaluation_status_activity(
        {
            "evaluation_id": "eval-1",
            "status": "success",
            "model": "custom-model",
            "provider": "custom-provider",
        }
    )

    assert res["status"] == "success"
    mock_container.repository.getojobu_api.patch_matching_evaluation.assert_awaited_once_with(
        evaluation_id="eval-1",
        status="success",
        evaluation_data=None,
        failure_message=None,
        provider="custom-provider",
        model="custom-model",
    )


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
        report_status_updates = []

        @activity.defn(name="update_evaluation_status_activity")
        async def fake_update_status(input: dict[str, Any]) -> dict[str, Any]:
            status_updates.append(input.get("status"))
            return {
                "status": input.get("status"),
                "overall_score": 88.0,
                "eligibility_status": "eligible",
                "recommendation_label": "strong_match",
            }

        @activity.defn(name="generate_report_activity")
        async def fake_generate_report(input: dict[str, Any]) -> dict[str, Any]:
            return {
                "job_title": "Go Engineer",
                "summary": "Great match",
                "strengths": ["Go"],
                "gaps": [],
                "experience_assessment": "Good",
                "cv_improvements": [],
                "learning_recommendations": [],
                "interview_preparation": [],
                "report_prompt_version": "generate-report-v1",
                "model": "model-1",
                "provider": "omniroute",
                "recommendation_policy_version": "deterministic-v1",
            }

        @activity.defn(name="update_report_status_activity")
        async def fake_update_report_status(input: dict[str, Any]) -> dict[str, Any]:
            report_status_updates.append(input.get("report_status"))
            return {"report_status": input.get("report_status")}

        async with Worker(
            env.client,
            task_queue=task_queue,
            workflows=[MatchJobWorkflow],
            activities=[
                fake_evaluate_match,
                fake_update_status,
                fake_generate_report,
                fake_update_report_status,
            ],
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
            # Verify report status updates transition through pending then generated
            assert report_status_updates == ["pending", "generated"]
            assert result["report_status"] == "generated"
            assert result["report"]["job_title"] == "Go Engineer"


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


@pytest.mark.asyncio
async def test_match_job_workflow_completes_when_report_step_fails():
    from temporalio.testing import WorkflowEnvironment
    from temporalio.worker import Worker

    try:
        env = await WorkflowEnvironment.start_time_skipping()
    except Exception as exc:  # noqa: BLE001
        pytest.skip(f"Ephemeral temporal server unavailable: {exc}")
        return

    async with env:
        task_queue = "test-match-queue-report-fail"

        @activity.defn(name="evaluate_match_activity")
        async def fake_evaluate_match(input: dict[str, Any]) -> dict[str, Any]:
            return {
                "evaluations": [
                    {"category": "technical_skills", "score": 88, "reason": "ok"}
                ],
                "mandatory_requirements": [],
            }

        @activity.defn(name="update_evaluation_status_activity")
        async def fake_update_status(input: dict[str, Any]) -> dict[str, Any]:
            return {
                "status": input.get("status"),
                "overall_score": 88.0,
                "eligibility_status": "eligible",
                "recommendation_label": "strong_match",
            }

        @activity.defn(name="generate_report_activity")
        async def fake_generate_report_fail(input: dict[str, Any]) -> dict[str, Any]:
            raise RuntimeError("LLM report generation timed out")

        report_status_updates = []

        @activity.defn(name="update_report_status_activity")
        async def fake_update_report_status(input: dict[str, Any]) -> dict[str, Any]:
            report_status_updates.append(input.get("report_status"))
            return {"report_status": input.get("report_status")}

        async with Worker(
            env.client,
            task_queue=task_queue,
            workflows=[MatchJobWorkflow],
            activities=[
                fake_evaluate_match,
                fake_update_status,
                fake_generate_report_fail,
                fake_update_report_status,
            ],
        ):
            handle = await env.client.start_workflow(
                MatchJobWorkflow.run,
                {
                    "evaluation_id": "eval-report-fail",
                    "slug": "test-job",
                    "candidate_profile": {"skills": ["go"]},
                    "job_profile": {"family": "technology"},
                    "evaluation_config": {"family": "technology"},
                },
                id="test-match-wf-report-fail",
                task_queue=task_queue,
            )
            result = await handle.result()

            assert result["status"] == "success"
            assert result["report_status"] == "failed"
            assert "timed out" in result["report_error"]
            assert report_status_updates == ["pending", "failed"]


@pytest.mark.asyncio
async def test_match_job_workflow_unit_success():
    wf = MatchJobWorkflow()

    executed_activities = []

    async def fake_execute_activity(act, input_data, **kwargs):
        name = getattr(act, "__name__", str(act))
        executed_activities.append((name, input_data))
        if name == "evaluate_match_activity":
            return {
                "evaluations": [{"category": "technical_skills", "score": 90, "reason": "strong"}],
                "mandatory_requirements": [],
                "prompt_version": "evaluate-match-v1",
                "model": "model-1",
                "provider": "omniroute",
            }
        elif name == "update_evaluation_status_activity":
            return {
                "status": input_data.get("status"),
                "overall_score": 90.0,
                "eligibility_status": "eligible",
                "recommendation_label": "strong_match",
            }
        elif name == "generate_report_activity":
            return {
                "job_title": "Go Engineer",
                "summary": "Great match",
                "strengths": ["Go"],
                "gaps": [],
                "experience_assessment": "Good fit",
                "cv_improvements": [],
                "learning_recommendations": [],
                "interview_preparation": [],
            }
        elif name == "update_report_status_activity":
            return {"report_status": input_data.get("report_status")}
        return {}

    with patch("workflows.match_job.workflow.execute_activity", side_effect=fake_execute_activity):
        res = await wf.run({
            "evaluation_id": "eval-unit-1",
            "slug": "test-job",
            "candidate_profile": {"skills": ["go"]},
            "job_profile": {"family": "technology"},
            "evaluation_config": {"family": "technology"},
        })

    assert res["status"] == "success"
    assert res["evaluation_id"] == "eval-unit-1"
    assert res["report_status"] == "generated"
    assert res["report"]["job_title"] == "Go Engineer"
    assert res["report_error"] is None

    activity_names = [a[0] for a in executed_activities]
    assert activity_names == [
        "update_evaluation_status_activity",
        "evaluate_match_activity",
        "update_evaluation_status_activity",
        "update_report_status_activity",
        "generate_report_activity",
        "update_report_status_activity",
    ]


@pytest.mark.asyncio
async def test_match_job_workflow_unit_report_failure_retains_success():
    wf = MatchJobWorkflow()

    executed_activities = []

    async def fake_execute_activity(act, input_data, **kwargs):
        name = getattr(act, "__name__", str(act))
        executed_activities.append((name, input_data))
        if name == "evaluate_match_activity":
            return {
                "evaluations": [{"category": "technical_skills", "score": 85, "reason": "good"}],
                "mandatory_requirements": [],
            }
        elif name == "update_evaluation_status_activity":
            return {
                "status": input_data.get("status"),
                "overall_score": 85.0,
                "eligibility_status": "eligible",
                "recommendation_label": "strong_match",
            }
        elif name == "generate_report_activity":
            raise RuntimeError("LLM report timed out")
        elif name == "update_report_status_activity":
            return {"report_status": input_data.get("report_status")}
        return {}

    with patch("workflows.match_job.workflow.execute_activity", side_effect=fake_execute_activity):
        res = await wf.run({
            "evaluation_id": "eval-unit-report-fail",
            "slug": "test-job",
            "candidate_profile": {"skills": ["go"]},
            "job_profile": {"family": "technology"},
            "evaluation_config": {"family": "technology"},
        })

    assert res["status"] == "success"
    assert res["evaluation_id"] == "eval-unit-report-fail"
    assert res["report_status"] == "failed"
    assert "timed out" in res["report_error"]

    # Verify update_report_status_activity was called with failed
    last_report_update = [a for a in executed_activities if a[0] == "update_report_status_activity"][-1]
    assert last_report_update[1]["report_status"] == "failed"
    assert "timed out" in last_report_update[1]["report_failure_message"]


