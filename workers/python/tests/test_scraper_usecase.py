"""Unit tests for ScraperService usecase."""

from unittest.mock import AsyncMock

import pytest

from repository.scraper.base import ScraperRepository
from usecase.scraper.service import (
    ScraperMethodNotConfiguredError,
    ScraperService,
)


class DummyScraper(ScraperRepository):
    def __init__(self, name: str, vendor: str):
        self.name = name
        self.vendor = vendor
        self.mock_scrape = AsyncMock(return_value=[{"title": "Dev", "site": vendor}])

    async def scrape(self, keyword: str, location: str):
        return await self.mock_scrape(keyword, location)


def test_resolve_configured_method():
    s_li = DummyScraper("linkedin-apify-curious_coder", "linkedin")
    s_gd = DummyScraper("glassdoor-apify-valig", "glassdoor")

    service = ScraperService(
        configured_methods={
            "linkedin": "linkedin-apify-curious_coder",
            "glassdoor": "glassdoor-apify-valig",
        },
        scrapers={
            s_li.name: s_li,
            s_gd.name: s_gd,
        },
    )

    resolved_li = service.resolve("linkedin")
    assert resolved_li == s_li

    resolved_gd = service.resolve("glassdoor")
    assert resolved_gd == s_gd


def test_resolve_disabled_vendor_returns_none():
    s_gd = DummyScraper("glassdoor-apify-valig", "glassdoor")
    service = ScraperService(
        configured_methods={
            "linkedin": "",
            "glassdoor": "glassdoor-apify-valig",
        },
        scrapers={s_gd.name: s_gd},
    )

    assert service.resolve("linkedin") is None
    assert service.resolve("unconfigured_vendor") is None


def test_resolve_unregistered_method_raises():
    service = ScraperService(
        configured_methods={"linkedin": "linkedin-nonexistent"},
        scrapers={},
    )
    with pytest.raises(
        ScraperMethodNotConfiguredError,
        match="method 'linkedin-nonexistent' not registered for vendor 'linkedin'",
    ):
        service.resolve("linkedin")


def test_resolve_vendor_mismatch_raises():
    s_li = DummyScraper("linkedin-apify-curious_coder", "linkedin")
    # Wrong: configured a linkedin method for glassdoor vendor
    service = ScraperService(
        configured_methods={"glassdoor": "linkedin-apify-curious_coder"},
        scrapers={s_li.name: s_li},
    )
    with pytest.raises(
        ScraperMethodNotConfiguredError,
        match="does not match",
    ):
        service.resolve("glassdoor")


@pytest.mark.asyncio
async def test_scrape_serialization_and_skip():
    s_li = DummyScraper("linkedin-apify-curious_coder", "linkedin")
    service = ScraperService(
        configured_methods={
            "linkedin": "linkedin-apify-curious_coder",
            "glassdoor": "",
        },
        scrapers={s_li.name: s_li},
    )

    # Scrape disabled vendor returns []
    res_gd = await service.scrape("glassdoor", "golang", "remote")
    assert res_gd == []
    assert s_li.mock_scrape.call_count == 0

    # Scrape enabled vendor calls scraper
    res_li = await service.scrape("linkedin", "golang", "remote")
    assert len(res_li) == 1
    assert s_li.mock_scrape.call_count == 1
