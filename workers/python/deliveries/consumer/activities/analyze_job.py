"""Temporal activity wrappers for Step 1B.

Thin layer: validates the activity input, delegates to the usecase, and
returns a JSON-serializable result. Mirrors the handler/ layer in
apps/backend/internal/deliveries/api/handler/.
"""

from __future__ import annotations

from typing import Any

from temporalio import activity

from contract.contract import WorkerContainer, new as contract_new
from config.config import load as config_load


# The container is built once per worker process (in worker.main) and
# attached to the module so activities can reuse the usecase.
_container: WorkerContainer | None = None


def init_container(container: WorkerContainer | None = None) -> WorkerContainer:
    """Build (or reuse) the dependency container. Called from worker.main
    before starting the Temporal worker."""
    global _container
    if _container is None:
        if container is not None:
            _container = container
        else:
            cfg = config_load()
            c, _stopper, err = contract_new(cfg)
            if err is not None or c is None:
                raise err or RuntimeError("container initialization failed")
            _container = c
    return _container


@activity.defn
async def analyze_job_activity(input: dict[str, Any]) -> dict[str, Any]:
    """Temporal activity for Step 1B: Analyze Job Description.

    Input:
        {
            "job_description": str,  # required
            "job_id": str | None,    # optional reference to shared jobs table
        }

    Output: the stored JobProfile as a dict.
    """
    global _container
    if _container is None:
        _container = init_container()

    assert _container is not None
    usecase = _container.usecase.analyze_job

    job_description = input.get("job_description")
    if not isinstance(job_description, str) or not job_description.strip():
        raise ValueError("job_description is required")

    job_id = input.get("job_id")
    if job_id is not None and not isinstance(job_id, str):
        raise ValueError("job_id must be a string")

    profile = await usecase.execute(
        job_description=job_description,
        job_id=job_id,
    )

    return profile.model_dump()
