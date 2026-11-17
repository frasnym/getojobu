"""Unit tests for the GenerateCoverLetter usecase."""

from unittest.mock import AsyncMock

import pytest

from usecase.generate_cover_letter.service import GenerateCoverLetter


def _content() -> dict:
    return {
        "candidate_profile": {"skills": ["go", "postgres"]},
        "job_profile": {"title": "Backend Engineer"},
        "match_evidence": {"strengths": ["distributed systems"]},
        "facts": {"name": "Alex"},
        "overall_score": 88,
        "eligibility_status": "eligible",
        "recommendation_label": "worth_applying",
    }


@pytest.mark.asyncio
async def test_variant_selects_prompt_version() -> None:
    llm = AsyncMock()
    llm.generate_cover_letter.return_value = "Dear Hiring Manager, ..."
    service = GenerateCoverLetter(llm=llm, model="use-combo")

    await service.generate(variant="short", **_content())
    assert (
        llm.generate_cover_letter.await_args.kwargs["prompt_version"]
        == "generate-cover-letter-short-v1"
    )

    await service.generate(variant="full", **_content())
    assert (
        llm.generate_cover_letter.await_args.kwargs["prompt_version"]
        == "generate-cover-letter-full-v1"
    )

    with pytest.raises(ValueError):
        await service.generate(variant="medium", **_content())


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("variant", "prompt_version"),
    [
        ("short", "generate-cover-letter-short-v1"),
        ("full", "generate-cover-letter-full-v1"),
    ],
)
async def test_generate_returns_nested_metadata(
    variant: str, prompt_version: str
) -> None:
    letter = "Dear Hiring Manager,\n\nI am excited to apply.\n\nSincerely, Alex"
    llm = AsyncMock()
    llm.generate_cover_letter.return_value = letter
    service = GenerateCoverLetter(llm=llm, model="use-combo")

    result = await service.generate(variant=variant, **_content())

    assert result == {
        "text": letter,
        "metadata": {"model": "use-combo", "prompt_version": prompt_version},
    }


@pytest.mark.asyncio
async def test_generate_rejects_empty_llm_output() -> None:
    llm = AsyncMock()
    llm.generate_cover_letter.return_value = ""
    service = GenerateCoverLetter(llm=llm, model="use-combo")

    with pytest.raises(RuntimeError):
        await service.generate(variant="short", **_content())
