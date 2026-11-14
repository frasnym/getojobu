"""Robust JSON parsing utilities for LLM output."""

from __future__ import annotations

import json
import re
from typing import Any


def parse_llm_json(content: str) -> Any:
    """Parse JSON output from an LLM safely.

    Handles common LLM artifacts:
    - Markdown code fences (```json ... ```)
    - Leading text before the first JSON object/array
    - Trailing text, commentary, or fences after the JSON object/array
    """
    text = content.strip()
    if text.startswith("```"):
        text = re.sub(r"^```(?:json)?\s*", "", text, flags=re.IGNORECASE)
        text = re.sub(r"\s*```$", "", text)
        text = text.strip()

    try:
        return json.loads(text)
    except json.JSONDecodeError:
        pass

    # Extract JSON object { ... } using raw_decode
    start_obj = text.find("{")
    if start_obj != -1:
        try:
            obj, _ = json.JSONDecoder().raw_decode(text[start_obj:])
            return obj
        except json.JSONDecodeError:
            pass

    # Extract JSON array [ ... ] using raw_decode
    start_arr = text.find("[")
    if start_arr != -1:
        try:
            arr, _ = json.JSONDecoder().raw_decode(text[start_arr:])
            return arr
        except json.JSONDecodeError:
            pass

    # Fall back to standard json.loads to raise original/clear error
    return json.loads(text)
