"""Unit tests for JobScrape activities and workflow."""

import json
from datetime import UTC, date, datetime, timedelta
from typing import Any, cast
from unittest.mock import MagicMock, patch

import pytest
from deliveries.consumer.activities.job_scrape import (
    _sanitize_for_json,
    build_job_slug,
    enrich_jobs,
    ingest_jobs,
    parse_user_ids,
    queue_analysis,
    scrape_scan,
    slugify,
)
from workflows.job_scrape import JobScrapeWorkflow


@pytest.fixture(autouse=True)
def setup_job_scrape_container():
    from deliveries.consumer.activities.job_scrape import init_job_scrape_container
    from repository.scraper.glassdoor_jobspy import GlassdoorJobspyScraper
    from usecase.scraper.service import ScraperService

    s_gd = GlassdoorJobspyScraper()
    service = ScraperService(
        configured_methods={"glassdoor": "glassdoor-jobspy"},
        scrapers={s_gd.name: s_gd},
    )
    mock_container = MagicMock()
    mock_container.usecase.scraper = service
    init_job_scrape_container(mock_container)
    yield
    init_job_scrape_container(None)


def test_parse_user_ids():
    assert parse_user_ids(["userA", "userB"]) == ["userA", "userB"]
    assert parse_user_ids([" userA ", "", "  ", None, 123, "userB"]) == [
        "userA",
        "userB",
    ]
    assert parse_user_ids([]) == []
    assert parse_user_ids(None) == []
    assert parse_user_ids("not-a-list") == []


class _FakeNaT:
    pass


def test_sanitize_for_json():
    data: dict[str, object] = {
        "title": "Engineer",
        "date_posted": date(2026, 11, 12),
        "created_at": datetime(2026, 11, 12, 10, 0, 0),
        "nan_field": float("nan"),
        "inf_field": float("inf"),
        "none_field": None,
        "nat_field": _FakeNaT(),
        "nested": [{"inner_date": date(2026, 1, 1)}],
    }
    sanitized = cast(dict[str, object], _sanitize_for_json(data))
    # Must be serializable by stdlib json without TypeError
    encoded = json.dumps(sanitized)
    assert "2026-11-12" in encoded
    decoded = cast(dict[str, object], json.loads(encoded))
    assert decoded["date_posted"] == "2026-11-12"
    assert decoded["created_at"] == "2026-11-12T10:00:00"
    assert decoded["nan_field"] is None
    assert decoded["inf_field"] is None
    assert decoded["nat_field"] is None
    nested = cast(list[dict[str, object]], decoded["nested"])
    assert nested[0]["inner_date"] == "2026-01-01"


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
            "title": "Dev",
            "company": "Acme",
            "location": "Dublin",
            "date_posted": now_iso,
            "site": None,
        },
        {
            "title": "Dev",
            "company": "Acme",
            "location": "Dublin",
            "date_posted": now_iso,
            "site": "  ",
        },
        {
            "title": "Valid Dev",
            "company": "Acme",
            "location": "ireland",
            "date_posted": now_iso,
            "site": "glassdoor",
            "description": "Great job",
        },
    ]

    res = enrich_jobs(
        {"user_ids": ["test-user"], "search_id": "s1", "raw_jobs": raw_jobs}
    )
    assert res["found"] == 7
    assert len(res["jobs"]) == 1

    job = res["jobs"][0]
    assert job["title"] == "Valid Dev"
    assert job["company"] == "Acme"
    assert job["location"] == "Ireland"  # Normalized
    assert job["source"] == "glassdoor"
    assert job["user_ids"] == ["test-user"]
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

    res = enrich_jobs(
        {
            "board": "linkedin",
            "user_ids": ["admin"],
            "search_id": "s1",
            "raw_jobs": raw_jobs,
        }
    )
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

    res = enrich_jobs({"user_ids": ["admin"], "raw_jobs": raw_jobs})
    jobs = res["jobs"]
    assert len(jobs) == 3
    locs = {j["title"]: j["location"] for j in jobs}
    assert locs["D1"] == "Ireland"
    assert locs["D2"] == "Norway"
    assert locs["D3"] == "Unknown City"


