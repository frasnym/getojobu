"""JobScrapeWorkflow — Temporal workflow definition for preference-driven job
discovery.

Orchestrates one scan of the configured job boards for a given set of keyword
× location searches:

    scrape_scan  →  enrich_jobs  →  ingest_jobs  →  queue_analysis

- scrape_scan:  one jobspy.scrape_jobs() call per board (sequentially per scan
  so one board's rate limits do not block another), errors are logged into
  attrs["errors"] rather than failing the scan.
- enrich_jobs:  normalize location, drop rows missing title/company/location,
  enforce the 24h recency filter by posted_date, attach audit attribution,
  sort remaining rows posted_date DESC.
- ingest_jobs:  call the Go API POST /v1/jobs/bulk (chunked, max 100), record
  created vs skipped (slug conflict, first-source-wins).
- queue_analysis:  start JobAnalyzeWorkflow for each newly created job with a
  stable id (job-analyze-{job_id}).

Stable workflow ids (job-scrape-scan-{scan} or job-scrape-{user_id}-{scan})
make daily reruns idempotent: Temporal re-runs the already-completed execution.
"""

from __future__ import annotations

from datetime import timedelta
from typing import Any, cast

from temporalio import workflow
from temporalio.common import RetryPolicy

with workflow.unsafe.imports_passed_through():
    from deliveries.consumer.activities.job_scrape import (
        enrich_jobs,
        ingest_jobs,
        parse_user_ids,
        queue_analysis,
        scrape_scan,
    )

NAMESPACE = "default"

DEFAULT_RETRY_POLICY = RetryPolicy(
    maximum_attempts=3,
    initial_interval=timedelta(seconds=2),
    backoff_coefficient=2.0,
    maximum_interval=timedelta(seconds=30),
)


@workflow.defn(name="JobScrapeWorkflow")
class JobScrapeWorkflow:
    @workflow.run
    async def run(self, input: dict[str, Any]) -> dict[str, Any]:
        """Run the job scrape scan.

        Input:
            {
                "board":        str,                 # board name (glassdoor, etc)
                "keyword":      str,                 # search term
                "location":     str,                 # search location
                "user_ids":     [str],               # matched internal user ids
                "search_id":    str,                 # deterministic scan id
                "searches":     [...],               # optional legacy searches
                "workflow_id":  str,                 # stable workflow id
            }
        """
        board_val = input.get("board")
        board: str = board_val.strip().lower() if isinstance(board_val, str) else ""
        if not board:
            raise ValueError("board is required and cannot be empty")
        keyword_val = input.get("keyword")
        keyword: str = keyword_val if isinstance(keyword_val, str) else ""
        location_val = input.get("location")
        location: str = location_val if isinstance(location_val, str) else ""

        user_ids = parse_user_ids(input.get("user_ids"))

        search_id = input.get("search_id")

        # 1. scrape_scan — issue board queries, accumulate raw results.
        scrape_scan_input: dict[str, Any] = {
            "board": board,
            "keyword": keyword,
            "location": location,
            "user_ids": user_ids,
            "search_id": search_id,
        }
        scan_results: dict[str, Any] = await workflow.execute_activity(
            scrape_scan,
            scrape_scan_input,
            start_to_close_timeout=timedelta(minutes=5),
            schedule_to_close_timeout=timedelta(minutes=10),
            heartbeat_timeout=timedelta(seconds=60),
            retry_policy=DEFAULT_RETRY_POLICY,
        )

        # 2. enrich_jobs — clean, normalize, filter by recency, attach audit.
        raw_jobs_val = scan_results.get("jobs")
        raw_jobs: list[Any] = (
            cast(list[Any], raw_jobs_val) if isinstance(raw_jobs_val, list) else []
        )
        errors_val = scan_results.get("errors")
        errors: list[Any] = (
            cast(list[Any], errors_val) if isinstance(errors_val, list) else []
        )

        enrich_jobs_input: dict[str, Any] = {
            "board": board,
            "keyword": keyword,
            "location": location,
            "user_ids": user_ids,
            "search_id": search_id,
            "raw_jobs": raw_jobs,
            "errors": errors,
        }
        enriched: dict[str, Any] = await workflow.execute_activity(
            enrich_jobs,
            enrich_jobs_input,
            start_to_close_timeout=timedelta(minutes=5),
            schedule_to_close_timeout=timedelta(minutes=10),
            heartbeat_timeout=timedelta(seconds=60),
            retry_policy=DEFAULT_RETRY_POLICY,
        )

        # 3. ingest_jobs — persist to Go API, split created vs skipped.
        enriched_jobs_val = enriched.get("jobs")
        enriched_jobs: list[Any] = (
            cast(list[Any], enriched_jobs_val)
            if isinstance(enriched_jobs_val, list)
            else []
        )

        ingest_jobs_input: dict[str, Any] = {
            "user_ids": user_ids,
            "search_id": search_id,
            "jobs": enriched_jobs,
        }
        ingested: dict[str, Any] = await workflow.execute_activity(
            ingest_jobs,
            ingest_jobs_input,
            start_to_close_timeout=timedelta(minutes=5),
            schedule_to_close_timeout=timedelta(minutes=10),
            heartbeat_timeout=timedelta(seconds=60),
            retry_policy=DEFAULT_RETRY_POLICY,
        )

        # 4. queue_analysis — start analysis for newly created jobs.
        created_slugs_val = ingested.get("created_slugs")
        created_slugs: list[Any] = (
            cast(list[Any], created_slugs_val)
            if isinstance(created_slugs_val, list)
            else []
        )

        queue_analysis_input: dict[str, Any] = {
            "user_ids": user_ids,
            "search_id": search_id,
            "created_job_slugs": created_slugs,
        }
        analysis: dict[str, Any] = await workflow.execute_activity(
            queue_analysis,
            queue_analysis_input,
            start_to_close_timeout=timedelta(minutes=5),
            schedule_to_close_timeout=timedelta(minutes=10),
            heartbeat_timeout=timedelta(seconds=60),
            retry_policy=DEFAULT_RETRY_POLICY,
        )

        boards_val = scan_results.get("boards")
        boards_list: list[Any] = (
            cast(list[Any], boards_val) if isinstance(boards_val, list) else []
        )
        found_val = enriched.get("found")
        filtered_val = enriched.get("filtered_out_recency")
        inserted_val = ingested.get("inserted")
        skipped_val = ingested.get("skipped")
        started_val = analysis.get("started")
        failed_val = analysis.get("failed")
        failed_list: list[Any] = (
            cast(list[Any], failed_val) if isinstance(failed_val, list) else []
        )

        return {
            "workflow": "JobScrapeWorkflow",
            "board": board,
            "keyword": keyword,
            "location": location,
            "user_ids": user_ids,
            "search_id": search_id,
            "workflow_id": input.get("workflow_id"),
            "scrape_scan": scan_results,
            "enrich_jobs": enriched,
            "ingest_jobs": ingested,
            "queue_analysis": analysis,
            "stats": {
                "boards_run": len(boards_list),
                "jobs_found": found_val if isinstance(found_val, int) else 0,
                "filtered_out_recency": (
                    filtered_val if isinstance(filtered_val, int) else 0
                ),
                "inserted": inserted_val if isinstance(inserted_val, int) else 0,
                "skipped": skipped_val if isinstance(skipped_val, int) else 0,
                "analysis_started": started_val if isinstance(started_val, int) else 0,
                "analysis_failed": len(failed_list),
            },
        }
