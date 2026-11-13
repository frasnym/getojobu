"""Temporal activity wrappers for Step 2: Compatibility Evaluation.

Thin layer: validates activity inputs, delegates to usecases/repositories,
and returns JSON-serializable results.
"""

from __future__ import annotations

from typing import Any, cast

from temporalio import activity

from config.config import load as config_load
from contract.contract import WorkerContainer
from contract.contract import new as contract_new

_container: WorkerContainer | None = None


def init_evaluate_match_container(
    container: WorkerContainer | None = None,
) -> WorkerContainer:
    """Build (or reuse) the dependency container."""
    global _container
    if container is not None:
        _container = container
        return _container

    if _container is None:
        cfg = config_load()
        c, _stopper, err = contract_new(cfg)
        if err is not None or c is None:
            raise err or RuntimeError("container initialization failed")
        _container = c
    return _container


@activity.defn
async def evaluate_match_activity(input: dict[str, Any]) -> dict[str, Any]:
    """Temporal activity for Step 2: Compatibility Evaluation using LLM."""
    container = init_evaluate_match_container()

    raw_candidate = input.get("candidate_profile")
    if not isinstance(raw_candidate, dict) or not raw_candidate:
        raise ValueError("candidate_profile is required")
    candidate_profile = cast(dict[str, Any], raw_candidate)

    raw_job = input.get("job_profile")
    if not isinstance(raw_job, dict) or not raw_job:
        raise ValueError("job_profile is required")
    job_profile = cast(dict[str, Any], raw_job)

    raw_config = input.get("evaluation_config")
    if not isinstance(raw_config, dict) or not raw_config:
        raise ValueError("evaluation_config is required")
    evaluation_config = cast(dict[str, Any], raw_config)

    usecase = container.usecase.evaluate_match
    if usecase is None:
        raise RuntimeError("evaluate_match usecase is not initialized")

    evaluation = await usecase.evaluate(
        candidate_profile=candidate_profile,
        job_profile=job_profile,
        evaluation_config=evaluation_config,
    )
    return evaluation.model_dump()


@activity.defn
async def update_evaluation_status_activity(input: dict[str, Any]) -> dict[str, Any]:
    """Temporal activity to update matching evaluation status via the Go API."""
    container = init_evaluate_match_container()

    evaluation_id = input.get("evaluation_id")
    if not evaluation_id or not isinstance(evaluation_id, str):
        raise ValueError("evaluation_id is required")

    status = input.get("status")
    if not status or not isinstance(status, str):
        raise ValueError("status is required")

    raw_eval_data = input.get("evaluation_data")
    eval_data = (
        cast(dict[str, Any], raw_eval_data)
        if isinstance(raw_eval_data, dict)
        else None
    )
    raw_fail_msg = input.get("failure_message")
    fail_msg = str(raw_fail_msg) if raw_fail_msg is not None else None
    raw_provider = input.get("provider")
    provider = str(raw_provider) if raw_provider is not None else None
    raw_model = input.get("model")
    model = (
        str(raw_model)
        if raw_model is not None
        else container.cfg.omni_route.model
    )

    return await container.repository.getojobu_api.patch_matching_evaluation(
        evaluation_id=evaluation_id,
        status=status,
        evaluation_data=eval_data,
        failure_message=fail_msg,
        provider=provider,
        model=model,
    )