@pytest.mark.asyncio
async def test_scrape_scan_with_mocked_jobspy():
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

    with patch("repository.scraper.glassdoor_jobspy.jobspy", mock_jobspy):
        input_data = {
            "board": "glassdoor",
            "keyword": "golang",
            "location": "ireland",
            "user_ids": ["admin"],
            "search_id": "s1",
        }
        res = await scrape_scan(input_data)
        assert len(res["jobs"]) == 1
        assert res["jobs"][0]["title"] == "Backend Go"
        assert res["boards"] == ["glassdoor"]
        assert len(res["errors"]) == 0
        assert mock_jobspy.scrape_jobs.call_count == 1


@pytest.mark.asyncio
async def test_scrape_scan_board_error_resilience():
    mock_jobspy = MagicMock()
    mock_jobspy.SUPPORTED_SITES = {"glassdoor", "linkedin"}
    mock_jobspy.scrape_jobs.side_effect = RuntimeError("429 Too Many Requests")

    with patch("repository.scraper.glassdoor_jobspy.jobspy", mock_jobspy):
        input_data = {
            "board": "glassdoor",
            "keyword": "golang",
            "location": "ireland",
            "user_ids": ["admin"],
            "search_id": "s1",
        }
        res = await scrape_scan(input_data)
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
        assert jobs_in[0]["user_ids"] == ["user-123"]
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
            "user_ids": ["user-123"],
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
            "user_ids": ["admin"],
            "search_id": "scan-1",
            "created_job_slugs": ["job-1", "job-2"],
        }
        res = await queue_analysis(input_data)
        assert res["started"] == 1
        assert res["failed"] == ["job-2"]


@pytest.mark.asyncio
async def test_ingest_jobs_omits_slugs_without_description():
    fake_client = MagicMock()

    async def fake_post(path: str, json: Any) -> Any:
        assert path == "/v1/jobs/bulk"
        return [
            {"slug": "job-with-desc", "created": True},
            {"slug": "job-no-desc", "created": True},
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
            "user_ids": ["user-123"],
            "search_id": "scan-1",
            "jobs": [
                {
                    "slug": "job-with-desc",
                    "title": "Job 1",
                    "description": "Valid job description",
                },
                {"slug": "job-no-desc", "title": "Job 2", "description": None},
            ],
        }
        res = await ingest_jobs(input_data)
        assert res["inserted"] == 2
        assert res["created_slugs"] == ["job-with-desc"]


@pytest.mark.asyncio
async def test_queue_analysis_falls_back_to_individual_on_chunk_error():
    fake_client = MagicMock()

    async def fake_post(path: str, json: Any) -> Any:
        assert path == "/v1/jobs/bulk-queue-analyze"
        assert isinstance(json, list)
        if len(json) > 1:
            # Simulate Go API rejecting chunk because one job has no description
            raise RuntimeError(
                "invalid status transition: job job-no-desc has no description to analyze"
            )
        # Individual retries
        slug = json[0]["slug"]
        if slug == "job-ok":
            return {"workflow_ids": ["wf-job-ok"]}
        raise RuntimeError("cannot analyze")

    async def fake_aclose() -> None:
        pass

    fake_client.post.side_effect = fake_post
    fake_client.aclose.side_effect = fake_aclose

    with patch(
        "deliveries.consumer.activities.job_scrape.NewGetojobuAPI",
        return_value=fake_client,
    ):
        input_data = {
            "user_ids": ["admin"],
            "search_id": "scan-1",
            "created_job_slugs": ["job-ok", "job-bad"],
        }
        res = await queue_analysis(input_data)
        assert res["started"] == 1
        assert res["failed"] == ["job-bad"]


