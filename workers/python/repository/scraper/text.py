"""Shared text helpers for scraper implementations."""

from __future__ import annotations

import re

_HTML_RE = re.compile(r"<[^>]+>")
_WS_RE = re.compile(r"\s+")


def strip_html(text: str | None) -> str:
    """Remove HTML tags, collapse whitespace, and clean up punctuation spacing."""
    if not text:
        return ""
    text = _HTML_RE.sub(" ", text)
    text = re.sub(r"\s+([.,!?:;])", r"\1", text)
    text = _WS_RE.sub(" ", text).strip()
    return text
