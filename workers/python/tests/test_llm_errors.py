"""Tests for the shared, two-part LLM error message."""

from utils.llm_errors import LLMError


def test_llm_error_exposes_general_message_and_detail() -> None:
    error = LLMError(
        "LLM error during match evaluation",
        detail="Error code: 499 - Request aborted",
    )

    assert error.message == "LLM error during match evaluation"
    assert error.detail == "Error code: 499 - Request aborted"
    assert str(error) == (
        "LLM error during match evaluation: Error code: 499 - Request aborted"
    )