@pytest.mark.asyncio
async def test_scrape_scan_single_target_normalized():
    mock_jobspy = MagicMock()
    mock_jobspy.SUPPORTED_SITES = {"glassdoor", "linkedin"}

    class FakeDataFrame:
        def to_dict(self, orient="records"):
            return [
                {
                    "title": "Backend Go Engineer",
                    "company": "Tech Corp",
                    "location": "Jakarta, Indonesia",
                    "date_posted": date(2026, 11, 12),
                    "min_amount": float("nan"),
                    "site": "glassdoor",
                }
            ]

    mock_jobspy.scrape_jobs.return_value = FakeDataFrame()

    with patch("repository.scraper.glassdoor_jobspy.jobspy", mock_jobspy):
        input_data = {
            "board": "glassdoor",
            "keyword": "golang",
            "location": "indonesia",
            "user_ids": ["userA", "userB"],
            "search_id": "glassdoor-golang-indonesia",
        }
        res = await scrape_scan(input_data)
        assert len(res["jobs"]) == 1
        assert res["jobs"][0]["title"] == "Backend Go Engineer"
        assert res["jobs"][0]["date_posted"] == "2026-11-12"
        assert res["jobs"][0]["min_amount"] is None
        assert res["boards"] == ["glassdoor"]
        assert len(res["errors"]) == 0
        # Verify JSON serializability for Temporal data converter
        assert json.dumps(res) is not None
        mock_jobspy.scrape_jobs.assert_called_once_with(
            site_name="glassdoor",
            search_term="golang",
            location="indonesia",
            results_wanted=20,
            hours_old=24,
            description_format="plain",
            fetch_description=True,
        )


def test_enrich_jobs_preserves_user_ids():
    now_iso = datetime.now(UTC).isoformat()
    raw_jobs = [
        {
            "title": "Backend Go Engineer",
            "company": "Tech Corp",
            "location": "Indonesia",
            "date_posted": now_iso,
            "site": "glassdoor",
            "description": "Awesome role",
        }
    ]

    res = enrich_jobs(
        {
            "board": "glassdoor",
            "keyword": "golang",
            "location": "indonesia",
            "user_ids": ["userA", "userB"],
            "raw_jobs": raw_jobs,
        }
    )
    assert len(res["jobs"]) == 1
    job = res["jobs"][0]
    assert job["user_ids"] == ["userA", "userB"]


@pytest.mark.asyncio
async def test_ingest_jobs_passes_user_ids_in_bulk():
    fake_client = MagicMock()

    async def fake_post(path: str, json: Any) -> Any:
        assert path == "/v1/jobs/bulk"
        jobs_in = json["jobs"]
        assert len(jobs_in) == 1
        assert jobs_in[0]["user_ids"] == ["userA", "userB"]
        return [{"slug": jobs_in[0]["slug"], "created": True}]

    async def fake_aclose() -> None:
        pass

    fake_client.post.side_effect = fake_post
    fake_client.aclose.side_effect = fake_aclose

    with patch(
        "deliveries.consumer.activities.job_scrape.NewGetojobuAPI",
        return_value=fake_client,
    ):
        input_data = {
            "user_ids": ["userA", "userB"],
            "jobs": [
                {
                    "slug": "job-multi-user",
                    "title": "Job Multi",
                    "user_ids": ["userA", "userB"],
                },
            ],
        }
        res = await ingest_jobs(input_data)
        assert res["inserted"] == 1
        assert res["created_slugs"] == ["job-multi-user"]


@pytest.mark.asyncio
async def test_scrape_scan_rejects_empty_board():
    with pytest.raises(ValueError, match="board is required"):
        await scrape_scan({"board": "", "keyword": "golang", "location": "indonesia"})
    with pytest.raises(ValueError, match="board is required"):
        await scrape_scan({"keyword": "golang", "location": "indonesia"})


@pytest.mark.asyncio
async def test_job_scrape_workflow_rejects_empty_board():
    wf = JobScrapeWorkflow()
    with pytest.raises(ValueError, match="board is required"):
        await wf.run({"board": "", "keyword": "golang", "location": "indonesia"})
    with pytest.raises(ValueError, match="board is required"):
        await wf.run({"keyword": "golang", "location": "indonesia"})


