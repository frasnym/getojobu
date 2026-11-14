"""MatchJobWorkflow — Temporal workflow definition for Step 2 Compatibility Evaluation and Step 4 Report Generation.

Flow:
1. Update evaluation status to 'pending' via update_evaluation_status_activity.
2. Run evaluate_match_activity to execute the LLM compatibility evaluation.
3. If successful: update evaluation status to 'success' with the evaluation data.
4. Auto-generate candidate match report (Step 4) via generate_report_activity and update_report_status_activity.
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
    from deliveries.consumer.activities.report_generation import (
        generate_report_activity,
        update_report_status_activity,
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
            cast(dict[str, Any], raw_job) if isinstance(raw_job, dict) else {}
        )

        raw_config = input.get("evaluation_config")
        evaluation_config: dict[str, Any] = (
            cast(dict[str, Any], raw_config) if isinstance(raw_config, dict) else {}
        )

        provider_val = input.get("provider")
        provider: str | None = str(provider_val) if provider_val is not None else None

        model_val = input.get("model")
        model: str | None = str(model_val) if model_val is not None else None

        # Step 1: Mark evaluation pending
        pending_input: dict[str, Any] = {
            "evaluation_id": evaluation_id,
            "status": "pending",
            "evaluation_status": "pending",
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
                "evaluation_status": "success",
                "evaluation_data": eval_output,
                "provider": provider,
                "model": model,
            }
            success_res = await workflow.execute_activity(
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
                "evaluation_status": "failed",
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

        # Step 4: Auto-generate match report
        is_retry = bool(
            input.get("is_retry") or input.get("evaluation_status") == "new-retry"
        )
        initial_report_status = "pending-retry" if is_retry else "pending"

        report_output: dict[str, Any] | None = None
        report_error: str | None = None
        try:
            report_pending_input: dict[str, Any] = {
                "evaluation_id": evaluation_id,
                "report_status": initial_report_status,
            }
            await workflow.execute_activity(
                update_report_status_activity,
                report_pending_input,
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=DEFAULT_RETRY_POLICY,
            )

            overall_score: float = (
                float(success_res.get("overall_score") or 0.0) if success_res else 0.0
            )
            eligibility_status: str = (
                str(success_res.get("eligibility_status") or "eligible")
                if success_res
                else "eligible"
            )
            recommendation_label: str = (
                str(success_res.get("recommendation_label") or "worth_applying")
                if success_res
                else "worth_applying"
            )

            report_input: dict[str, Any] = {
                "candidate_profile": candidate_profile,
                "job_profile": job_profile,
                "evaluation_results": eval_output,
                "overall_score": overall_score,
                "eligibility_status": eligibility_status,
                "recommendation_label": recommendation_label,
            }

            report_output = await workflow.execute_activity(
                generate_report_activity,
                report_input,
                start_to_close_timeout=timedelta(minutes=5),
                retry_policy=DEFAULT_RETRY_POLICY,
            )

            report_success_input: dict[str, Any] = {
                "evaluation_id": evaluation_id,
                "report_status": "success",
                "report_data": report_output,
            }
            await workflow.execute_activity(
                update_report_status_activity,
                report_success_input,
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=DEFAULT_RETRY_POLICY,
            )
        except Exception as exc:  # noqa: BLE001
            cause = getattr(exc, "cause", None)
            if cause is not None:
                report_error = getattr(cause, "message", None) or str(cause)
            else:
                report_error = str(exc)
            report_failed_input: dict[str, Any] = {
                "evaluation_id": evaluation_id,
                "report_status": "failed",
                "report_failure_message": report_error,
            }
            await workflow.execute_activity(
                update_report_status_activity,
                report_failed_input,
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=DEFAULT_RETRY_POLICY,
            )

        return {
            "workflow": "MatchJobWorkflow",
            "evaluation_id": evaluation_id,
            "status": "success",
            "evaluations": eval_output,
            "report_status": "success" if report_output is not None else "failed",
            "report": report_output,
            "report_error": report_error,
        }
