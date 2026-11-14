"""Unit tests for concrete scraper implementations and parsers."""

from datetime import UTC, datetime
from unittest.mock import MagicMock, patch

import pytest

from repository.scraper.glassdoor_apify_valig import (
    GlassdoorApifyValigScraper,
    strip_html,
)
from repository.scraper.glassdoor_jobspy import (
    GlassdoorJobspyScraper,
)
from repository.scraper.linkedin_apify_curious_coder import (
    LinkedinApifyCuriousCoderScraper,
)


def test_strip_html():
    assert strip_html("<p>Hello <b>World</b></p>") == "Hello World"
    assert strip_html(None) == ""
    assert strip_html("   Plain text   ") == "Plain text"


def test_linkedin_apify_curious_coder_input_and_parse():
    scraper = LinkedinApifyCuriousCoderScraper()
    assert scraper.name == "linkedin-apify-curious_coder"
    assert scraper.vendor == "linkedin"

    run_input = scraper.build_run_input("backend engineer", "dubai")
    assert "urls" in run_input
    assert len(run_input["urls"]) == 1
    assert "f_TPR=r86400" in run_input["urls"][0]
    assert "keywords=backend+engineer" in run_input["urls"][0]

    raw_items = [
        {
            "id": "12345",
            "title": "Backend Engineer",
            "companyName": "Tech Corp",
            "location": "Dubai, UAE",
            "postedAt": "2026-11-14T08:00:00.000Z",
            "descriptionText": "Great Go role",
            "link": "https://www.linkedin.com/jobs/view/12345",
            "isRemote": False,
        }
    ]
    parsed = scraper.parse_items(raw_items)
    assert len(parsed) == 1
    p = parsed[0]
    assert p["title"] == "Backend Engineer"
    assert p["company"] == "Tech Corp"
    assert p["location"] == "Dubai, UAE"
    assert p["posted_date"] == "2026-11-14"
    assert p["description"] == "Great Go role"
    assert p["source_url"] == "https://www.linkedin.com/jobs/view/12345"
    assert p["site"] == "linkedin"


def test_glassdoor_apify_valig_input_and_parse():
    scraper = GlassdoorApifyValigScraper()
    assert scraper.name == "glassdoor-apify-valig"
    assert scraper.vendor == "glassdoor"

    run_input = scraper.build_run_input("golang", "berlin")
    assert run_input["keywords"] == "golang"
    assert run_input["location"] == "berlin"
    assert run_input["daysOld"] == 7

    raw_items = [
        {
            "id": 999,
            "title": "Senior Go Dev",
            "employer": {"name": "Gmbh Corp"},
            "location": {"name": "Berlin, Germany"},
            "ageInDays": 1,
            "description": "<p>Build <strong>systems</strong>.</p>",
            "url": "https://glassdoor.com/job/999",
        }
    ]
    parsed = scraper.parse_items(raw_items)
    assert len(parsed) == 1
    p = parsed[0]
    assert p["title"] == "Senior Go Dev"
    assert p["company"] == "Gmbh Corp"
    assert p["location"] == "Berlin, Germany"
    assert p["description"] == "Build systems."
    assert p["source_url"] == "https://glassdoor.com/job/999"
    assert p["site"] == "glassdoor"
    assert p["posted_date"] != ""


@pytest.mark.asyncio
async def test_glassdoor_jobspy_scrape():
    scraper = GlassdoorJobspyScraper()
    assert scraper.name == "glassdoor-jobspy"
    assert scraper.vendor == "glassdoor"

    mock_jobspy = MagicMock()
    mock_jobspy.scrape_jobs.return_value = [
        {
            "title": "Platform Engineer",
            "company": "Cloud Inc",
            "location": "Remote",
            "date_posted": datetime.now(UTC).isoformat(),
            "site": "glassdoor",
        }
    ]

    with patch("repository.scraper.glassdoor_jobspy.jobspy", mock_jobspy):
        jobs = await scraper.scrape("platform", "remote")
        assert len(jobs) == 1
        assert jobs[0]["title"] == "Platform Engineer"
        assert jobs[0]["site"] == "glassdoor"
        assert mock_jobspy.scrape_jobs.call_count == 1
