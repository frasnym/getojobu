"""Temporal activity wrappers for the JobScrapeWorkflow.

    scrape_scan    — issue jobspy.scrape_jobs() per board, collect raw results
    enrich_jobs    — normalize, recency-filter, map to job rows
    ingest_jobs    — call Go API POST /v1/jobs/bulk (chunked)
    queue_analysis — start JobAnalyzeWorkflow for newly created jobs

Error strategy: per-site errors are accumulated and logged rather than failing
the scan, so a transient board block does not kill the whole scan.
"""

from __future__ import annotations

import json
import math
import os
import re
from collections.abc import Iterable
from datetime import UTC, date, datetime, time, timedelta
from typing import Any, cast

from temporalio import activity

from config.config import load as config_load
from contract.contract import WorkerContainer
from contract.contract import new as contract_new
from repository.getojobu_api import NewGetojobuAPI
from utils.logging import get_logger

logger = get_logger("activities.job_scrape")

_container: WorkerContainer | None = None


def init_job_scrape_container(
    container: WorkerContainer | None = None,
) -> WorkerContainer | None:
    """Build (or reuse) the dependency container.

    Called from worker.main before starting the Temporal worker.
    """
    global _container
    if container is not None:
        _container = container
        return _container

    if _container is None:
        try:
            cfg = config_load()
            c, _stopper, err = contract_new(cfg)
            if err is None and c is not None:
                _container = c
        except Exception as e:  # noqa: BLE001
            logger.debug("init_job_scrape_container_fallback", error=str(e))
    return _container


