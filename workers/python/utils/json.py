"""Robust JSON parsing utilities for LLM output."""

from __future__ import annotations

import json
import re
from typing import cast


def parse_llm_json(content: str) -> dict[str, object]:
    """Parse a JSON object from LLM output safely.

    Handles common LLM artifacts:
    - Markdown code fences (```json ... ```)
    - Leading text before the first JSON object
    - Trailing text, commentary, or fences after the JSON object

    Raises ``TypeError`` when the response is not a JSON object (for example a
    bare JSON array), so callers never have to guard against a list.
    """
    text = content.strip()
    if text.startswith("```"):
        text = re.sub(r"^```(?:json)?\s*", "", text, flags=re.IGNORECASE)
        text = re.sub(r"\s*```$", "", text)
        text = text.strip()

    try:
        parsed = cast(object, json.loads(text))
    except json.JSONDecodeError:
        start_obj = text.find("{")
        if start_obj == -1:
            raise
        parsed, _ = cast(
            tuple[object, int], json.JSONDecoder().raw_decode(text[start_obj:])
        )

    if not isinstance(parsed, dict):
        raise TypeError(
            f"parse_llm_json: expected JSON object, got {type(parsed).__name__}"
        )
    return cast(dict[str, object], parsed)
