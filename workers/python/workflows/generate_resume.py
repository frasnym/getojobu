"""Standalone GenerateResumeWorkflow, isolated from report generation."""

from __future__ import annotations

from datetime import timedelta
from typing import Any

from temporalio import workflow
from temporalio.common import RetryPolicy

with workflow.unsafe.imports_passed_through():
    from deliveries.consumer.activities.resume_generation import (
        generate_resume_activity,
        update_resume_status_activity,
        upload_resume_activity,
    )

RETRY_POLICY = RetryPolicy(maximum_attempts=3, initial_interval=timedelta(seconds=2))

# Generic strings Temporal's ActivityError/FailureError wrapper can fall back to
# instead of the real underlying cause. Never store or return these as-is.
_GENERIC_ACTIVITY_MESSAGES = {
    "Activity task failed",
    "Activity error",
    "Encoded failure",
    "",
}


def _activity_error_message(exc: BaseException) -> str:
    """Return the underlying reason for an activity failure.

    Temporal wraps activity failures in ActivityError/FailureError whose
    ``str()`` can be the generic wrapper (e.g. "Activity task failed") rather
    than the real cause. The real reason is carried on the wrapped failure
    proto; fall back to ``str(exc)`` when it isn't available.
    """
    message = str(exc).strip()
    if message in _GENERIC_ACTIVITY_MESSAGES:
        failure = getattr(exc, "failure", None)
        if failure is not None:
            proto_message = getattr(failure, "message", None)
            if proto_message and proto_message not in _GENERIC_ACTIVITY_MESSAGES:
                message = proto_message
    return message


@workflow.defn(name="GenerateResumeWorkflow")
class GenerateResumeWorkflow:
    @workflow.run
    async def run(self, input: dict[str, Any]) -> dict[str, Any]:
        evaluation_id = input.get("evaluation_id")
        if not isinstance(evaluation_id, str) or not evaluation_id:
            raise ValueError("evaluation_id is required")
        pending_payload: dict[str, Any] = {
            "evaluation_id": evaluation_id,
            "resume_status": "pending",
        }
        await workflow.execute_activity(
            update_resume_status_activity,
            pending_payload,
            start_to_close_timeout=timedelta(seconds=30),
            retry_policy=RETRY_POLICY,
        )
        try:
            generated: dict[str, Any] = await workflow.execute_activity(
                generate_resume_activity,
                input,
                start_to_close_timeout=timedelta(minutes=5),
                retry_policy=RETRY_POLICY,
            )
            upload_payload: dict[str, Any] = {
                "evaluation_id": evaluation_id,
                "job_id": input.get("job_id"),
                "user_id": input.get("user_id"),
                **generated,
            }
            artifacts: dict[str, Any] = await workflow.execute_activity(
                upload_resume_activity,
                upload_payload,
                start_to_close_timeout=timedelta(minutes=2),
                retry_policy=RETRY_POLICY,
            )
            success_payload: dict[str, Any] = {
                "evaluation_id": evaluation_id,
                "resume_status": "success",
                "resume_artifacts": artifacts,
            }
            await workflow.execute_activity(
                update_resume_status_activity,
                success_payload,
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=RETRY_POLICY,
            )
            return {
                "workflow": "GenerateResumeWorkflow",
                "evaluation_id": evaluation_id,
                "resume_status": "success",
                "artifacts": artifacts,
            }
        except Exception as exc:  # noqa: BLE001
            message = _activity_error_message(exc)
            fail_payload: dict[str, Any] = {
                "evaluation_id": evaluation_id,
                "resume_status": "failed",
                "resume_failure_message": message,
            }
            await workflow.execute_activity(
                update_resume_status_activity,
                fail_payload,
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=RETRY_POLICY,
            )
            # A failed generation must end the workflow as failed, not completed.
            raise
