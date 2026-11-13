"""Temporal activity wrappers for Step 4: Match Report Generation."""

from __future__ import annotations

from typing import Any, cast

from temporalio import activity

from config.config import load as config_load
from contract.contract import WorkerContainer
from contract.contract import new as contract_new

_container: WorkerContainer | None = None


def init_report_generation_container(
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
async def generate_report_activity(input: dict[str, Any]) -> dict[str, Any]:
    """Temporal activity for Step 4: Match Report Generation using LLM."""
    container = init_report_generation_container()

    raw_candidate = input.get("candidate_profile")
    if not isinstance(raw_candidate, dict) or not raw_candidate:
        raise ValueError("candidate_profile is required")
    candidate_profile = cast(dict[str, Any], raw_candidate)

    raw_job = input.get("job_profile")
    if not isinstance(raw_job, dict) or not raw_job:
        raise ValueError("job_profile is required")
    job_profile = cast(dict[str, Any], raw_job)

    raw_eval_results = input.get("evaluation_results")
    if not isinstance(raw_eval_results, dict) or not raw_eval_results:
        raise ValueError("evaluation_results is required")
    evaluation_results = cast(dict[str, Any], raw_eval_results)

    raw_score = input.get("overall_score")
    if raw_score is None:
        raise ValueError("overall_score is required")
    overall_score = float(raw_score)

    raw_eligibility = input.get("eligibility_status")
    if not raw_eligibility or not isinstance(raw_eligibility, str):
        raise ValueError("eligibility_status is required")
    eligibility_status = str(raw_eligibility)

    raw_rec = input.get("recommendation_label")
    if not raw_rec or not isinstance(raw_rec, str):
        raise ValueError("recommendation_label is required")
    recommendation_label = str(raw_rec)

    usecase = container.usecase.generate_report
    if usecase is None:
        raise RuntimeError("generate_report usecase is not initialized")

    report = await usecase.generate(
        candidate_profile=candidate_profile,
        job_profile=job_profile,
        evaluation_results=evaluation_results,
        overall_score=overall_score,
        eligibility_status=eligibility_status,
        recommendation_label=recommendation_label,
    )
    return report.model_dump()


@activity.defn
async def update_report_status_activity(input: dict[str, Any]) -> dict[str, Any]:
    """Temporal activity to update matching evaluation report status via the Go API."""
    container = init_report_generation_container()

    evaluation_id = input.get("evaluation_id")
    if not evaluation_id or not isinstance(evaluation_id, str):
        raise ValueError("evaluation_id is required")

    report_status = input.get("report_status")
    if not report_status or not isinstance(report_status, str):
        raise ValueError("report_status is required")

    raw_report_data = input.get("report_data")
    report_data = (
        cast(dict[str, Any], raw_report_data)
        if isinstance(raw_report_data, dict)
        else None
    )
    raw_fail_msg = input.get("report_failure_message")
    report_fail_msg = str(raw_fail_msg) if raw_fail_msg is not None else None

    return await container.repository.getojobu_api.patch_matching_evaluation_report(
        evaluation_id=evaluation_id,
        report_status=report_status,
        report_data=report_data,
        report_failure_message=report_fail_msg,
    )
