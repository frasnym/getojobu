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
        fetch_report_input_activity,
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
            }
        """
        raw_eval_id = input.get("evaluation_id")
        if not raw_eval_id or not isinstance(raw_eval_id, str):
            raise ValueError("evaluation_id is required")
        evaluation_id: str = raw_eval_id

        fetched: dict[str, Any] = await workflow.execute_activity(
            fetch_report_input_activity,
            {"evaluation_id": evaluation_id},
            start_to_close_timeout=timedelta(seconds=30),
            retry_policy=DEFAULT_RETRY_POLICY,
        )
        if fetched.get("status") == "skipped":
            return {
                "workflow": "GenerateReportWorkflow",
                "evaluation_id": evaluation_id,
                "report_status": "skipped",
                "report_error": "evaluation report is not queued",
            }

        candidate_profile = cast(dict[str, Any], fetched["candidate_profile"])
        job_profile = cast(dict[str, Any], fetched["job_profile"])
        evaluation_results = cast(dict[str, Any], fetched["evaluation_results"])
        overall_score: float = float(fetched["overall_score"])
        eligibility_status: str = str(fetched["eligibility_status"])
        recommendation_label: str = str(fetched["recommendation_label"])

        # Step 1: Mark report pending
        pending_input: dict[str, Any] = {
            "evaluation_id": evaluation_id,
            "report_status": "pending",
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
