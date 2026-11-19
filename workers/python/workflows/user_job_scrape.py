"""UserJobScrapeWorkflow — LLM extraction for a user-supplied job URL.

    fetch_and_extract_job → update_scraped_job → queue_scraped_job_analysis

The skeleton job starts at scrape_pending. A successful update transitions it
to new and queues analysis; a failure records scrape_failed so the user can
retry or delete the job.
"""

from __future__ import annotations

from datetime import timedelta
from typing import Any

from temporalio import workflow
from temporalio.common import RetryPolicy

with workflow.unsafe.imports_passed_through():
    from deliveries.consumer.activities.user_job_scrape import (
        fetch_and_extract_job,
        mark_scrape_failed,
        queue_scraped_job_analysis,
        update_scraped_job,
    )

FETCH_RETRY = RetryPolicy(maximum_attempts=2)
API_RETRY = RetryPolicy(maximum_attempts=3)


@workflow.defn(name="UserJobScrapeWorkflow")
class UserJobScrapeWorkflow:
    @workflow.run
    async def run(self, input: dict[str, Any]) -> dict[str, Any]:
        slug = str(input.get("slug") or "")
        current_slug = slug
        owner_id = str(input.get("owner_id") or "")
        source_url = str(input.get("source_url") or "")

        try:
            extracted: dict[str, Any] = await workflow.execute_activity(
                fetch_and_extract_job,
                {"source_url": source_url},
                start_to_close_timeout=timedelta(seconds=60),
                retry_policy=FETCH_RETRY,
            )

            updated: dict[str, Any] = await workflow.execute_activity(
                update_scraped_job,
                {"old_slug": slug, "owner_id": owner_id, "extracted": extracted},
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=API_RETRY,
            )
            new_slug = str(updated.get("slug") or slug)
            current_slug = new_slug

            analysis: dict[str, Any] = await workflow.execute_activity(
                queue_scraped_job_analysis,
                {"slug": new_slug},
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=API_RETRY,
            )
            return {
                "workflow": "UserJobScrapeWorkflow",
                "status": "new",
                "slug": new_slug,
                "extracted": extracted,
                "analysis": analysis,
            }
        except Exception as exc:  # noqa: BLE001
            failure_message = f"{type(exc).__name__}: {exc}"
            await workflow.execute_activity(
                mark_scrape_failed,
                {"slug": current_slug, "failure_message": failure_message},
                start_to_close_timeout=timedelta(seconds=30),
                retry_policy=API_RETRY,
            )
            return {
                "workflow": "UserJobScrapeWorkflow",
                "status": "scrape_failed",
                "slug": slug,
                "failure_message": failure_message,
            }
