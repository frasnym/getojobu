"""Unit tests for JobScrape activities and workflow."""

from datetime import UTC, datetime, timedelta
from typing import Any
from unittest.mock import MagicMock, patch

import pytest
from deliveries.consumer.activities.job_scrape import (
    build_job_slug,
    enrich_jobs,
    ingest_jobs,
    queue_analysis,
    scrape_scan,
    slugify,
)


def test_slugify_and_build_job_slug():
    assert slugify("Senior Go Developer") == "senior-go-developer"
    assert slugify(" Acme, Inc. ") == "acme-inc"
    assert slugify("Dublin, Ireland") == "dublin-ireland"

    slug = build_job_slug("Senior Go Developer", "Acme, Inc.", "Dublin, Ireland")
    assert slug == "senior-go-developer-acme-inc-dublin-ireland"

    fallback = build_job_slug("", "", "")
    assert fallback == "unknown-title-unknown-company-unspecified-location"


def test_enrich_jobs_drops_missing_required_fields():
    now_iso = datetime.now(UTC).isoformat()
    raw_jobs = [
        {
            "title": None,
            "company": "Acme",
            "location": "Dublin",
            "date_posted": now_iso,
        },
        {"title": "Dev", "company": None, "location": "Dublin", "date_posted": now_iso},
        {"title": "Dev", "company": "Acme", "location": None, "date_posted": now_iso},
        {"title": "Dev", "company": "Acme", "location": "Dublin", "date_posted": None},
        {
            "title": "Valid Dev",
            "company": "Acme",
            "location": "ireland",
            "date_posted": now_iso,
            "site": "glassdoor",
            "description": "Great job",
        },
    ]

    res = enrich_jobs({"user_id": "test-user", "search_id": "s1", "raw_jobs": raw_jobs})
    assert res["found"] == 5
    assert len(res["jobs"]) == 1

    job = res["jobs"][0]
    assert job["title"] == "Valid Dev"
    assert job["company"] == "Acme"
    assert job["location"] == "Ireland"  # Normalized
    assert job["source"] == "glassdoor"
    assert job["user_id"] == "test-user"
    assert job["updated_by_type"] == "cron"
    assert job["updated_by_id"] == "cron-scrape"
    assert job["slug"] == "valid-dev-acme-ireland"


def test_enrich_jobs_recency_filter_and_sorting():
    now = datetime.now(UTC)
    recent_1h = (now - timedelta(hours=1)).isoformat()
    recent_2h = (now - timedelta(hours=2)).isoformat()
    too_old_2d = (now - timedelta(days=2)).isoformat()

    raw_jobs = [
        {
            "title": "Older",
            "company": "Co",
            "location": "Norway",
            "date_posted": recent_2h,
        },
        {
            "title": "Expired",
            "company": "Co",
            "location": "Norway",
            "date_posted": too_old_2d,
        },
        {
            "title": "Newer",
            "company": "Co",
            "location": "Norway",
            "date_posted": recent_1h,
        },
    ]

    res = enrich_jobs({"user_id": "admin", "search_id": "s1", "raw_jobs": raw_jobs})
    assert res["found"] == 3
    assert res["filtered_out_recency"] == 1
    assert len(res["jobs"]) == 2

    # Verify sorted DESC by posted_date (Newer before Older)
    assert res["jobs"][0]["title"] == "Newer"
    assert res["jobs"][1]["title"] == "Older"
    assert res["jobs"][0]["location"] == "Norway"


def test_enrich_jobs_location_normalization():
    now_iso = datetime.now(UTC).isoformat()
    raw_jobs = [
        {
            "title": "D1",
            "company": "C1",
            "location": "ireland",
            "date_posted": now_iso,
            "site": "glassdoor",
        },
        {
            "title": "D2",
            "company": "C2",
            "location": "norway",
            "date_posted": now_iso,
            "site": "linkedin",
        },
        {
            "title": "D3",
            "company": "C3",
            "location": "Unknown City",
            "date_posted": now_iso,
            "site": "glassdoor",
        },
    ]

    res = enrich_jobs({"user_id": "admin", "raw_jobs": raw_jobs})
    jobs = res["jobs"]
    assert len(jobs) == 3
    locs = {j["title"]: j["location"] for j in jobs}
    assert locs["D1"] == "Ireland"
    assert locs["D2"] == "Norway"
    assert locs["D3"] == "Unknown City"


