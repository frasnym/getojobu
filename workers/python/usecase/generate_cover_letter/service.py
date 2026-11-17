"""Cover-letter generation by variant (short / full)."""

from __future__ import annotations

from typing import Any, Protocol

VARIANT_SHORT = "short"
VARIANT_FULL = "full"

_PROMPT_VERSIONS: dict[str, str] = {
    VARIANT_SHORT: "generate-cover-letter-short-v1",
    VARIANT_FULL: "generate-cover-letter-full-v1",
}


class CoverLetterLLMClient(Protocol):
    async def generate_cover_letter(
        self, *, prompt_version: str, **content: str
    ) -> str: ...


class GenerateCoverLetter:
    def __init__(
        self,
        llm: CoverLetterLLMClient | None = None,
        *,
        model: str = "",
    ) -> None:
        self._llm = llm
        self._model = model

    async def generate(self, *, variant: str, **content: Any) -> dict[str, Any]:
        prompt_version = _PROMPT_VERSIONS.get(variant)
        if prompt_version is None:
            raise ValueError(
                f"unknown cover letter variant: {variant!r}; expected one of "
                f"{sorted(_PROMPT_VERSIONS)}"
            )

        # Callers may supply evaluation_results (the resume/report convention)
        # instead of match_evidence; normalize to the prompt's placeholder name.
        if "match_evidence" not in content and "evaluation_results" in content:
            content = dict(content)
            content["match_evidence"] = content.pop("evaluation_results")

        if self._llm is None:
            raise RuntimeError("llm client is required for cover letter generation")

        text = await self._llm.generate_cover_letter(
            **content, prompt_version=prompt_version
        )
        if not text or not text.strip():
            raise RuntimeError("cover letter generation: empty LLM response")

        return {
            "text": text,
            "metadata": {"model": self._model, "prompt_version": prompt_version},
        }
