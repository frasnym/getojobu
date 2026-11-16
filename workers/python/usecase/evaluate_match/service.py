"""Business logic for Step 2: Compatibility Evaluation.

Coordinates prompt rendering, LLM evaluation, and response validation.
"""

from __future__ import annotations

from typing import Any, Protocol, cast

from models.evaluation import MatchEvaluationOutput

# ---------------------------------------------------------------------------
# LLM output sanitisation helpers
#
# LLMs sometimes return synonyms or hallucinated values for constrained
# enum fields.  The helpers below normalise these values *before* Pydantic
# validation so that minor schema drift does not crash the activity.
# ---------------------------------------------------------------------------

_VALID_RELATIONSHIPS = frozenset({"direct", "transferable"})

# Mapping of common LLM hallucinated relationship values → valid value.
# Values containing "transfer" are mapped to "transferable"; everything else
# (including "gaps", "strengths", "match", etc.) defaults to "direct".
_RELATIONSHIP_MAP: dict[str, str] = {
    "direct": "direct",
    "transferable": "transferable",
    "transferable skills": "transferable",
    "transferable skill": "transferable",
    "indirect": "transferable",
    "related": "transferable",
    "partial": "transferable",
}

_VALID_STATUSES = frozenset({"met", "not_met", "uncertain"})

_STATUS_MAP: dict[str, str] = {
    "met": "met",
    "pass": "met",
    "passed": "met",
    "yes": "met",
    "satisfied": "met",
    "fulfilled": "met",
    "not_met": "not_met",
    "not met": "not_met",
    "unmet": "not_met",
    "fail": "not_met",
    "failed": "not_met",
    "no": "not_met",
    "unsatisfied": "not_met",
    "not_satisfied": "not_met",
    "uncertain": "uncertain",
    "unknown": "uncertain",
    "unclear": "uncertain",
    "partial": "uncertain",
    "partially": "uncertain",
    "maybe": "uncertain",
    "insufficient": "uncertain",
}


def _normalise_relationship(value: str) -> str:
    """Map an LLM-returned relationship string to a valid literal."""
    lowered = value.strip().lower()
    if lowered in _VALID_RELATIONSHIPS:
        return lowered
    mapped = _RELATIONSHIP_MAP.get(lowered)
    if mapped is not None:
        return mapped
    # Heuristic: anything containing "transfer" → transferable
    if "transfer" in lowered:
        return "transferable"
    return "direct"


def _normalise_status(value: str) -> str:
    """Map an LLM-returned mandatory-requirement status to a valid literal."""
    lowered = value.strip().lower()
    if lowered in _VALID_STATUSES:
        return lowered
    mapped = _STATUS_MAP.get(lowered)
    if mapped is not None:
        return mapped
    # Conservative default for anything we cannot classify.
    return "uncertain"


def _sanitise_evaluations(evaluations: list[object]) -> None:
    """Sanitise evidence relationship values in-place."""
    for raw_eval in evaluations:
        if not isinstance(raw_eval, dict):
            continue
        eval_dict = cast(dict[str, Any], raw_eval)
        evidence_list = eval_dict.get("evidence")
        if not isinstance(evidence_list, list):
            continue
        for ev in cast(list[object], evidence_list):
            if not isinstance(ev, dict):
                continue
            ev_dict = cast(dict[str, Any], ev)
            rel = ev_dict.get("relationship")
            if isinstance(rel, str) and rel not in _VALID_RELATIONSHIPS:
                ev_dict["relationship"] = _normalise_relationship(rel)


def _sanitise_mandatory_requirements(requirements: list[object]) -> None:
    """Sanitise mandatory requirement status values in-place."""
    for raw_req in requirements:
        if not isinstance(raw_req, dict):
            continue
        req_dict = cast(dict[str, Any], raw_req)
        status = req_dict.get("status")
        if isinstance(status, str) and status not in _VALID_STATUSES:
            req_dict["status"] = _normalise_status(status)


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

        categories_cfg = evaluation_config.get("categories", [])
        expected_categories = [
            c["name"]
            for c in categories_cfg
            if isinstance(c, dict) and "name" in c and isinstance(c["name"], str)
        ]

        evaluations_raw = raw_output.get("evaluations", [])
        if isinstance(evaluations_raw, list):
            # Sanitise evidence relationship values before category filtering.
            _sanitise_evaluations(cast(list[object], evaluations_raw))

            eval_by_cat: dict[str, dict[str, Any]] = {}
            for raw_item in cast(list[object], evaluations_raw):
                if not isinstance(raw_item, dict):
                    continue
                item = cast(dict[str, Any], raw_item)
                cat = str(item.get("category", "")).strip()
                if (
                    not expected_categories or cat in expected_categories
                ) and cat not in eval_by_cat:
                    eval_by_cat[cat] = item

            if expected_categories:
                for exp_cat in expected_categories:
                    if exp_cat not in eval_by_cat:
                        eval_by_cat[exp_cat] = {
                            "category": exp_cat,
                            "score": 50,
                            "reason": f"Evaluation automatically derived for {exp_cat} as it was not explicitly evaluated by the model.",
                            "strengths": [],
                            "gaps": [
                                f"Insufficient specific details for {exp_cat} in model evaluation"
                            ],
                            "evidence": [],
                        }

                raw_output["evaluations"] = [
                    eval_by_cat[exp_cat] for exp_cat in expected_categories
                ]
            else:
                raw_output["evaluations"] = list(eval_by_cat.values())

        # Sanitise mandatory requirement status values.
        mandatory_raw = raw_output.get("mandatory_requirements", [])
        if isinstance(mandatory_raw, list):
            _sanitise_mandatory_requirements(cast(list[object], mandatory_raw))

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

        raw_output.setdefault(
            "metadata", {"model": model, "prompt_version": self._prompt_version}
        )

        return MatchEvaluationOutput.model_validate(raw_output)