# Location normalizer: owner input -> board-specific location string.
# v1: country-only. Add city rows here when needed.
LOCATION_MAP: dict[str, dict[str, str]] = {
    "austria": {"glassdoor": "Austria", "linkedin": "Austria"},
    "australia": {"glassdoor": "Australia", "linkedin": "Australia"},
    "belgium": {"glassdoor": "Belgium", "linkedin": "Belgium"},
    "canada": {"glassdoor": "Canada", "linkedin": "Canada"},
    "denmark": {"glassdoor": "Denmark", "linkedin": "Denmark"},
    "finland": {"glassdoor": "Finland", "linkedin": "Finland"},
    "france": {"glassdoor": "France", "linkedin": "France"},
    "germany": {"glassdoor": "Germany", "linkedin": "Germany"},
    "indonesia": {"glassdoor": "Indonesia", "linkedin": "Indonesia"},
    "ireland": {"glassdoor": "Ireland", "linkedin": "Ireland"},
    "japan": {"glassdoor": "Japan", "linkedin": "Japan"},
    "malaysia": {"glassdoor": "Malaysia", "linkedin": "Malaysia"},
    "netherlands": {"glassdoor": "Netherlands", "linkedin": "Netherlands"},
    "norway": {"glassdoor": "Norway", "linkedin": "Norway"},
    "poland": {"glassdoor": "Poland", "linkedin": "Poland"},
    "portugal": {"glassdoor": "Portugal", "linkedin": "Portugal"},
    "singapore": {"glassdoor": "Singapore", "linkedin": "Singapore"},
    "spain": {"glassdoor": "Spain", "linkedin": "Spain"},
    "sweden": {"glassdoor": "Sweden", "linkedin": "Sweden"},
    "switzerland": {"glassdoor": "Switzerland", "linkedin": "Switzerland"},
    "united arab emirates": {
        "glassdoor": "United Arab Emirates",
        "linkedin": "United Arab Emirates",
    },
    "united kingdom": {"glassdoor": "United Kingdom", "linkedin": "United Kingdom"},
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


def parse_user_ids(raw: object) -> list[str]:
    """Extract and validate a list of non-empty user id strings."""
    if not isinstance(raw, list):
        return []
    result: list[str] = []
    for uid in cast(list[object], raw):
        if isinstance(uid, str) and uid.strip():
            result.append(uid.strip())
    return result


def _sanitize_for_json(val: Any) -> Any:
    """Recursively convert dates, timestamps, NaNs and custom types to JSON-safe primitives."""
    if val is None:
        return None
    if "NaT" in val.__class__.__name__:
        return None
    if isinstance(val, (datetime, date)):
        return val.isoformat()
    if isinstance(val, float):
        if math.isnan(val) or math.isinf(val):
            return None
        return val
    if isinstance(val, (int, str, bool)):
        return val
    if isinstance(val, dict):
        return {
            f"{k}": _sanitize_for_json(v)
            for k, v in cast(dict[object, object], val).items()
        }
    if isinstance(val, (list, tuple, set)):
        return [_sanitize_for_json(item) for item in cast(Iterable[object], val)]
    if hasattr(val, "item"):
        try:
            return _sanitize_for_json(val.item())
        except Exception:  # noqa: BLE001, S110
            pass
    if hasattr(val, "isoformat"):
        try:
            return val.isoformat()
        except Exception:  # noqa: BLE001, S110
            pass
    try:
        json.dumps(val)
        return val
    except (TypeError, ValueError):
        return str(val)


def _safe_heartbeat(**details: Any) -> None:
    if activity.in_activity():
        try:
            activity.heartbeat(details)
        except Exception as e:  # noqa: BLE001
            logger.debug("heartbeat_failed", error=f"{e}")


@activity.defn
async def scrape_scan(input_data: dict[str, Any]) -> dict[str, Any]:
    """Issue scrape calls per keyword × location × enabled board scraper method.

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
    user_ids = parse_user_ids(input_data.get("user_ids"))

    search_id_raw = input_data.get("search_id")
    search_id: str = search_id_raw if isinstance(search_id_raw, str) else ""

    board_val = input_data.get("board")
    board: str = board_val.strip().lower() if isinstance(board_val, str) else ""
    if not board:
        raise ValueError("board is required and cannot be empty")
    keyword_val = input_data.get("keyword")
    keyword: str = keyword_val if isinstance(keyword_val, str) else ""
    location_val = input_data.get("location")
    location: str = location_val if isinstance(location_val, str) else ""

    all_jobs: list[dict[str, Any]] = []
    boards_run: list[str] = []
    errors: list[dict[str, Any]] = []

    container = init_job_scrape_container()
    scraper_service = (
        container.usecase.scraper if container and container.usecase else None
    )

    if keyword and location:
        if scraper_service is not None:
            try:
                scraper = scraper_service.resolve(board)
                if scraper is None:
                    logger.info(
                        "board scraper disabled, skipping",
                        user_ids=user_ids,
                        scan_id=search_id,
                        board=board,
                    )
                else:
                    logger.info(
                        "scraping",
                        user_ids=user_ids,
                        scan_id=search_id,
                        board=board,
                        method=scraper.name,
                        search_term=keyword,
                        location=location,
                    )
                    records = await scraper_service.scrape(board, keyword, location)
                    sanitized_records: list[dict[str, Any]] = [
                        cast(dict[str, Any], _sanitize_for_json(r)) for r in records
                    ]
                    all_jobs.extend(sanitized_records)
                    if board not in boards_run:
                        boards_run.append(board)

                    logger.info(
                        "scrape complete",
                        user_ids=user_ids,
                        scan_id=search_id,
                        board=board,
                        found=len(records),
                    )
            except Exception as e:  # noqa: BLE001
                logger.warning(
                    "scrape failed for board",
                    user_ids=user_ids,
                    scan_id=search_id,
                    board=board,
                    location=location,
                    error=f"{e}",
                )
                errors.append(
                    {
                        "board": board,
                        "search_term": keyword,
                        "location": location,
                        "error": f"{e}",
                    }
                )
        else:
            err_msg = "scraper service is not initialized in worker container"
            logger.error("scraper_service_not_initialized", board=board)
            errors.append(
                {
                    "board": board,
                    "search_term": keyword,
                    "location": location,
                    "error": err_msg,
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
            "jobs": [{user_ids, slug, source, source_url, apply_url, title, company,
                      location, description, posted_date, is_remote,
                      updated_by_type, updated_by_id}],
            "found": int,
            "filtered_out_recency": int,
            "errors": [...],
        }
    """
    user_ids = parse_user_ids(input_data.get("user_ids"))

    search_id_raw = input_data.get("search_id")
    search_id: str = search_id_raw if isinstance(search_id_raw, str) else ""

    raw_jobs = input_data.get("raw_jobs")
    raw: list[dict[str, Any]] = (
        cast(list[dict[str, Any]], raw_jobs) if isinstance(raw_jobs, list) else []
    )
    raw_errors = input_data.get("errors")
    errors: list[dict[str, Any]] = (
        cast(list[dict[str, Any]], raw_errors) if isinstance(raw_errors, list) else []
    )

    cut = datetime.now(UTC) - timedelta(days=RECENT_DAYS)
    jobs: list[tuple[datetime, dict[str, Any]]] = []
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
                    user_ids=user_ids,
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
            if posted_raw is None or (
                isinstance(posted_raw, str) and not posted_raw.strip()
            ):
                continue

            posted: datetime | None = None
            if isinstance(posted_raw, datetime):
                posted = posted_raw
            elif isinstance(posted_raw, date):
                posted = datetime.combine(posted_raw, time.min, tzinfo=UTC)
            elif isinstance(posted_raw, str):
                cleaned = posted_raw.strip().replace("Z", "+00:00")
                try:
                    posted = datetime.fromisoformat(cleaned)
                except ValueError:
                    continue

            if posted is None:
                continue

            if posted.tzinfo is None:
                posted = posted.replace(tzinfo=UTC)

            if posted < cut:
                filtered_recency += 1
                continue

            site_val = row.get("site") or input_data.get("board")
            if not isinstance(site_val, str) or not site_val.strip():
                logger.debug(
                    "drop row missing site",
                    user_ids=user_ids,
                    scan_id=search_id,
                )
                continue
            board: str = site_val.strip().lower()

            # Normalize location per board lookup.
            normalized_map = LOCATION_MAP.get(raw_location.lower(), {})
            loc_str: str = normalized_map.get(board, raw_location).strip()

            source_url_val = row.get("job_url_direct") or row.get("job_url")
            source_url: str = source_url_val if isinstance(source_url_val, str) else ""
            apply_url_val = row.get("apply_url")
            apply_url: str = (
                apply_url_val if isinstance(apply_url_val, str) else source_url
            )

            is_remote_val = row.get("is_remote")
            is_remote: bool = is_remote_val is True

            jobs.append(
                (
                    posted,
                    {
                        "user_ids": user_ids,
                        "slug": build_job_slug(title_str, company_str, loc_str),
                        "source": board,
                        "source_url": source_url,
                        "apply_url": apply_url,
                        "title": title_str,
                        "company": company_str,
                        "location": loc_str,
                        "description": row.get("description"),
                        "posted_date": posted.strftime("%Y-%m-%d"),
                        "is_remote": is_remote,
                        "updated_by_type": "cron",
                        "updated_by_id": "cron-scrape",
                    },
                )
            )

        except Exception as e:
            logger.warning(
                "failed to enrich one job row, skipping",
                user_ids=user_ids,
                scan_id=search_id,
                error=f"{e}",
                exc_info=True,
            )

    jobs.sort(key=lambda item: item[0], reverse=True)
    sorted_jobs: list[dict[str, Any]] = [item[1] for item in jobs]

    _safe_heartbeat(
        found=found,
        filtered_out_recency=filtered_recency,
        inserted=len(sorted_jobs),
    )

    return {
        "jobs": sorted_jobs,
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
    user_ids = parse_user_ids(input_data.get("user_ids"))

    search_id_raw = input_data.get("search_id")
    search_id: str = search_id_raw if isinstance(search_id_raw, str) else ""

    raw_jobs = input_data.get("jobs")
    rows: list[dict[str, Any]] = (
        cast(list[dict[str, Any]], raw_jobs) if isinstance(raw_jobs, list) else []
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
            # Ensure every row carries user_ids for API key auth
            for item in chunk:
                if not item.get("user_ids") and user_ids:
                    item["user_ids"] = user_ids
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

            chunk_slug_can_analyze: dict[str, bool] = {}
            for item in chunk:
                item_slug = str(item.get("slug") or "")
                if "description" in item:
                    desc_val = item.get("description")
                    chunk_slug_can_analyze[item_slug] = bool(
                        desc_val and str(desc_val).strip()
                    )
                else:
                    chunk_slug_can_analyze[item_slug] = True

            for res in results:
                if res.get("created"):
                    inserted += 1
                    slug = res.get("slug")
                    if (
                        isinstance(slug, str)
                        and slug
                        and chunk_slug_can_analyze.get(slug, True)
                    ):
                        created_slugs.append(slug)
                else:
                    skipped += 1

            _safe_heartbeat(found=len(rows), inserted=inserted, skipped=skipped)

        logger.info(
            "ingestion complete",
            user_ids=user_ids,
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
    user_ids = parse_user_ids(input_data.get("user_ids"))

    search_id_raw = input_data.get("search_id")
    search_id: str = search_id_raw if isinstance(search_id_raw, str) else ""

    slugs_input = input_data.get("created_job_slugs")
    slugs: list[str] = (
        cast(list[str], slugs_input) if isinstance(slugs_input, list) else []
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
            try:
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
            except Exception as e:  # noqa: BLE001
                logger.warning(
                    "bulk queue analyze chunk failed, retrying items individually",
                    user_ids=user_ids,
                    scan_id=search_id,
                    chunk_size=len(chunk),
                    error=f"{e}",
                )
                for slug in chunk:
                    try:
                        single_resp = await client.post(
                            "/v1/jobs/bulk-queue-analyze",
                            json=[{"slug": slug, "force": True}],
                        )
                        single_ids: list[str] = []
                        if isinstance(single_resp, dict):
                            s_dict = cast(dict[str, Any], single_resp)
                            raw_s_ids = s_dict.get("workflow_ids")
                            if isinstance(raw_s_ids, list):
                                single_ids = cast(list[str], raw_s_ids)
                        if single_ids:
                            started += len(single_ids)
                        else:
                            failed.append(slug)
                    except Exception as single_err:  # noqa: BLE001
                        logger.warning(
                            "skipping job that cannot be queued for analysis",
                            user_ids=user_ids,
                            scan_id=search_id,
                            slug=slug,
                            error=f"{single_err}",
                        )
                        failed.append(slug)

            _safe_heartbeat(queued=started, failed=len(failed))

        if failed:
            logger.warning(
                "queue analysis failed for some slugs",
                user_ids=user_ids,
                scan_id=search_id,
                failed_slugs=failed,
            )

        logger.info(
            "queue analysis complete",
            user_ids=user_ids,
            scan_id=search_id,
            started=started,
            failed=len(failed),
        )

        return {"started": started, "failed": failed}

    finally:
        await client.aclose()
