"""GenerateReportWorkflow — Temporal workflow definition for Step 4 Match Report Generation.

Standalone workflow used for bulk regeneration of match reports without re-running Step 2 matching.
"""

from __future__ import annotations

from datetime import timedelta
from typing import Any, cast

from temporalio import workflow
from temporalio.common import RetryPolicy

with workflow.unsafe.imports_passed_through():
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


@workflow.defn(name="GenerateReportWorkflow")
class GenerateReportWorkflow:
    @workflow.run
    async def run(self, input: dict[str, Any]) -> dict[str, Any]:
        """Run the GenerateReportWorkflow.

        Input:
            {
                "evaluation_id": str,
                "candidate_profile": dict,
                "job_profile": dict,
                "evaluation_results": dict,
                "overall_score": float,
                "eligibility_status": str,
                "recommendation_label": str,
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

        raw_eval_results = input.get("evaluation_results")
        evaluation_results: dict[str, Any] = (
            cast(dict[str, Any], raw_eval_results)
            if isinstance(raw_eval_results, dict)
            else {}
        )

        raw_score = input.get("overall_score")
        overall_score: float = float(raw_score) if raw_score is not None else 0.0

        raw_eligibility = input.get("eligibility_status")
        eligibility_status: str = str(raw_eligibility) if raw_eligibility is not None else "eligible"

        raw_rec = input.get("recommendation_label")
        recommendation_label: str = str(raw_rec) if raw_rec is not None else "worth_applying"

        raw_report_status = input.get("report_status")
        initial_report_status: str = (
            str(raw_report_status)
            if raw_report_status in ("pending", "pending-retry")
            else "pending"
        )

        # Step 1: Mark report pending
        pending_input: dict[str, Any] = {
            "evaluation_id": evaluation_id,
            "report_status": initial_report_status,
        }
        await workflow.execute_activity(
            update_report_status_activity,
            pending_input,
            start_to_close_timeout=timedelta(seconds=30),
            retry_policy=DEFAULT_RETRY_POLICY,
        )

        # Step 2: Generate report and save
        report_output: dict[str, Any] | None = None
        report_error: str | None = None
        try:
            report_input: dict[str, Any] = {
                "candidate_profile": candidate_profile,
                "job_profile": job_profile,
                "evaluation_results": evaluation_results,
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
            "workflow": "GenerateReportWorkflow",
            "evaluation_id": evaluation_id,
            "report_status": "success" if report_output is not None else "failed",
            "report": report_output,
            "report_error": report_error,
        }