def test_scrape_scan_with_mocked_jobspy():
    mock_jobspy = MagicMock()
    mock_jobspy.SUPPORTED_SITES = {"glassdoor", "linkedin"}

    class FakeDataFrame:
        def to_dict(self, orient="records"):
            return [
                {
                    "title": "Backend Go",
                    "company": "Acme",
                    "location": "Ireland",
                    "date_posted": datetime.now(UTC).isoformat(),
                    "site": "glassdoor",
                }
            ]

    mock_jobspy.scrape_jobs.return_value = FakeDataFrame()

    with patch("deliveries.consumer.activities.job_scrape.jobspy", mock_jobspy):
        input_data = {
            "user_id": "admin",
            "search_id": "s1",
            "searches": [
                {
                    "keywords": ["golang"],
                    "locations": ["ireland"],
                    "boards": ["glassdoor"],
                }
            ],
        }
        res = scrape_scan(input_data)
        assert len(res["jobs"]) == 1
        assert res["jobs"][0]["title"] == "Backend Go"
        assert res["boards"] == ["glassdoor"]
        assert len(res["errors"]) == 0
        assert mock_jobspy.scrape_jobs.call_count == 1


def test_scrape_scan_board_error_resilience():
    mock_jobspy = MagicMock()
    mock_jobspy.SUPPORTED_SITES = {"glassdoor", "linkedin"}
    mock_jobspy.scrape_jobs.side_effect = RuntimeError("429 Too Many Requests")

    with patch("deliveries.consumer.activities.job_scrape.jobspy", mock_jobspy):
        input_data = {
            "user_id": "admin",
            "search_id": "s1",
            "searches": [
                {
                    "keywords": ["golang"],
                    "locations": ["ireland"],
                    "boards": ["glassdoor"],
                }
            ],
        }
        res = scrape_scan(input_data)
        assert len(res["jobs"]) == 0
        assert len(res["errors"]) == 1
        assert res["errors"][0]["board"] == "glassdoor"
        assert "429" in res["errors"][0]["error"]


@pytest.mark.asyncio
async def test_ingest_jobs_bulk_post():
    fake_client = MagicMock()

    async def fake_post(path: str, json: Any) -> Any:
        assert path == "/v1/jobs/bulk"
        jobs_in = json["jobs"]
        assert len(jobs_in) == 2
        assert jobs_in[0]["user_id"] == "user-123"
        return [
            {"slug": jobs_in[0]["slug"], "created": True},
            {"slug": jobs_in[1]["slug"], "created": False},
        ]

    async def fake_aclose() -> None:
        pass

    fake_client.post.side_effect = fake_post
    fake_client.aclose.side_effect = fake_aclose

    with patch(
        "deliveries.consumer.activities.job_scrape.NewGetojobuAPI",
        return_value=fake_client,
    ):
        input_data = {
            "user_id": "user-123",
            "search_id": "scan-1",
            "jobs": [
                {"slug": "job-1", "title": "Job 1"},
                {"slug": "job-2", "title": "Job 2"},
            ],
        }
        res = await ingest_jobs(input_data)
        assert res["inserted"] == 1
        assert res["skipped"] == 1
        assert res["created_slugs"] == ["job-1"]


@pytest.mark.asyncio
async def test_queue_analysis_bulk():
    fake_client = MagicMock()

    async def fake_post(path: str, json: Any) -> Any:
        assert path == "/v1/jobs/bulk-queue-analyze"
        assert isinstance(json, list)
        assert json[0]["slug"] == "job-1"
        assert json[0]["force"] is True
        return {"workflow_ids": ["job-analyze-1"]}

    async def fake_aclose() -> None:
        pass

    fake_client.post.side_effect = fake_post
    fake_client.aclose.side_effect = fake_aclose

    with patch(
        "deliveries.consumer.activities.job_scrape.NewGetojobuAPI",
        return_value=fake_client,
    ):
        input_data = {
            "user_id": "admin",
            "search_id": "scan-1",
            "created_job_slugs": ["job-1", "job-2"],
        }
        res = await queue_analysis(input_data)
        assert res["started"] == 1
        assert res["failed"] == ["job-2"]
