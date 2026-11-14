"""Business logic for Step 2: Compatibility Evaluation.

Coordinates prompt rendering, LLM evaluation, and response validation.
"""

from __future__ import annotations

from typing import Any, Protocol

from models.evaluation import MatchEvaluationOutput


class LLMClient(Protocol):
    """Protocol for an LLM client used to extract match evaluation."""

    async def extract_match_evaluation(
        self,
        *,
        candidate_profile: dict[str, Any],
        job_profile: dict[str, Any],
        evaluation_config: dict[str, Any],
        prompt_version: str = "evaluate-match-v1",
    ) -> dict[str, Any]: ...


class EvaluateMatch:
    """Service that coordinates match evaluation via LLM."""

    def __init__(
        self,
        llm: LLMClient | None = None,
        *,
        prompt_version: str = "evaluate-match-v1",
        model: str | None = None,
        provider: str | None = None,
    ) -> None:
        self._llm = llm
        self._prompt_version = prompt_version
        self._model = model
        self._provider = provider

    async def evaluate(
        self,
        *,
        candidate_profile: dict[str, Any],
        job_profile: dict[str, Any],
        evaluation_config: dict[str, Any],
    ) -> MatchEvaluationOutput:
        """Run the compatibility evaluation using the candidate, job, and config payloads."""
        if not candidate_profile:
            raise ValueError("candidate_profile is required")
        if not job_profile:
            raise ValueError("job_profile is required")
        if not evaluation_config:
            raise ValueError("evaluation_config is required")

        if self._llm is None:
            raise RuntimeError("llm client is required for match evaluation")

        raw_output = await self._llm.extract_match_evaluation(
            candidate_profile=candidate_profile,
            job_profile=job_profile,
            evaluation_config=evaluation_config,
            prompt_version=self._prompt_version,
        )

        model = self._model if isinstance(self._model, str) else None
        if not model:
            val = getattr(self._llm, "_model", None)
            if isinstance(val, str):
                model = val

        provider = self._provider if isinstance(self._provider, str) else None
        if not provider:
            val = getattr(self._llm, "_provider", None)
            if isinstance(val, str):
                provider = val

        raw_output.setdefault("prompt_version", self._prompt_version)
        raw_output.setdefault("model", model)
        raw_output.setdefault("provider", provider)

        return MatchEvaluationOutput.model_validate(raw_output)
