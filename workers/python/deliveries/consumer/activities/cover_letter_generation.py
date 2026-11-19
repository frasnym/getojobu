"""Temporal activities for cover-letter generation."""

from __future__ import annotations

from typing import Any, cast

from temporalio import activity

from config.config import load as config_load
from contract.contract import WorkerContainer
from contract.contract import new as contract_new

_container: WorkerContainer | None = None


def init_cover_letter_generation_container(
    container: WorkerContainer | None = None,
) -> WorkerContainer:
    global _container
    if container is not None:
        _container = container
    if _container is None:
        cfg = config_load()
        built, _stopper, err = contract_new(cfg)
        if err is not None or built is None:
            raise err or RuntimeError("container initialization failed")
        _container = built
    return _container


@activity.defn
async def fetch_cover_letter_input_activity(input: dict[str, Any]) -> dict[str, Any]:
    """Fetch the stored cover-letter input and gate on cover_letter_status."""
    container = init_cover_letter_generation_container()

    evaluation_id = input.get("evaluation_id")
    if not evaluation_id or not isinstance(evaluation_id, str):
        raise ValueError("evaluation_id is required")

    evaluation = await container.repository.getojobu_api.get_evaluation(evaluation_id)
    status = evaluation.get("cover_letter_status")
    if status not in ("queued", "queued_retry"):
        return {"status": "skipped", "cover_letter_status": status}

    result: dict[str, Any] = {
        "status": "ready",
        "cover_letter_status": status,
        "overall_score": float(evaluation.get("overall_score") or 0),
        "eligibility_status": str(evaluation.get("eligibility_status") or "eligible"),
        "recommendation_label": str(
            evaluation.get("recommendation_label") or "worth_applying"
        ),
    }
    for name in ("candidate_profile", "job_profile", "evaluation_results", "facts"):
        value = evaluation.get(name)
        if not isinstance(value, dict) or not value:
            raise ValueError(f"{name} is required")
        result[name] = cast(dict[str, Any], value)
    return result


@activity.defn
async def generate_cover_letter_activity(input: dict[str, Any]) -> dict[str, Any]:
    container = init_cover_letter_generation_container()
    for name in ("candidate_profile", "job_profile", "evaluation_results", "facts"):
        if not isinstance(input.get(name), dict) or not input[name]:
            raise ValueError(f"{name} is required")
    variant = input.get("variant")
    if not isinstance(variant, str) or not variant:
        raise ValueError("variant is required")
    usecase = getattr(container.usecase, "generate_cover_letter", None)
    if usecase is None:
        raise RuntimeError("generate_cover_letter usecase is not initialized")
    return await usecase.generate(
        variant=variant,
        candidate_profile=cast(dict[str, Any], input["candidate_profile"]),
        job_profile=cast(dict[str, Any], input["job_profile"]),
        evaluation_results=cast(dict[str, Any], input["evaluation_results"]),
        facts=cast(dict[str, Any], input["facts"]),
        overall_score=float(input.get("overall_score", 0)),
        eligibility_status=str(input.get("eligibility_status", "eligible")),
        recommendation_label=str(input.get("recommendation_label", "")),
    )


@activity.defn
async def update_cover_letter_status_activity(input: dict[str, Any]) -> dict[str, Any]:
    container = init_cover_letter_generation_container()
    evaluation_id = input.get("evaluation_id")
    status = input.get("cover_letter_status")
    if not isinstance(evaluation_id, str) or not evaluation_id:
        raise ValueError("evaluation_id is required")
    if not isinstance(status, str) or not status:
        raise ValueError("cover_letter_status is required")
    raw_content = input.get("cover_letter_content")
    content: dict[str, Any] | None = (
        cast(dict[str, Any], raw_content) if isinstance(raw_content, dict) else None
    )
    raw_fail_msg = input.get("cover_letter_failure_message")
    fail_msg = str(raw_fail_msg) if raw_fail_msg is not None else None

    return (
        await container.repository.getojobu_api.patch_matching_evaluation_cover_letter(
            evaluation_id=evaluation_id,
            cover_letter_status=status,
            cover_letter_content=content,
            cover_letter_failure_message=fail_msg,
        )
    )
