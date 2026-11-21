"""Unit tests for Apify account failover and token rotation."""

import json
from unittest.mock import MagicMock, patch
from urllib.error import URLError

import pytest

from repository.scraper import apify_accounts


def _limits_response(limit: float, used: float) -> MagicMock:
    payload = json.dumps(
        {
            "data": {
                "limits": {"maxMonthlyUsageUsd": limit},
                "current": {"monthlyUsageUsd": used},
                "monthlyUsageCycle": {"endAt": "2099-12-31T00:00:00.000Z"},
            }
        }
    ).encode()
    response = MagicMock()
    response.__enter__.return_value = response
    response.__exit__.return_value = False
    response.read.return_value = payload
    return response


def test_tokens_parsing():
    with patch.dict("os.environ", {"APIFY_API_TOKENS": "tok1, tok2, tok3"}, clear=True):
        assert apify_accounts._tokens() == ["tok1", "tok2", "tok3"]

    with patch.dict("os.environ", {"APIFY_API_TOKEN": "single_tok"}, clear=True):
        assert apify_accounts._tokens() == ["single_tok"]

    with patch.dict("os.environ", {}, clear=True):
        assert apify_accounts._tokens() == []


def test_pick_token_returns_none_when_no_tokens(tmp_path):
    with patch.dict(
        "os.environ",
        {"APIFY_ACCOUNT_STATE_DIR": str(tmp_path)},
        clear=True,
    ):
        token, label, stats = apify_accounts.pick_token()
        assert token is None
        assert label is None
        assert stats == {}


def test_pick_token_selects_fresh_account_with_credit(tmp_path):
    state_file = tmp_path / "apify_accounts.json"
    state_file.write_text(
        json.dumps(
            {
                "active_index": 0,
                "accounts": {
                    "0": {
                        "credit_remaining_usd": 1.50,
                        "credit_limit_usd": 5.00,
                        "cycle_end": "2099-12-31",
                        "email": "dev1@example.com",
                    }
                },
            }
        )
    )

    env = {
        "APIFY_API_TOKENS": "tok1,tok2",
        "APIFY_MIN_CREDIT_USD": "0.50",
        "APIFY_ACCOUNT_STATE_DIR": str(tmp_path),
    }
    with patch.dict("os.environ", env, clear=True):
        token, label, stats = apify_accounts.pick_token()
        assert token == "tok1"
        assert "dev1@example.com" in (label or "")
        assert stats.get("credit_remaining_usd") == 1.50


def test_pick_token_failover_when_first_account_drained(tmp_path):
    env = {
        "APIFY_API_TOKENS": "tok1,tok2",
        "APIFY_MIN_CREDIT_USD": "0.50",
        "APIFY_ACCOUNT_STATE_DIR": str(tmp_path),
    }

    # tok1 has 0.10, tok2 has 2.00
    def mock_usage(tok):
        if tok == "tok1":
            return {
                "credit_limit_usd": 5.0,
                "credit_used_usd": 4.9,
                "credit_remaining_usd": 0.10,
                "credit_cycle_end": "2099-12-31",
            }
        return {
            "credit_limit_usd": 5.0,
            "credit_used_usd": 3.0,
            "credit_remaining_usd": 2.00,
            "credit_cycle_end": "2099-12-31",
        }

    with (
        patch.dict("os.environ", env, clear=True),
        patch.object(apify_accounts, "_usage", side_effect=mock_usage),
        patch.object(
            apify_accounts,
            "_profile",
            return_value={"email": "acct2@example.com"},
        ),
    ):
        token, _label, stats = apify_accounts.pick_token()
        assert token == "tok2"
        assert stats.get("credit_remaining_usd") == 2.00


def test_pick_token_returns_none_when_all_accounts_underfunded(tmp_path):
    env = {
        "APIFY_API_TOKENS": "tok1,tok2",
        "APIFY_MIN_CREDIT_USD": "0.50",
        "APIFY_ACCOUNT_STATE_DIR": str(tmp_path),
    }

    def mock_usage(_tok):
        return {
            "credit_limit_usd": 5.0,
            "credit_used_usd": 4.95,
            "credit_remaining_usd": 0.05,
            "credit_cycle_end": "2099-12-31",
        }

    with (
        patch.dict("os.environ", env, clear=True),
        patch.object(apify_accounts, "_usage", side_effect=mock_usage),
    ):
        token, label, stats = apify_accounts.pick_token()
        assert token is None
        assert label is None
        assert stats == {}


def test_usage_retries_transient_failure():
    attempts = 0

    def fake_urlopen(_req, timeout):
        nonlocal attempts
        attempts += 1
        if attempts == 1:
            raise URLError("temporary network error")
        return _limits_response(limit=5.0, used=1.0)

    with (
        patch("urllib.request.urlopen", side_effect=fake_urlopen),
        patch("time.sleep"),
    ):
        result = apify_accounts._usage("tok1")

    assert result == {
        "credit_limit_usd": 5.0,
        "credit_used_usd": 1.0,
        "credit_remaining_usd": 4.0,
        "credit_cycle_end": "2099-12-31",
    }
    assert attempts == 2


def test_usage_gives_up_after_three_attempts():
    attempts = 0

    def fake_urlopen(_req, timeout):
        nonlocal attempts
        attempts += 1
        raise URLError("apify down")

    with (
        patch("urllib.request.urlopen", side_effect=fake_urlopen),
        patch("time.sleep"),
    ):
        result = apify_accounts._usage("tok1")

    assert result == {}
    assert attempts == 3


def test_pick_token_raises_when_all_balance_lookups_fail(tmp_path):
    env = {
        "APIFY_API_TOKENS": "tok1,tok2",
        "APIFY_MIN_CREDIT_USD": "0.50",
        "APIFY_ACCOUNT_STATE_DIR": str(tmp_path),
    }
    with (
        patch.dict("os.environ", env, clear=True),
        patch.object(apify_accounts, "_usage", return_value={}),
    ):
        with pytest.raises(RuntimeError, match="could not check Apify balance"):
            apify_accounts.pick_token()


def test_record_balance_updates_state(tmp_path):
    env = {
        "APIFY_API_TOKENS": "tok1",
        "APIFY_ACCOUNT_STATE_DIR": str(tmp_path),
    }
    with patch.dict("os.environ", env, clear=True):
        apify_accounts.record_balance(
            "tok1",
            {
                "credit_remaining_usd": 0.85,
                "credit_limit_usd": 5.00,
                "credit_cycle_end": "2099-12-31",
                "email": "test@example.com",
            },
        )
        state_file = tmp_path / "apify_accounts.json"
        assert state_file.exists()
        data = json.loads(state_file.read_text())
        assert data["accounts"]["0"]["credit_remaining_usd"] == 0.85
