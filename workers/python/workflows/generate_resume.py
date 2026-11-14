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


@workflow.defn(name="GenerateResumeWorkflow")
class GenerateResumeWorkflow:
    @workflow.run
    async def run(self, input: dict[str, Any]) -> dict[str, Any]:
        evaluation_id = input.get("evaluation_id")
        if not isinstance(evaluation_id, str) or not evaluation_id:
            raise ValueError("evaluation_id is required")
        if input.get("recommendation_label") not in {"strong_match", "worth_applying"}:
            return {
                "workflow": "GenerateResumeWorkflow",
                "evaluation_id": evaluation_id,
                "resume_status": "skipped",
            }
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
            message = str(exc)
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
            return {
                "workflow": "GenerateResumeWorkflow",
                "evaluation_id": evaluation_id,
                "resume_status": "failed",
                "resume_error": message,
            }
