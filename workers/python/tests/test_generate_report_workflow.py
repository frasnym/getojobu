"""Tests for GenerateReportWorkflow."""

from typing import Any
from unittest.mock import patch

import pytest
from temporalio import activity
from temporalio.testing import WorkflowEnvironment
from temporalio.worker import Worker

from workflows.generate_report import GenerateReportWorkflow


@pytest.mark.asyncio
async def test_generate_report_workflow_unit_success():
    wf = GenerateReportWorkflow()

    executed_activities = []

    async def fake_execute_activity(act, input_data, **kwargs):
        executed_activities.append((act.__name__, input_data))
        if act.__name__ == "generate_report_activity":
            return {
                "job_title": "Senior Go Engineer",
                "executive_summary": "Strong fit for the role.",
                "strengths": ["Deep Go expertise"],
                "gaps": [],
                "application_readiness": "ready",
            }
        return {"report_status": input_data.get("report_status")}

    with patch("workflows.generate_report.workflow.execute_activity", side_effect=fake_execute_activity):
        res = await wf.run({
            "evaluation_id": "eval-unit-123",
            "candidate_profile": {"skills": ["go"]},
            "job_profile": {"title": "Senior Go Engineer"},
            "evaluation_results": {"evaluations": []},
            "overall_score": 92.0,
            "eligibility_status": "eligible",
            "recommendation_label": "strong_match",
        })

    assert res["workflow"] == "GenerateReportWorkflow"
    assert res["evaluation_id"] == "eval-unit-123"
    assert res["report_status"] == "generated"
    assert res["report"]["job_title"] == "Senior Go Engineer"
    assert res["report_error"] is None

    assert len(executed_activities) == 3
    assert executed_activities[0] == ("update_report_status_activity", {"evaluation_id": "eval-unit-123", "report_status": "pending"})
    assert executed_activities[1][0] == "generate_report_activity"
    assert executed_activities[2][0] == "update_report_status_activity"
    assert executed_activities[2][1]["report_status"] == "generated"


@pytest.mark.asyncio
async def test_generate_report_workflow_unit_failure():
    wf = GenerateReportWorkflow()

    executed_activities = []

    async def fake_execute_activity(act, input_data, **kwargs):
        executed_activities.append((act.__name__, input_data))
        if act.__name__ == "generate_report_activity":
            raise RuntimeError("LLM rate limit reached")
        return {"report_status": input_data.get("report_status")}

    with patch("workflows.generate_report.workflow.execute_activity", side_effect=fake_execute_activity):
        res = await wf.run({
            "evaluation_id": "eval-unit-fail",
            "candidate_profile": {"skills": ["go"]},
            "job_profile": {"title": "Senior Go Engineer"},
            "evaluation_results": {},
            "overall_score": 75.0,
        })

    assert res["workflow"] == "GenerateReportWorkflow"
    assert res["evaluation_id"] == "eval-unit-fail"
    assert res["report_status"] == "failed"
    assert res["report"] is None
    assert "rate limit" in res["report_error"]

    assert len(executed_activities) == 3
    assert executed_activities[0] == ("update_report_status_activity", {"evaluation_id": "eval-unit-fail", "report_status": "pending"})
    assert executed_activities[1][0] == "generate_report_activity"
    assert executed_activities[2][0] == "update_report_status_activity"
    assert executed_activities[2][1]["report_status"] == "failed"
    assert "rate limit" in executed_activities[2][1]["report_failure_message"]


@pytest.mark.asyncio
async def test_generate_report_workflow_missing_eval_id():
    workflow = GenerateReportWorkflow()
    with pytest.raises(ValueError, match="evaluation_id is required"):
        await workflow.run({"evaluation_id": ""})


@pytest.mark.asyncio
async def test_generate_report_workflow_temporal_env_success():
    try:
        env = await WorkflowEnvironment.start_time_skipping()
    except Exception as exc:  # noqa: BLE001
        pytest.skip(f"Ephemeral temporal server unavailable: {exc}")
        return

    async with env:
        task_queue = "test-generate-report-queue"

        report_status_updates = []

        @activity.defn(name="generate_report_activity")
        async def fake_generate_report(input: dict[str, Any]) -> dict[str, Any]:
            return {
                "job_title": "Senior Go Engineer",
                "executive_summary": "Strong fit for the role.",
                "strengths": ["Deep Go expertise"],
                "gaps": [],
                "application_readiness": "ready",
            }

        @activity.defn(name="update_report_status_activity")
        async def fake_update_report_status(input: dict[str, Any]) -> dict[str, Any]:
            report_status_updates.append(input.get("report_status"))
            return {"report_status": input.get("report_status")}

        async with Worker(
            env.client,
            task_queue=task_queue,
            workflows=[GenerateReportWorkflow],
            activities=[fake_generate_report, fake_update_report_status],
        ):
            handle = await env.client.start_workflow(
                GenerateReportWorkflow.run,
                {
                    "evaluation_id": "eval-gen-123",
                    "candidate_profile": {"skills": ["go"]},
                    "job_profile": {"title": "Senior Go Engineer"},
                    "evaluation_results": {"evaluations": []},
                    "overall_score": 92.0,
                    "eligibility_status": "eligible",
                    "recommendation_label": "strong_match",
                },
                id="test-gen-report-wf-1",
                task_queue=task_queue,
            )
            result = await handle.result()

            assert result["workflow"] == "GenerateReportWorkflow"
            assert result["evaluation_id"] == "eval-gen-123"
            assert result["report_status"] == "generated"
            assert result["report"]["job_title"] == "Senior Go Engineer"
            assert result["report_error"] is None
            assert report_status_updates == ["pending", "generated"]
