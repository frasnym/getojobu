"""Unit tests for the Step 2 LLM client."""

import json
from unittest.mock import AsyncMock, MagicMock

import pytest
from usecase.evaluate_match.llm_client import OpenAIClient
from utils.llm_errors import LLMError


@pytest.fixture
def mock_openai_client() -> OpenAIClient:
    client = OpenAIClient(
        api_key="test-key",
        base_url="http://mock-omniroute:20128",
        model="test-model",
    )
    return client


@pytest.mark.asyncio
async def test_extract_match_evaluation_renders_and_parses(
    mock_openai_client: OpenAIClient,
) -> None:
    expected_response = {
        "evaluations": [
            {
                "category": "technical_skills",
                "score": 90,
                "reason": "Strong Go skills",
                "strengths": ["Golang"],
                "gaps": [],
                "evidence": [
                    {
                        "candidate_evidence": "6 years Go",
                        "job_requirement": "Golang",
                        "relationship": "direct",
                    }
                ],
            }
        ],
        "mandatory_requirements": [
            {
                "requirement": "Golang",
                "status": "met",
                "reason": "Present in CV",
                "evidence": ["CV experience"],
            }
        ],
    }

    mock_chat_completion = MagicMock()
    mock_choice = MagicMock()
    mock_choice.message.content = json.dumps(expected_response)
    mock_chat_completion.choices = [mock_choice]

    mock_openai_client._client.chat.completions.create = AsyncMock(  # type: ignore[method-assign]
        return_value=mock_chat_completion
    )

    result = await mock_openai_client.extract_match_evaluation(
        candidate_profile={"skills": ["golang"]},
        job_profile={"title": "Backend Engineer"},
        evaluation_config={"family": "technology"},
    )

    assert result == expected_response
    mock_openai_client._client.chat.completions.create.assert_awaited_once()  # type: ignore[attr-defined]


@pytest.mark.asyncio
async def test_extract_match_evaluation_injects_category_enum(
    mock_openai_client: OpenAIClient,
) -> None:
    expected_response = {
        "evaluations": [],
        "mandatory_requirements": [],
    }

    mock_chat_completion = MagicMock()
    mock_choice = MagicMock()
    mock_choice.message.content = json.dumps(expected_response)
    mock_chat_completion.choices = [mock_choice]

    mock_create = AsyncMock(return_value=mock_chat_completion)
    mock_openai_client._client.chat.completions.create = mock_create  # type: ignore[method-assign]

    eval_config = {
        "family": "technology",
        "categories": [
            {"name": "technical_skills", "weight": 0.5},
            {"name": "responsibilities", "weight": 0.5},
        ],
    }

    await mock_openai_client.extract_match_evaluation(
        candidate_profile={"skills": ["golang"]},
        job_profile={"title": "Backend Engineer"},
        evaluation_config=eval_config,
    )

    mock_create.assert_awaited_once()
    kwargs = mock_create.call_args.kwargs
    # Check that category enum was injected
    schema = kwargs["response_format"]["json_schema"]["schema"]
    cat_enum = schema["$defs"]["CategoryEvaluation"]["properties"]["category"]["enum"]
    assert cat_enum == ["technical_skills", "responsibilities"]

    # Check user prompt explicitly mentions categories
    messages = kwargs["messages"]
    user_msg = next(m["content"] for m in messages if m["role"] == "user")
    assert "technical_skills" in user_msg
    assert "responsibilities" in user_msg


@pytest.mark.asyncio
async def test_extract_match_evaluation_raises_on_truncated_response(
    mock_openai_client: OpenAIClient,
) -> None:
    """A truncated LLM response (finish_reason='length') must fail clearly."""
    mock_chat_completion = MagicMock()
    mock_choice = MagicMock()
    mock_choice.finish_reason = "length"
    mock_choice.message.content = '{"evaluations":[{"category":"technical_sk'
    mock_chat_completion.choices = [mock_choice]

    mock_openai_client._client.chat.completions.create = AsyncMock(  # type: ignore[method-assign]
        return_value=mock_chat_completion
    )

    with pytest.raises(RuntimeError, match="truncated"):
        await mock_openai_client.extract_match_evaluation(
            candidate_profile={"skills": ["golang"]},
            job_profile={"title": "Backend Engineer"},
            evaluation_config={"family": "technology"},
        )


@pytest.mark.asyncio
async def test_extract_match_evaluation_rejects_json_array(
    mock_openai_client: OpenAIClient,
) -> None:
    """A top-level JSON array is not a valid match evaluation object."""
    mock_chat_completion = MagicMock()
    mock_choice = MagicMock()
    mock_choice.finish_reason = "stop"
    mock_choice.message.content = "[]"
    mock_chat_completion.choices = [mock_choice]

    mock_openai_client._client.chat.completions.create = AsyncMock(  # type: ignore[method-assign]
        return_value=mock_chat_completion
    )

    with pytest.raises(LLMError) as exc_info:
        await mock_openai_client.extract_match_evaluation(
            candidate_profile={"skills": ["golang"]},
            job_profile={"title": "Backend Engineer"},
            evaluation_config={"family": "technology"},
        )

    assert exc_info.value.message == "LLM error during match evaluation"
    assert "expected JSON object" in exc_info.value.detail
    assert "list" in exc_info.value.detail


@pytest.mark.asyncio
async def test_extract_match_evaluation_wraps_provider_error(
    mock_openai_client: OpenAIClient,
) -> None:
    """A provider/SDK failure becomes a clear LLM error with a detail message."""
    mock_openai_client._client.chat.completions.create = AsyncMock(  # type: ignore[method-assign]
        side_effect=Exception("Error code: 499 - Request aborted")
    )

    with pytest.raises(LLMError) as exc_info:
        await mock_openai_client.extract_match_evaluation(
            candidate_profile={"skills": ["golang"]},
            job_profile={"title": "Backend Engineer"},
            evaluation_config={"family": "technology"},
        )

    error = exc_info.value
    assert error.message == "LLM error during match evaluation"
    assert error.detail == "Error code: 499 - Request aborted"


def test_render_prompt_missing_placeholder(tmp_path: pytest.TempPathFactory) -> None:
    client = OpenAIClient(
        api_key="test-key",
        base_url="http://mock",
        model="test-model",
    )
    with pytest.raises(FileNotFoundError):
        client._render_prompt(
            "non-existent-prompt",
            candidate_profile={},
            job_profile={},
            evaluation_config={},
        )
