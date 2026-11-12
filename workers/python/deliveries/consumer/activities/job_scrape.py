"""Temporal activity wrappers for the JobScrapeWorkflow.

    scrape_scan    — issue jobspy.scrape_jobs() per board, collect raw results
    enrich_jobs    — normalize, recency-filter, map to job rows
    ingest_jobs    — call Go API POST /v1/jobs/bulk (chunked)
    queue_analysis — start JobAnalyzeWorkflow for newly created jobs

Error strategy: per-site errors are accumulated and logged rather than failing
the scan, so a transient board block does not kill the whole scan.
"""

from __future__ import annotations

import os
import re
from datetime import UTC, date, datetime, time, timedelta
from typing import Any, cast

jobspy: Any = None
try:
    import jobspy  # type: ignore  # pyright: ignore[reportMissingImports]
except ImportError:  # pragma: no cover - optional dependency in host env
    jobspy = None

from repository.getojobu_api import NewGetojobuAPI
from temporalio import activity
from utils.logging import get_logger

logger = get_logger("activities.job_scrape")

# Location normalizer: owner input -> board-specific location string.
# v1: country-only. Add city rows here when needed.
LOCATION_MAP: dict[str, dict[str, str]] = {
    "ireland": {"glassdoor": "Ireland", "linkedin": "Ireland"},
    "norway": {"glassdoor": "Norway", "linkedin": "Norway"},
    "indonesia": {"glassdoor": "Indonesia", "linkedin": "Indonesia"},
    "malaysia": {"glassdoor": "Malaysia", "linkedin": "Malaysia"},
}

# Recency: jobs older than this are dropped by enrich_jobs.
RECENT_DAYS = 1


def slugify(s: str) -> str:
    """Trim, lowercase, replace non-alphanumeric runs with hyphens."""
    s = s.strip().lower()
    s = re.sub(r"[^a-z0-9]+", "-", s)
    return s.strip("-")


def build_job_slug(title: str, company: str, location: str) -> str:
    """Generate slug matching Go models.BuildJobSlug."""
    t = slugify(title) or "unknown-title"
    c = slugify(company) or "unknown-company"
    l = slugify(location) or "unspecified-location"
    return f"{t}-{c}-{l}"


def _safe_heartbeat(**details: Any) -> None:
    if activity.in_activity():
        try:
            activity.heartbeat(details)
        except Exception as e:  # noqa: BLE001
            logger.debug("heartbeat_failed", error=f"{e}")


