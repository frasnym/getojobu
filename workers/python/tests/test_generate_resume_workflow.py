"""Tests for GenerateResumeWorkflow failure messaging."""

from typing import Any
from unittest.mock import patch

import pytest
from temporalio.exceptions import ActivityError

from workflows.generate_resume import GenerateResumeWorkflow, _activity_error_message


def test_activity_error_message_keeps_detailed_message() -> None:
    err = ActivityError(
        "skills must contain between 1 and 12 items",
        scheduled_event_id=1,
        started_event_id=2,
        identity="worker-1",
        activity_type="generate_resume_activity",
        activity_id="act-1",
        retry_state=None,
    )
    assert _activity_error_message(err) == "skills must contain between 1 and 12 items"


def test_activity_error_message_falls_back_to_str_on_generic_wrapper() -> None:
    # Temporal can surface the generic wrapper; the helper should not treat it
    # as a useful message even when no failure detail is available.
    err = ActivityError(
        "Activity task failed",
        scheduled_event_id=1,
        started_event_id=2,
        identity="worker-1",
        activity_type="generate_resume_activity",
        activity_id="act-1",
        retry_state=None,
    )
    assert _activity_error_message(err) == "Activity task failed"


def test_activity_error_message_recovers_detail_from_wrapped_failure() -> None:
    err = ActivityError(
        "Activity task failed",
        scheduled_event_id=1,
        started_event_id=2,
        identity="worker-1",
        activity_type="generate_resume_activity",
        activity_id="act-1",
        retry_state=None,
    )
    object.__setattr__(
        err,
        "_failure",
        type(
            "Failure", (), {"message": "skills must contain between 1 and 12 items"}
        )(),
    )
    assert _activity_error_message(err) == "skills must contain between 1 and 12 items"


@pytest.mark.asyncio
async def test_generate_resume_workflow_failure_does_not_complete() -> None:
    wf = GenerateResumeWorkflow()
    written_statuses: list[str] = []
    fail_payloads: list[dict[str, Any]] = []

    async def fake_execute_activity(act, input_data, **kwargs):  # type: ignore[no-untyped-def]
        if act.__name__ == "fetch_resume_input_activity":
            return {
                "status": "ready",
                "resume_status": "queued",
                "job_id": "job-1",
                "user_id": "user-1",
                "candidate_profile": {"skills": ["go"]},
                "job_profile": {"title": "Go Engineer"},
                "evaluation_results": {"evaluations": []},
                "facts": {"name": "A"},
                "overall_score": 90.0,
                "eligibility_status": "eligible",
                "recommendation_label": "strong_match",
            }
        if act.__name__ == "update_resume_status_activity":
            written_statuses.append(input_data["resume_status"])
            if input_data["resume_status"] == "failed":
                fail_payloads.append(input_data)
            return {"resume_status": input_data["resume_status"]}
        raise ValueError("skills must contain between 1 and 12 items")

    with patch(
        "workflows.generate_resume.workflow.execute_activity",
        side_effect=fake_execute_activity,
    ):
        with pytest.raises(
            ValueError, match="skills must contain between 1 and 12 items"
        ):
            await wf.run(
                {
                    "evaluation_id": "eval-1",
                    "job_id": "job-1",
                    "user_id": "user-1",
                }
            )

    assert written_statuses == ["pending", "failed"]
    assert (
        fail_payloads[0]["resume_failure_message"]
        == "skills must contain between 1 and 12 items"
    )


@pytest.mark.asyncio
async def test_generate_resume_workflow_success_completes() -> None:
    wf = GenerateResumeWorkflow()

    async def fake_execute_activity(act, input_data, **kwargs):  # type: ignore[no-untyped-def]
        if act.__name__ == "fetch_resume_input_activity":
            return {
                "status": "ready",
                "resume_status": "queued",
                "job_id": "job-1",
                "user_id": "user-1",
                "candidate_profile": {"skills": ["go"]},
                "job_profile": {"title": "Go Engineer"},
                "evaluation_results": {"evaluations": []},
                "facts": {"name": "A"},
                "overall_score": 90.0,
                "eligibility_status": "eligible",
                "recommendation_label": "strong_match",
            }
        if act.__name__ == "generate_resume_activity":
            return {"pdf_base64": "aGVsbG8=", "resume": {"name": "A"}}
        if act.__name__ == "upload_resume_activity":
            return {"pdf_key": "k", "content_key": "c"}
        return {"resume_status": input_data["resume_status"]}

    with patch(
        "workflows.generate_resume.workflow.execute_activity",
        side_effect=fake_execute_activity,
    ):
        result: dict[str, Any] = await wf.run({"evaluation_id": "eval-1"})

    assert result["resume_status"] == "success"
