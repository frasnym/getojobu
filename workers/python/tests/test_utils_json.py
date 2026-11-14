"""Unit tests for robust LLM JSON parsing utility."""

import json

import pytest

from utils.json import parse_llm_json


def test_parse_llm_json_clean_object():
    content = '{"key": "value", "num": 123}'
    res = parse_llm_json(content)
    assert res == {"key": "value", "num": 123}


def test_parse_llm_json_clean_array():
    content = '[1, 2, "three"]'
    res = parse_llm_json(content)
    assert res == [1, 2, "three"]


def test_parse_llm_json_markdown_code_fence():
    content = """```json
{
  "name": "test",
  "score": 95.5
}
```"""
    res = parse_llm_json(content)
    assert res == {"name": "test", "score": 95.5}


def test_parse_llm_json_trailing_extra_data():
    content = '{"valid": true, "nested": {"a": [1, 2]}}\n\nExtra commentary from model at end of generation'
    res = parse_llm_json(content)
    assert res == {"valid": True, "nested": {"a": [1, 2]}}


def test_parse_llm_json_leading_and_trailing_text():
    content = 'Here is the requested output:\n```json\n{"status": "ok"}\n```\nHope this helps!'
    res = parse_llm_json(content)
    assert res == {"status": "ok"}


def test_parse_llm_json_invalid_raises():
    with pytest.raises(json.JSONDecodeError):
        parse_llm_json("not valid json at all")