@activity.defn
def scrape_scan(input_data: dict[str, Any]) -> dict[str, Any]:
    """Issue scrape_jobs() calls per keyword × location × enabled board.

    Boards run sequentially within a scan so one board's rate limits do not
    block another. Per-site errors are logged into attrs["errors"] rather than
    failing the entire scan.

    Returns:
        {
            "jobs": [raw job dicts],
            "boards": ["glassdoor", ...],
            "errors": [{"board", "error", "location", "search_term"}],
        }
    """
    user_id_raw = input_data.get("user_id")
    user_id: str = user_id_raw if isinstance(user_id_raw, str) else "admin"
    search_id_raw = input_data.get("search_id")
    search_id: str = search_id_raw if isinstance(search_id_raw, str) else ""

    raw_searches = input_data.get("searches")
    searches: list[dict[str, Any]] = (
        cast(list[dict[str, Any]], raw_searches)
        if isinstance(raw_searches, list)
        else []
    )

    all_jobs: list[dict[str, Any]] = []
    boards_run: list[str] = []
    errors: list[dict[str, Any]] = []

    if jobspy is None:
        logger.warning(
            "jobspy_not_installed",
            user_id=user_id,
            scan_id=search_id,
        )
        return {"jobs": [], "boards": [], "errors": [{"error": "jobspy not installed"}]}

    js: Any = jobspy

    for src in searches:
        raw_boards = src.get("boards")
        boards: list[str] = (
            cast(list[str], raw_boards)
            if isinstance(raw_boards, list)
            else ["glassdoor"]
        )
        if not boards:
            boards = ["glassdoor"]

        raw_keywords = src.get("keywords")
        keywords: list[str] = (
            cast(list[str], raw_keywords)
            if isinstance(raw_keywords, list)
            else []
        )
        raw_locations = src.get("locations")
        locations: list[str] = (
            cast(list[str], raw_locations)
            if isinstance(raw_locations, list)
            else []
        )

        for keyword in keywords:
            for location in locations:
                for board in boards:
                    supported = getattr(
                        js,
                        "SUPPORTED_SITES",
                        {"glassdoor", "linkedin", "indeed", "zip_recruiter"},
                    )
                    if board not in supported:
                        logger.info(
                            "board not in JobSpy, skipping",
                            user_id=user_id,
                            scan_id=search_id,
                            board=board,
                        )
                        continue

                    try:
                        logger.info(
                            "scraping",
                            user_id=user_id,
                            scan_id=search_id,
                            board=board,
                            search_term=keyword,
                            location=location,
                            results_wanted=20,
                        )

                        result = js.scrape_jobs(
                            site_name=board,
                            search_term=keyword,
                            location=location,
                            results_wanted=20,
                            hours_old=24,
                            description_format="plain",
                        )

                        records: list[dict[str, Any]] = []
                        if hasattr(result, "to_dict"):
                            raw_records = result.to_dict("records")
                            if isinstance(raw_records, list):
                                records = cast(list[dict[str, Any]], raw_records)
                        elif isinstance(result, list):
                            records = cast(list[dict[str, Any]], result)

                        for r in records:
                            if not r.get("site"):
                                r["site"] = board
                        all_jobs.extend(records)
                        if board not in boards_run:
                            boards_run.append(board)

                        logger.info(
                            "scrape complete",
                            user_id=user_id,
                            scan_id=search_id,
                            board=board,
                            found=len(records),
                        )

                    except Exception as e:
                        logger.warning(
                            "scrape failed for board",
                            user_id=user_id,
                            scan_id=search_id,
                            board=board,
                            location=location,
                            error=f"{e}",
                            exc_info=True,
                        )
                        errors.append(
                            {
                                "board": board,
                                "error": f"{e}",
                                "location": location,
                                "search_term": keyword,
                            }
                        )

                    _safe_heartbeat(
                        jobs_found=len(all_jobs),
                        boards_run=len(boards_run),
                        errors=len(errors),
                    )

    return {"jobs": all_jobs, "boards": boards_run, "errors": errors}


