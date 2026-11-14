"""Temporal activities for the independent Step 5 resume pipeline."""

from __future__ import annotations

import base64
import json
from typing import Any, cast

from temporalio import activity

from config.config import load as config_load
from contract.contract import WorkerContainer
from contract.contract import new as contract_new

_container: WorkerContainer | None = None


def init_resume_generation_container(
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
async def generate_resume_activity(input: dict[str, Any]) -> dict[str, Any]:
    container = init_resume_generation_container()
    for name in ("candidate_profile", "job_profile", "evaluation_results", "facts"):
        if not isinstance(input.get(name), dict) or not input[name]:
            raise ValueError(f"{name} is required")
    usecase = getattr(container.usecase, "generate_resume", None)
    if usecase is None:
        raise RuntimeError("generate_resume usecase is not initialized")
    template_name = str(input["template_name"]) if input.get("template_name") else None
    output = await usecase.generate(
        candidate_profile=cast(dict[str, Any], input["candidate_profile"]),
        job_profile=cast(dict[str, Any], input["job_profile"]),
        evaluation_results=cast(dict[str, Any], input["evaluation_results"]),
        facts=cast(dict[str, Any], input["facts"]),
        overall_score=float(input.get("overall_score", 0)),
        eligibility_status=str(input.get("eligibility_status", "eligible")),
        recommendation_label=str(input.get("recommendation_label", "")),
        template_name=template_name,
    )
    output["pdf_base64"] = base64.b64encode(output.pop("pdf")).decode("ascii")
    return output


@activity.defn
async def upload_resume_activity(input: dict[str, Any]) -> dict[str, Any]:
    """Upload PDF and JSON artifacts to R2 object storage."""
    container = init_resume_generation_container()
    evaluation_id = input.get("evaluation_id")
    if not isinstance(evaluation_id, str) or not evaluation_id:
        raise ValueError("evaluation_id is required")

    job_id = input.get("job_id")
    user_id = input.get("user_id")

    if job_id and user_id:
        prefix = f"{job_id}/{user_id}"
    else:
        prefix = f"resumes/{evaluation_id}"

    pdf_key = f"{prefix}/{evaluation_id}_resume.pdf"
    content_key = f"{prefix}/{evaluation_id}_resume.json"

    pdf_base64 = input.get("pdf_base64")
    if isinstance(pdf_base64, str) and pdf_base64:
        pdf_bytes = base64.b64decode(pdf_base64.encode("ascii"))
        container.repository.storage.put_object(
            key=pdf_key,
            body=pdf_bytes,
            content_type="application/pdf",
        )

    resume_data = input.get("resume")
    if resume_data is not None:
        json_bytes = json.dumps(resume_data, indent=2).encode("utf-8")
        container.repository.storage.put_object(
            key=content_key,
            body=json_bytes,
            content_type="application/json",
        )

    return {
        "pdf_key": pdf_key,
        "content_key": content_key,
    }


@activity.defn
async def update_resume_status_activity(input: dict[str, Any]) -> dict[str, Any]:
    container = init_resume_generation_container()
    evaluation_id = input.get("evaluation_id")
    status = input.get("resume_status")
    if not isinstance(evaluation_id, str) or not evaluation_id:
        raise ValueError("evaluation_id is required")
    if not isinstance(status, str) or not status:
        raise ValueError("resume_status is required")
    raw_artifacts = input.get("resume_artifacts")
    artifacts: dict[str, Any] = (
        cast(dict[str, Any], raw_artifacts)
        if isinstance(raw_artifacts, dict)
        else {}
    )
    raw_fail_msg = input.get("resume_failure_message")
    fail_msg = str(raw_fail_msg) if raw_fail_msg is not None else None

    raw_pdf_key = artifacts.get("pdf_key") or input.get("resume_pdf_key")
    pdf_key = str(raw_pdf_key) if raw_pdf_key is not None else None

    raw_json_key = artifacts.get("content_key") or input.get("resume_json_key")
    json_key = str(raw_json_key) if raw_json_key is not None else None

    return await container.repository.getojobu_api.patch_matching_evaluation_resume(
        evaluation_id=evaluation_id,
        resume_status=status,
        resume_failure_message=fail_msg,
        resume_pdf_key=pdf_key,
        resume_json_key=json_key,
        resume_artifacts=artifacts,
    )
