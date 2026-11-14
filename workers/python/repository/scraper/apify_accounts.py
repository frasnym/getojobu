"""Apify account failover and token rotation.

Picks a token that has >= min_credit_usd left, preferring the account that was
used last. Reads/writes a small JSON state file so a subsequent run needs zero
API calls when the last balance is still valid.

State file: $STATE_DIR/apify_accounts.json (default /var/lib/job-scrape/),
overridable via APIFY_ACCOUNT_STATE_DIR for tests and runtime configuration.
"""

from __future__ import annotations

import json
import os
import urllib.error
import urllib.parse
import urllib.request
from datetime import UTC, datetime
from typing import Any, cast

from utils.logging import get_logger

logger = get_logger("scraper.apify_accounts")

APIFY_BASE = "https://api.apify.com/v2"
DEFAULT_MIN_CREDIT = 0.50
STATE_FILENAME = "apify_accounts.json"


def _state_dir() -> str:
    return os.environ.get("APIFY_ACCOUNT_STATE_DIR", "/var/lib/job-scrape")


def _state_path() -> str:
    return os.path.join(_state_dir(), STATE_FILENAME)


def _tokens() -> list[str]:
    """All tokens, comma-separated in APIFY_API_TOKENS, or single token."""
    raw = (
        os.environ.get("APIFY_API_TOKENS") or os.environ.get("APIFY_API_TOKEN") or ""
    ).strip()
    if not raw:
        return []
    return [t.strip() for t in raw.split(",") if t.strip()]


def _load_state() -> dict[str, Any]:
    path = _state_path()
    try:
        with open(path) as fh:
            return json.load(fh)
    except (FileNotFoundError, json.JSONDecodeError, OSError):
        return {}


def _save_state(state: dict[str, Any]) -> None:
    path = _state_path()
    try:
        os.makedirs(os.path.dirname(path), exist_ok=True)
        tmp = f"{path}.tmp"
        with open(tmp, "w") as fh:
            json.dump(state, fh, indent=2)
        os.replace(tmp, path)
    except OSError as e:
        logger.warning("state save failed", error=str(e))


def _profile(token: str) -> dict[str, Any]:
    """Apify /users/me — returns {email, username, name}."""
    try:
        url = f"{APIFY_BASE}/users/me?token={urllib.parse.quote(token)}"
        req = urllib.request.Request(url)
        with urllib.request.urlopen(req, timeout=20) as resp:
            data = json.loads(resp.read().decode()).get("data", {})
        profile = data.get("profile", {})
        return {
            "email": data.get("email"),
            "username": data.get("username"),
            "name": profile.get("name"),
        }
    except (
        urllib.error.URLError,
        OSError,
        json.JSONDecodeError,
        KeyError,
        ValueError,
        TypeError,
    ) as e:
        logger.warning("profile lookup failed", error=str(e))
        return {}


def _usage(token: str) -> dict[str, Any]:
    """Apify /users/me/limits."""
    try:
        url = f"{APIFY_BASE}/users/me/limits?token={urllib.parse.quote(token)}"
        req = urllib.request.Request(url)
        with urllib.request.urlopen(req, timeout=20) as resp:
            data = json.loads(resp.read().decode()).get("data", {})
        limit = float(data.get("limits", {}).get("maxMonthlyUsageUsd") or 0)
        used = float(data.get("current", {}).get("monthlyUsageUsd") or 0)
        end = (data.get("monthlyUsageCycle", {}).get("endAt") or "")[:10]
        return {
            "credit_limit_usd": round(limit, 2),
            "credit_used_usd": round(used, 2),
            "credit_remaining_usd": round(max(limit - used, 0), 2),
            "credit_cycle_end": end,
        }
    except (
        urllib.error.URLError,
        OSError,
        json.JSONDecodeError,
        KeyError,
        ValueError,
        TypeError,
    ) as e:
        logger.warning("usage lookup failed", error=str(e))
        return {}


def get_usage(token: str) -> dict[str, Any]:
    """Retrieve Apify usage and remaining credit limits for a token."""
    return _usage(token)


def _is_fresh(entry: dict[str, Any]) -> bool:
    """Balance is valid until the monthly cycle ends."""
    cycle_end = entry.get("cycle_end", "")
    if not cycle_end:
        return False
    today = datetime.now(UTC).strftime("%Y-%m-%d")
    return today <= cycle_end


def _label_for_index(idx: int, tokens: list[str], email: str | None = None) -> str:
    """Human label for an account. Prefers the Apify email, else the position."""
    if len(tokens) == 1:
        return email or "sole account"
    pos = f"({idx + 1}/{len(tokens)})"
    return f"{email} {pos}" if email else f"account {idx + 1}/{len(tokens)}"