@activity.defn
def enrich_jobs(input_data: dict[str, Any]) -> dict[str, Any]:
    """Clean, normalize, and recency-filter raw JobSpy results.

    Drops rows missing title/company/location, enforces posted within 24h,
    normalizes location via the lookup table, maps fields to job rows, and
    attaches audit attribution (updated_by_type="cron", updated_by_id="cron-scrape").

    Returns:
        {
            "jobs": [{user_id, slug, source, source_url, apply_url, title, company,
                      location, description, posted_date, is_remote,
                      updated_by_type, updated_by_id}],
            "found": int,
            "filtered_out_recency": int,
            "errors": [...],
        }
    """
    user_id_raw = input_data.get("user_id")
    user_id: str = user_id_raw if isinstance(user_id_raw, str) else "admin"
    search_id_raw = input_data.get("search_id")
    search_id: str = search_id_raw if isinstance(search_id_raw, str) else ""

    raw_jobs = input_data.get("raw_jobs")
    raw: list[dict[str, Any]] = (
        cast(list[dict[str, Any]], raw_jobs)
        if isinstance(raw_jobs, list)
        else []
    )
    raw_errors = input_data.get("errors")
    errors: list[dict[str, Any]] = (
        cast(list[dict[str, Any]], raw_errors)
        if isinstance(raw_errors, list)
        else []
    )

    cut = datetime.now(UTC) - timedelta(days=RECENT_DAYS)
    jobs: list[dict[str, Any]] = []
    filtered_recency = 0
    found = 0

    for row in raw:
        try:
            found += 1

            title_val = row.get("title")
            company_val = row.get("company")
            location_val = row.get("location")

            if (
                not isinstance(title_val, str)
                or not isinstance(company_val, str)
                or not isinstance(location_val, str)
            ):
                logger.debug(
                    "drop row missing required field",
                    user_id=user_id,
                    scan_id=search_id,
                    row_title=title_val,
                    row_company=company_val,
                    row_location=location_val,
                )
                continue

            title_str: str = title_val.strip()
            company_str: str = company_val.strip()
            raw_location: str = location_val.strip()

            if not title_str or not company_str or not raw_location:
                continue

            posted_raw = row.get("date_posted") or row.get("posted_date")
            if posted_raw is None:
                continue

            posted: datetime | None = None
            if isinstance(posted_raw, datetime):
                posted = posted_raw
            elif isinstance(posted_raw, date):
                posted = datetime.combine(posted_raw, time.min, tzinfo=UTC)
            elif isinstance(posted_raw, str):
                cleaned = posted_raw.replace("Z", "+00:00")
                posted = datetime.fromisoformat(cleaned)

            if posted is None:
                continue

            if posted.tzinfo is None:
                posted = posted.replace(tzinfo=UTC)

            if posted < cut:
                filtered_recency += 1
                continue

            site_val = row.get("site")
            board: str = (site_val if isinstance(site_val, str) else "glassdoor").lower()

            # Normalize location per board lookup.
            normalized_map = LOCATION_MAP.get(raw_location.lower(), {})
            loc_str: str = normalized_map.get(board, raw_location).strip()

            source_url_val = row.get("job_url_direct") or row.get("job_url")
            source_url: str = source_url_val if isinstance(source_url_val, str) else ""
            apply_url_val = row.get("apply_url")
            apply_url: str = apply_url_val if isinstance(apply_url_val, str) else source_url

            is_remote_val = row.get("is_remote")
            is_remote: bool = is_remote_val is True

            jobs.append(
                {
                    "user_id": user_id,
                    "slug": build_job_slug(title_str, company_str, loc_str),
                    "source": board,
                    "source_url": source_url,
                    "apply_url": apply_url,
                    "title": title_str,
                    "company": company_str,
                    "location": loc_str,
                    "description": row.get("description"),
                    "posted_date": posted.isoformat(),
                    "is_remote": is_remote,
                    "updated_by_type": "cron",
                    "updated_by_id": "cron-scrape",
                }
            )

        except Exception as e:
            logger.warning(
                "failed to enrich one job row, skipping",
                user_id=user_id,
                scan_id=search_id,
                error=f"{e}",
                exc_info=True,
            )

    def _posted_date_key(j: dict[str, Any]) -> str:
        pd = j.get("posted_date")
        return pd if isinstance(pd, str) else ""

    jobs.sort(key=_posted_date_key, reverse=True)

    _safe_heartbeat(
        found=found,
        filtered_out_recency=filtered_recency,
        inserted=len(jobs),
    )

    return {
        "jobs": jobs,
        "found": found,
        "filtered_out_recency": filtered_recency,
        "errors": errors,
    }