@pytest.mark.asyncio
async def test_job_scrape_workflow_activities_have_retry_policy():
    wf = JobScrapeWorkflow()
    calls = []

    async def fake_execute_activity(activity, *args, **kwargs):
        calls.append((activity, kwargs))
        if activity == scrape_scan:
            return {"jobs": [], "errors": [], "boards": ["glassdoor"]}
        if activity == enrich_jobs:
            return {"jobs": [], "found": 0, "filtered_out_recency": 0}
        if activity == ingest_jobs:
            return {"inserted": 0, "skipped": 0, "created_slugs": []}
        if activity == queue_analysis:
            return {"started": 0, "failed": []}
        return {}

    with patch(
        "temporalio.workflow.execute_activity", side_effect=fake_execute_activity
    ):
        await wf.run(
            {
                "board": "glassdoor",
                "keyword": "golang",
                "location": "dublin",
                "user_ids": ["u1"],
            }
        )

    assert len(calls) == 4
    for activity, kwargs in calls:
        policy = kwargs.get("retry_policy")
        assert policy is not None, f"Activity {activity} missing retry_policy"
        assert policy.maximum_attempts == 3
        assert policy.initial_interval == timedelta(seconds=2)
        assert policy.backoff_coefficient == 2.0
        assert policy.maximum_interval == timedelta(seconds=30)


@pytest.mark.asyncio
async def test_scrape_scan_with_scraper_service():
    from deliveries.consumer.activities.job_scrape import init_job_scrape_container
    from repository.scraper.base import ScraperRepository
    from usecase.scraper.service import ScraperService

    class MockScraper(ScraperRepository):
        def __init__(self, name: str, vendor: str):
            self.name = name
            self.vendor = vendor

        async def scrape(self, keyword: str, location: str):
            return [
                {
                    "title": "Senior Go Developer",
                    "company": "Acme",
                    "location": "Dubai",
                    "site": self.vendor,
                    "date_posted": "2026-11-14",
                }
            ]

    s_li = MockScraper("linkedin-apify-curious_coder", "linkedin")
    service = ScraperService(
        configured_methods={
            "linkedin": "linkedin-apify-curious_coder",
            "glassdoor": "",
        },
        scrapers={s_li.name: s_li},
    )

    mock_container = MagicMock()
    mock_container.usecase.scraper = service
    init_job_scrape_container(mock_container)

    try:
        # 1. Enabled board: linkedin
        input_data = {
            "board": "linkedin",
            "keyword": "golang",
            "location": "dubai",
            "user_ids": ["admin"],
            "search_id": "s-li-1",
        }
        res = await scrape_scan(input_data)
        assert len(res["jobs"]) == 1
        assert res["jobs"][0]["title"] == "Senior Go Developer"
        assert res["boards"] == ["linkedin"]
        assert len(res["errors"]) == 0

        # 2. Disabled board: glassdoor -> skips and logs
        input_disabled = {
            "board": "glassdoor",
            "keyword": "golang",
            "location": "dubai",
            "user_ids": ["admin"],
            "search_id": "s-gd-1",
        }
        res_disabled = await scrape_scan(input_disabled)
        assert len(res_disabled["jobs"]) == 0
        assert res_disabled["boards"] == []
        assert len(res_disabled["errors"]) == 0

    finally:
        init_job_scrape_container(None)


@pytest.mark.asyncio
async def test_scrape_scan_unregistered_method_error_resilience():
    from deliveries.consumer.activities.job_scrape import init_job_scrape_container
    from usecase.scraper.service import ScraperService

    service = ScraperService(
        configured_methods={"linkedin": "linkedin-nonexistent"},
        scrapers={},
    )
    mock_container = MagicMock()
    mock_container.usecase.scraper = service
    init_job_scrape_container(mock_container)

    try:
        input_data = {
            "board": "linkedin",
            "keyword": "golang",
            "location": "dubai",
            "user_ids": ["admin"],
            "search_id": "s-err",
        }
        res = await scrape_scan(input_data)
        assert len(res["jobs"]) == 0
        assert len(res["errors"]) == 1
        assert res["errors"][0]["board"] == "linkedin"
        assert "not registered" in res["errors"][0]["error"]
    finally:
        init_job_scrape_container(None)
