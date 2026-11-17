"""JobAnalyzeWorkflow — Temporal workflow definition.

Orchestrates the job-matching pipeline. Step 1B (Analyze Job Description)
is wired to a real Temporal activity: analyze_job_activity.

Mirrors the pipeline in docs/workflows/getojobu-job-matching-implementation-reference.md.
"""

from __future__ import annotations

from datetime import timedelta
from typing import Any

from temporalio import workflow
from temporalio.common import RetryPolicy

with workflow.unsafe.imports_passed_through():
    from deliveries.consumer.activities.analyze_job import (
        analyze_job_activity,
    )

DEFAULT_RETRY_POLICY = RetryPolicy(
    maximum_attempts=3,
    initial_interval=timedelta(seconds=2),
    backoff_coefficient=2.0,
    maximum_interval=timedelta(seconds=30),
)


@workflow.defn(name="JobAnalyzeWorkflow")
class JobAnalyzeWorkflow:
    @workflow.run
    async def run(self, input: dict[str, Any]) -> dict[str, Any]:
        """Run the JobAnalyze workflow.

        Input:
            {
                "job_description": str,  # required for Step 1B
                "slug": str | None,      # optional slug of the job listing
            }

        Returns a dict with the analysis result and overall status.
        """
        job_description = input.get("job_description")
        slug = input.get("slug")

        results: dict[str, Any] = {}

        # Step 1B: Analyze job description (real activity).
        step_1b = await workflow.execute_activity(
            analyze_job_activity,
            {"job_description": job_description, "slug": slug},
            start_to_close_timeout=timedelta(minutes=10),
            schedule_to_close_timeout=timedelta(minutes=15),
            retry_policy=DEFAULT_RETRY_POLICY,
        )
        results["step_1b_analyze_job"] = step_1b

        return {
            "workflow": "JobAnalyzeWorkflow",
            "status": "completed",
            "slug": slug,
            "results": results,
        }
