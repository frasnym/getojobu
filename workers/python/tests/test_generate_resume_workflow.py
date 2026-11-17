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
async def test_generate_resume_workflow_stores_detailed_failure_message() -> None:
    wf = GenerateResumeWorkflow()

    async def fake_execute_activity(act, input_data, **kwargs):  # type: ignore[no-untyped-def]
        if act.__name__ == "update_resume_status_activity":
            return {"resume_status": input_data["resume_status"]}
        raise ValueError("skills must contain between 1 and 12 items")

    with patch(
        "workflows.generate_resume.workflow.execute_activity",
        side_effect=fake_execute_activity,
    ):
        result: dict[str, Any] = await wf.run(
            {
                "evaluation_id": "eval-1",
                "job_id": "job-1",
                "user_id": "user-1",
            }
        )

    assert result["resume_status"] == "failed"
    assert result["resume_error"] == "skills must contain between 1 and 12 items"
