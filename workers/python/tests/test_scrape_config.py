"""Tests for ScrapeConfig in config/config.py."""

from unittest.mock import patch

import pytest

from config.config import MissingEnvError, ScrapeConfig, load

BASE_ENV = {
    "TEMPORAL_NAMESPACE": "getojobu",
    "API_BASE_URL": "http://localhost:8080",
    "API_KEY": "test-api-key",
    "OMNIROUTE_BASE_URL": "http://localhost:20128",
    "OMNIROUTE_API_KEY": "test-omni-key",
}


def test_scrape_config_with_valid_methods():
    env = {
        **BASE_ENV,
        "SCRAPER_METHODS_BY_BOARD_LINKEDIN": "linkedin-apify-curious_coder",
        "SCRAPER_METHODS_BY_BOARD_GLASSDOOR": "glassdoor-apify-valig",
    }
    with patch.dict("os.environ", env, clear=True):
        cfg = load()
        assert isinstance(cfg.scrape, ScrapeConfig)
        assert cfg.scrape.get_method("linkedin") == "linkedin-apify-curious_coder"
        assert cfg.scrape.get_method("glassdoor") == "glassdoor-apify-valig"
        assert cfg.scrape.is_enabled("linkedin") is True
        assert cfg.scrape.is_enabled("glassdoor") is True
        assert cfg.scrape.is_enabled("indeed") is False
        assert cfg.scrape.get_method("indeed") is None


def test_scrape_config_with_one_disabled_vendor():
    env = {
        **BASE_ENV,
        "SCRAPER_METHODS_BY_BOARD_LINKEDIN": "",
        "SCRAPER_METHODS_BY_BOARD_GLASSDOOR": "glassdoor-jobspy",
    }
    with patch.dict("os.environ", env, clear=True):
        cfg = load()
        assert cfg.scrape.get_method("linkedin") is None
        assert cfg.scrape.is_enabled("linkedin") is False
        assert cfg.scrape.get_method("glassdoor") == "glassdoor-jobspy"
        assert cfg.scrape.is_enabled("glassdoor") is True


def test_scrape_config_fail_fast_when_all_disabled_or_missing():
    # Both unset
    with (
        patch.dict("os.environ", BASE_ENV, clear=True),
        pytest.raises(MissingEnvError, match="no scraper method enabled"),
    ):
        load()

    # Both explicitly empty
    env_empty = {
        **BASE_ENV,
        "SCRAPER_METHODS_BY_BOARD_LINKEDIN": "",
        "SCRAPER_METHODS_BY_BOARD_GLASSDOOR": "",
    }
    with (
        patch.dict("os.environ", env_empty, clear=True),
        pytest.raises(MissingEnvError, match="no scraper method enabled"),
    ):
        load()