def _ensure_email(state: dict[str, Any], idx: int, tokens: list[str]) -> str | None:
    """Cached email for an account, fetching /users/me once when absent."""
    entry = state.setdefault("accounts", {}).get(str(idx), {})
    email = entry.get("email")
    if email:
        return email
    profile = _profile(tokens[idx])
    email = profile.get("email")
    if email:
        entry["email"] = email
        if profile.get("username"):
            entry["username"] = profile["username"]
        state["accounts"][str(idx)] = entry
        _save_state(state)
    return email


def pick_token(
    config: dict[str, Any] | None = None,
) -> tuple[str | None, str | None, dict[str, Any]]:
    """Returns (token, label, stats).

    token/label are None when no account has enough credit.
    stats always reflects the account that *will* run (or the last-checked).
    """
    min_credit_val: float | int | str | None = None
    if config:
        apify_raw = config.get("apify")
        if isinstance(apify_raw, dict):
            apify_cfg: dict[str, Any] = cast(dict[str, Any], apify_raw)
            val = apify_cfg.get("min_credit_usd")
            if isinstance(val, (int, float, str)):
                min_credit_val = val

    env_val = os.environ.get("APIFY_MIN_CREDIT_USD")
    credit_source = env_val if env_val is not None else min_credit_val
    try:
        min_credit = (
            float(credit_source)
            if credit_source is not None
            else DEFAULT_MIN_CREDIT
        )
    except (TypeError, ValueError):
        min_credit = DEFAULT_MIN_CREDIT

    tokens = _tokens()
    if not tokens:
        logger.error("no APIFY_API_TOKEN(S) set")
        return None, None, {}

    state = _load_state()
    active_idx = state.get("active_index", 0)
    if active_idx >= len(tokens):
        active_idx = 0

    # Step 1: try the stored active account if it looks fresh & funded
    acct_key = str(active_idx)
    active_entry = state.get("accounts", {}).get(acct_key, {})
    if (
        _is_fresh(active_entry)
        and active_entry.get("credit_remaining_usd", 0) >= min_credit
    ):
        token = tokens[active_idx]
        label = _label_for_index(
            active_idx, tokens, _ensure_email(state, active_idx, tokens)
        )
        stats = {
            "credit_remaining_usd": active_entry.get("credit_remaining_usd"),
            "credit_limit_usd": active_entry.get("credit_limit_usd"),
            "credit_cycle_end": active_entry.get("cycle_end"),
        }
        return token, label, stats

    # Step 2: probe from active onward, wrap to start
    order = list(range(active_idx, len(tokens))) + list(range(active_idx))
    for idx in order:
        acct_key = str(idx)
        entry = state.get("accounts", {}).get(acct_key, {})

        if not _is_fresh(entry):
            pass
        elif entry.get("credit_remaining_usd", 0) >= min_credit:
            token = tokens[idx]
            label = _label_for_index(idx, tokens, entry.get("email"))
            stats = {
                "credit_remaining_usd": entry.get("credit_remaining_usd"),
                "credit_limit_usd": entry.get("credit_limit_usd"),
                "credit_cycle_end": entry.get("cycle_end"),
            }
            if idx != active_idx:
                state["active_index"] = idx
                state["accounts"][str(idx)] = entry
                _save_state(state)
            return token, label, stats

        # probe for real
        usage = _usage(tokens[idx])
        remaining = usage.get("credit_remaining_usd")
        cycle_end = usage.get("credit_cycle_end", "")

        state.setdefault("accounts", {})
        state["accounts"][acct_key] = {
            "credit_remaining_usd": remaining,
            "credit_limit_usd": usage.get("credit_limit_usd"),
            "cycle_end": cycle_end,
        }

        if remaining is not None and remaining >= min_credit:
            token = tokens[idx]
            state["active_index"] = idx
            label = _label_for_index(idx, tokens, _ensure_email(state, idx, tokens))
            _save_state(state)
            return token, label, usage

    # If no account has >= min_credit, do not fall back. Fail fast per spec.
    _save_state(state)
    return None, None, {}


def record_balance(token_or_idx: int | str, usage: dict[str, Any]) -> None:
    """Write back a freshly-measured balance (after-run update)."""
    tokens = _tokens()
    if isinstance(token_or_idx, int) and 0 <= token_or_idx < len(tokens):
        idx = token_or_idx
    else:
        for i, t in enumerate(tokens):
            if t == token_or_idx:
                idx = i
                break
        else:
            return
    state = _load_state()
    state.setdefault("accounts", {})
    prev = state["accounts"].get(str(idx), {})
    state["accounts"][str(idx)] = {
        "credit_remaining_usd": usage.get("credit_remaining_usd"),
        "credit_limit_usd": usage.get("credit_limit_usd"),
        "cycle_end": usage.get("credit_cycle_end", ""),
        "email": usage.get("email") or prev.get("email"),
        "username": prev.get("username"),
    }
    state["active_index"] = idx
    _save_state(state)
