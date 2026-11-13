"""MatchJobWorkflow — Temporal workflow definition for Step 2 Compatibility Evaluation.

Flow:
1. Update evaluation status to 'pending' via update_evaluation_status_activity.
2. Run evaluate_match_activity to execute the LLM compatibility evaluation.
3. If successful: update evaluation status to 'success' with the evaluation data.
4. If failed: update evaluation status to 'failed' with the failure message.
"""

from __future__ import annotations

from datetime import timedelta
from typing import Any, cast

from temporalio import workflow
from temporalio.common import RetryPolicy

with workflow.unsafe.imports_passed_through():
    from deliveries.consumer.activities.evaluate_match import (
        evaluate_match_activity,
        update_evaluation_status_activity,
    )

DEFAULT_RETRY_POLICY = RetryPolicy(
    maximum_attempts=3,
    initial_interval=timedelta(seconds=2),
    backoff_coefficient=2.0,
    maximum_interval=timedelta(seconds=30),
)


@workflow.defn(name="MatchJobWorkflow")
class MatchJobWorkflow:
    @workflow.run
    async def run(self, input: dict[str, Any]) -> dict[str, Any]:
        """Run the MatchJobWorkflow.

        Input:
            {
                "evaluation_id": str,
                "slug": str,
                "candidate_profile": dict,
                "job_profile": dict,
                "evaluation_config": dict,
            }
        """
        raw_eval_id = input.get("evaluation_id")
        if not raw_eval_id or not isinstance(raw_eval_id, str):
            raise ValueError("evaluation_id is required")
        evaluation_id: str = raw_eval_id

        raw_candidate = input.get("candidate_profile")
        candidate_profile: dict[str, Any] = (
            cast(dict[str, Any], raw_candidate)
            if isinstance(raw_candidate, dict)
            else {}
        )

        raw_job = input.get("job_profile")
        job_profile: dict[str, Any] = (
            cast(dict[str, Any], raw_job)
            if isinstance(raw_job, dict)
            else {}
        )

        raw_config = input.get("evaluation_config")
        evaluation_config: dict[str, Any] = (
            cast(dict[str, Any], raw_config)
            if isinstance(raw_config, dict)
            else {}
        )

        provider_val = input.get("provider")
        provider: str = str(provider_val) if provider_val is not None else "omniroute"

        model_val = input.get("model")
        model: str | None = str(model_val) if model_val is not None else None

        # Step 1: Mark evaluation pending
        pending_input: dict[str, Any] = {
            "evaluation_id": evaluation_id,
            "status": "pending",
        }
        await workflow.execute_activity(
            update_evaluation_status_activity,
            pending_input,
            start_to_close_timeout=timedelta(seconds=30),
            retry_policy=DEFAULT_RETRY_POLICY,
        )

        # Step 2: Evaluate match using LLM, and Step 3: Mark evaluation success
        try:
            eval_input: dict[str, Any] = {
                "candidate_profile": candidate_profile,
                "job_profile": job_profile,
                "evaluation_config": evaluation_config,
            }
            eval_output: dict[str, Any] = await workflow.execute_activity(
                evaluate_match_activity,
                eval_input,
                start_to_close_timeout=timedelta(minutes=5),
                retry_policy=DEFAULT_RETRY_POLICY,
            )

            # Step 3: Mark evaluation success with result
            success_input: dict[str, Any] = {
                "evaluation_id": evaluation_id,
                "status": "success",
                "evaluation_data": eval_output,
                "provider": provider,
                "model": model,
            }
            await workflow.execute_activity(
                update_evaluation_status_activity,
                success_input,
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=DEFAULT_RETRY_POLICY,
            )
        except Exception as exc:
            # Mark evaluation failed on error
            failed_input: dict[str, Any] = {
                "evaluation_id": evaluation_id,
                "status": "failed",
                "failure_message": str(exc),
                "provider": provider,
                "model": model,
            }
            await workflow.execute_activity(
                update_evaluation_status_activity,
                failed_input,
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=DEFAULT_RETRY_POLICY,
            )
            raise

        return {
            "workflow": "MatchJobWorkflow",
            "evaluation_id": evaluation_id,
            "status": "success",
            "evaluations": eval_output,
        }
