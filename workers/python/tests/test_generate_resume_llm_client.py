"""Resume LLM client must fail clearly when the provider returns no JSON object."""

from types import SimpleNamespace
from unittest.mock import AsyncMock

import pytest

from usecase.generate_resume.llm_client import OpenAIResumeClient
from utils.llm_errors import LLMError

PROMPT_VERSION = "critique-resume-v1"


def _client_with_response(content: str | None, finish_reason: object = "stop"):
    client = OpenAIResumeClient(
        api_key="test-key", base_url="http://mock", model="test-model"
    )
    response = SimpleNamespace(
        choices=[
            SimpleNamespace(
                message=SimpleNamespace(content=content),
                finish_reason=finish_reason,
            )
        ]
    )
    client._client.chat.completions.create = AsyncMock(  # type: ignore[method-assign]
        return_value=response
    )
    return client


async def _critique(client: OpenAIResumeClient) -> None:
    await client.critique_resume(
        prompt_version=PROMPT_VERSION,
        resume_draft={"headline": "Engineer"},
        candidate_profile="{}",
        job_profile="{}",
        match_evidence="{}",
        facts="{}",
    )


@pytest.mark.asyncio
async def test_empty_response_is_reported_as_empty() -> None:
    with pytest.raises(LLMError) as exc_info:
        await _critique(_client_with_response(""))

    assert exc_info.value.detail == "resume_critique: empty LLM response"


@pytest.mark.asyncio
async def test_whitespace_only_response_is_reported_as_empty() -> None:
    with pytest.raises(LLMError) as exc_info:
        await _critique(_client_with_response("   \n\t"))

    assert exc_info.value.detail == "resume_critique: empty LLM response"


@pytest.mark.asyncio
async def test_plain_text_response_reports_non_json_diagnostics() -> None:
    with pytest.raises(LLMError) as exc_info:
        await _critique(_client_with_response("I cannot audit this resume."))

    detail = exc_info.value.detail
    assert "resume_critique: non-JSON LLM response" in detail
    assert "finish_reason=stop" in detail
    assert "has_object=False" in detail
    assert "fenced=False" in detail


@pytest.mark.asyncio
async def test_truncated_response_is_reported_as_truncation() -> None:
    with pytest.raises(LLMError) as exc_info:
        await _critique(_client_with_response('{"passed": true', "length"))

    assert (
        exc_info.value.detail
        == "resume_critique: LLM response truncated before JSON completed"
    )


@pytest.mark.asyncio
async def test_fenced_json_response_still_parses() -> None:
    client = _client_with_response('```json\n{"passed": true, "issues": []}\n```')

    result = await client.critique_resume(
        prompt_version=PROMPT_VERSION,
        resume_draft={"headline": "Engineer"},
        candidate_profile="{}",
        job_profile="{}",
        match_evidence="{}",
        facts="{}",
    )

    assert result == {"passed": True, "issues": []}
