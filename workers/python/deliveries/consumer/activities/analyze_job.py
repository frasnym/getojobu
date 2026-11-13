"""Temporal activity wrappers for Step 1B.

Thin layer: validates the activity input, delegates to the usecase, and
returns a JSON-serializable result.
"""

from __future__ import annotations

from typing import Any

from temporalio import activity

from config.config import load as config_load
from contract.contract import WorkerContainer
from contract.contract import new as contract_new

# The container is built once per worker process (in worker.main) and
# attached to the module so activities can reuse the usecase.
_container: WorkerContainer | None = None


def init_analyze_job_container(
    container: WorkerContainer | None = None,
) -> WorkerContainer:
    """Build (or reuse) the dependency container. Called from worker.main
    before starting the Temporal worker."""
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
async def analyze_job_activity(input: dict[str, Any]) -> dict[str, Any]:
    """Temporal activity for Step 1B: Analyze Job Description.

    Input:
        {
            "slug": str,             # required slug of the job listing
            "job_description": str,  # optional (fetched from Go API if omitted)
            "job_id": str | None,    # optional reference to shared jobs table
        }

    Output: the stored JobAnalysis as a dict.
    """
    container = init_analyze_job_container()
    slug = input.get("slug")
    if not slug or not isinstance(slug, str):
        raise ValueError("slug is required")

    analysis = await container.usecase.analyze_job.analyze(
        slug=slug,
        job_description=input.get("job_description"),
        job_id=input.get("job_id"),
    )
    return analysis.model_dump()
