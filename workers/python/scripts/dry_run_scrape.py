"""One-off dry-run script for the JobScrapeWorkflow.

Runs scrape_scan -> enrich_jobs only. Skips ingest_jobs and queue_analysis,
so nothing is written to the Go API / database and no analysis workflows are
queued. Use this to inspect real scrape + enrichment results before running
the full workflow via Temporal.

Usage:
    make dry-run

Env overrides:
    SCRAPE_BOARD        board name (default: glassdoor)
    SCRAPE_KEYWORD      search term (default: golang)
    SCRAPE_LOCATION     search location (default: indonesia)
    SCRAPE_USER_IDS     comma-separated user ids (default: dry-run-user)
"""

from __future__ import annotations

import asyncio
import json
import os
from typing import Any
from unittest.mock import MagicMock

from deliveries.consumer.activities.job_scrape import (
    enrich_jobs,
    init_job_scrape_container,
    scrape_scan,
)
from repository.scraper.glassdoor_jobspy import GlassdoorJobspyScraper
from usecase.scraper.service import ScraperService


def _env_list(name: str, default: list[str]) -> list[str]:
    raw = os.environ.get(name, "")
    if not raw.strip():
        return default
    return [s.strip() for s in raw.split(",") if s.strip()]


async def main() -> None:
    board = os.environ.get("SCRAPE_BOARD", "glassdoor")
    keyword = os.environ.get("SCRAPE_KEYWORD", "software engineer")
    location = os.environ.get("SCRAPE_LOCATION", "sweden")
    user_ids = _env_list("SCRAPE_USER_IDS", ["dry-run-user"])
    search_id = "dry-run-scan"

    # Build a real scraper service backed by python-jobspy (Glassdoor).
    scraper = GlassdoorJobspyScraper()
    service = ScraperService(
        configured_methods={"glassdoor": "glassdoor-jobspy"},
        scrapers={scraper.name: scraper},
    )

    # Wire only the scraper usecase into the activity container.
    mock_container = MagicMock()
    mock_container.usecase.scraper = service
    init_job_scrape_container(mock_container)

    try:
        input_data: dict[str, Any] = {
            "board": board,
            "keyword": keyword,
            "location": location,
            "user_ids": user_ids,
            "search_id": search_id,
        }

        print(
            f"=== DRY RUN: scrape_scan board={board} keyword={keyword} location={location} ==="
        )
        scan = await scrape_scan(input_data)
        print(json.dumps(scan, indent=2, default=str))

        print(f"\n=== DRY RUN: enrich_jobs (recency filter, 24h) ===")
        enriched = enrich_jobs(
            {
                "user_ids": user_ids,
                "search_id": search_id,
                "raw_jobs": scan["jobs"],
                "errors": scan["errors"],
            }
        )
        print(json.dumps(enriched, indent=2, default=str))

        print(f"\n=== DRY RUN SUMMARY ===")
        print(f"boards_run: {scan.get('boards')}")
        print(f"raw_found: {enriched.get('found')}")
        print(f"filtered_out_recency: {enriched.get('filtered_out_recency')}")
        print(f"enriched_jobs: {len(enriched.get('jobs', []))}")
        print(f"scan_errors: {len(scan.get('errors', []))}")
        print("\n(skipped ingest_jobs + queue_analysis — nothing written to DB)")
    finally:
        init_job_scrape_container(None)


if __name__ == "__main__":
    asyncio.run(main())