@activity.defn
async def ingest_jobs(input_data: dict[str, Any]) -> dict[str, Any]:
    """Call the Go API POST /v1/jobs/bulk, chunked at 100 rows.

    Conflicting slugs are handled as first-source-wins (created: false).

    Returns:
        {
            "inserted": int,
            "skipped": int,
            "created_slugs": [str],
        }
    """
    user_id_raw = input_data.get("user_id")
    user_id: str = user_id_raw if isinstance(user_id_raw, str) else "admin"
    search_id_raw = input_data.get("search_id")
    search_id: str = search_id_raw if isinstance(search_id_raw, str) else ""

    raw_jobs = input_data.get("jobs")
    rows: list[dict[str, Any]] = (
        cast(list[dict[str, Any]], raw_jobs)
        if isinstance(raw_jobs, list)
        else []
    )

    client = NewGetojobuAPI(
        base_url=os.environ.get("API_BASE_URL", "http://localhost:8080"),
        api_key=os.environ.get("API_KEY", ""),
    )

    try:
        inserted = 0
        skipped = 0
        created_slugs: list[str] = []

        for i in range(0, len(rows), 100):
            chunk = rows[i : i + 100]
            # Ensure every row carries the authenticated owner user_id for API key auth
            for item in chunk:
                if not item.get("user_id"):
                    item["user_id"] = user_id
            payload: dict[str, Any] = {"jobs": chunk}
            resp = await client.post("/v1/jobs/bulk", json=payload)

            results: list[dict[str, Any]] = []
            if isinstance(resp, list):
                results = cast(list[dict[str, Any]], resp)
            elif isinstance(resp, dict):
                resp_dict = cast(dict[str, Any], resp)
                raw_results = resp_dict.get("results") or resp_dict.get("jobs")
                if isinstance(raw_results, list):
                    results = cast(list[dict[str, Any]], raw_results)

            for res in results:
                if res.get("created"):
                    inserted += 1
                    slug = res.get("slug")
                    if isinstance(slug, str) and slug:
                        created_slugs.append(slug)
                else:
                    skipped += 1

            _safe_heartbeat(found=len(rows), inserted=inserted, skipped=skipped)

        logger.info(
            "ingestion complete",
            user_id=user_id,
            scan_id=search_id,
            inserted=inserted,
            skipped=skipped,
        )

        return {
            "inserted": inserted,
            "skipped": skipped,
            "created_slugs": created_slugs,
        }

    finally:
        await client.aclose()


@activity.defn
async def queue_analysis(input_data: dict[str, Any]) -> dict[str, Any]:
    """Start JobAnalyzeWorkflow for each newly created job via the Go API.

    POST /v1/jobs/bulk-queue-analyze accepts an array of [{"slug": s, "force": True}]
    and returns {"workflow_ids": [...]}.
    """
    user_id_raw = input_data.get("user_id")
    user_id: str = user_id_raw if isinstance(user_id_raw, str) else "admin"
    search_id_raw = input_data.get("search_id")
    search_id: str = search_id_raw if isinstance(search_id_raw, str) else ""

    slugs_input = input_data.get("created_job_slugs")
    slugs: list[str] = (
        cast(list[str], slugs_input)
        if isinstance(slugs_input, list)
        else []
    )

    client = NewGetojobuAPI(
        base_url=os.environ.get("API_BASE_URL", "http://localhost:8080"),
        api_key=os.environ.get("API_KEY", ""),
    )

    try:
        started = 0
        failed: list[str] = []

        for chunk_i in range(0, len(slugs), 20):
            chunk = slugs[chunk_i : chunk_i + 20]
            payload: list[dict[str, Any]] = [
                {"slug": slug, "force": True} for slug in chunk
            ]
            resp = await client.post("/v1/jobs/bulk-queue-analyze", json=payload)
            workflow_ids: list[str] = []
            if isinstance(resp, dict):
                resp_dict = cast(dict[str, Any], resp)
                raw_ids = resp_dict.get("workflow_ids")
                if isinstance(raw_ids, list):
                    workflow_ids = cast(list[str], raw_ids)

            # Count started workflows
            chunk_started = len(workflow_ids)
            started += chunk_started
            if chunk_started < len(chunk):
                # Slugs beyond the returned count are recorded as failed
                failed.extend(chunk[chunk_started:])

            _safe_heartbeat(queued=started, failed=len(failed))

        if failed:
            logger.warning(
                "queue analysis failed for some slugs",
                user_id=user_id,
                scan_id=search_id,
                failed_slugs=failed,
            )

        logger.info(
            "queue analysis complete",
            user_id=user_id,
            scan_id=search_id,
            started=started,
            failed=len(failed),
        )

        return {"started": started, "failed": failed}

    finally:
        await client.aclose()
