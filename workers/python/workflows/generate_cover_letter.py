"""Standalone GenerateCoverLetterWorkflow, isolated from report/resume generation."""

from __future__ import annotations

from datetime import timedelta
from typing import Any

from temporalio import workflow
from temporalio.common import RetryPolicy

with workflow.unsafe.imports_passed_through():
    from deliveries.consumer.activities.cover_letter_generation import (
        fetch_cover_letter_input_activity,
        generate_cover_letter_activity,
        update_cover_letter_status_activity,
    )

RETRY_POLICY = RetryPolicy(maximum_attempts=3, initial_interval=timedelta(seconds=2))

# User-visible failure message is fixed: never surface raw exceptions, prompt
# text, or CV contents to the API (and therefore to the user).
COVER_LETTER_FAILURE_MESSAGE = "Cover letter generation failed"

STATUS_PENDING = "pending"
STATUS_SUCCESS = "success"
STATUS_FAILED = "failed"


@workflow.defn(name="GenerateCoverLetterWorkflow")
class GenerateCoverLetterWorkflow:
    @workflow.run
    async def run(self, input: dict[str, Any]) -> dict[str, Any]:
        evaluation_id = input.get("evaluation_id")
        if not isinstance(evaluation_id, str) or not evaluation_id:
            raise ValueError("evaluation_id is required")

        fetched: dict[str, Any] = await workflow.execute_activity(
            fetch_cover_letter_input_activity,
            {"evaluation_id": evaluation_id},
            start_to_close_timeout=timedelta(seconds=30),
            retry_policy=RETRY_POLICY,
        )
        if fetched.get("status") == "skipped":
            return {
                "workflow": "GenerateCoverLetterWorkflow",
                "evaluation_id": evaluation_id,
                "cover_letter_status": "skipped",
            }

        letter_input: dict[str, Any] = {
            key: value
            for key, value in fetched.items()
            if key not in ("status", "cover_letter_status")
        }
        letter_input["variant"] = input.get("variant")
        pending_payload: dict[str, Any] = {
            "evaluation_id": evaluation_id,
            "cover_letter_status": STATUS_PENDING,
        }
        await workflow.execute_activity(
            update_cover_letter_status_activity,
            pending_payload,
            start_to_close_timeout=timedelta(seconds=30),
            retry_policy=RETRY_POLICY,
        )
        try:
            content: dict[str, Any] = await workflow.execute_activity(
                generate_cover_letter_activity,
                letter_input,
                start_to_close_timeout=timedelta(minutes=5),
                retry_policy=RETRY_POLICY,
            )
            success_payload: dict[str, Any] = {
                "evaluation_id": evaluation_id,
                "cover_letter_status": STATUS_SUCCESS,
                "cover_letter_content": content,
            }
            await workflow.execute_activity(
                update_cover_letter_status_activity,
                success_payload,
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=RETRY_POLICY,
            )
            return {
                "workflow": "GenerateCoverLetterWorkflow",
                "evaluation_id": evaluation_id,
                "cover_letter_status": STATUS_SUCCESS,
                "cover_letter": content,
            }
        except Exception:  # noqa: BLE001
            # Deliberately omit cover_letter_content: the Go usecase preserves
            # the previous letter text, and an empty content would clear it.
            fail_payload: dict[str, Any] = {
                "evaluation_id": evaluation_id,
                "cover_letter_status": STATUS_FAILED,
                "cover_letter_failure_message": COVER_LETTER_FAILURE_MESSAGE,
            }
            await workflow.execute_activity(
                update_cover_letter_status_activity,
                fail_payload,
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=RETRY_POLICY,
            )
            return {
                "workflow": "GenerateCoverLetterWorkflow",
                "evaluation_id": evaluation_id,
                "cover_letter_status": STATUS_FAILED,
                "cover_letter_error": COVER_LETTER_FAILURE_MESSAGE,
            }
