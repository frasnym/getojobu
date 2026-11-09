"""JobAnalyzeWorkflow — Temporal workflow definition.

Orchestrates the job-matching pipeline. Step 1B (Analyze Job Description)
is wired to a real Temporal activity; Steps 1A, 2, 3, 4 are stubbed for
the skeleton so the workflow skeleton compiles and runs.

Mirrors the pipeline in docs/workflows/getojobu-job-matching-implementation-reference.md:
  1A. Analyze CV (stub)
  1B. Analyze job description (activity: analyze_job_activity)
  2. Evaluate compatibility (stub)
  3. Calculate score and eligibility (stub)
  4. Generate report (stub)
"""

from __future__ import annotations

from datetime import timedelta
from typing import Any

from temporalio import workflow


with workflow.unsafe.imports_passed_through():
    from deliveries.consumer.activities.analyze_job import (
        analyze_job_activity,
    )


@workflow.defn(name="JobAnalyzeWorkflow")
class JobAnalyzeWorkflow:
    @workflow.run
    async def run(self, input: dict[str, Any]) -> dict[str, Any]:
        """Run the JobAnalyze workflow.

        Input:
            {
                "job_description": str,  # required for Step 1B
                "job_id": str | None,    # optional reference to shared jobs table
            }

        Returns a dict with step results and overall status.
        """
        job_description = input.get("job_description")
        job_id = input.get("job_id")

        results: dict[str, Any] = {}
        status = "partial"

        # Step 1B: Analyze job description (real activity).
        step_1b = await workflow.execute_activity(
            analyze_job_activity,
            {"job_description": job_description, "job_id": job_id},
            start_to_close_timeout=timedelta(minutes=10),
            schedule_to_close_timeout=timedelta(minutes=15),
        )
        results["step_1b_analyze_job"] = step_1b

        # Steps 1A, 2, 3, 4 are stubbed for the skeleton.
        results["step_1a_analyze_cv"] = {"status": "stubbed"}
        results["step_2_evaluate_match"] = {"status": "stubbed"}
        results["step_3_calculate_score"] = {"status": "stubbed"}
        results["step_4_generate_report"] = {"status": "stubbed"}

        status = "completed"
        return {
            "workflow": "JobAnalyzeWorkflow",
            "status": status,
            "job_id": job_id,
            "results": results,
        }
