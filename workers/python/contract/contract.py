"""Aggregate dependency container for the worker.

Mirrors apps/backend/internal/contract/contract.go: config → repository →
usecase, with a combined stopper for graceful shutdown.

The worker persists results via the Go API (internal Docker network), not
direct database access.
"""

from __future__ import annotations

import logging
from collections.abc import Callable, Coroutine
from typing import TYPE_CHECKING, Any

logger = logging.getLogger(__name__)

from config.config import Config
from contract.repository import new as repository_new
from contract.usecase import new as usecase_new

if TYPE_CHECKING:
    from contract.repository import RepositoryContract
    from contract.usecase import UsecaseContract


Stopper = Callable[[], Coroutine[Any, Any, None]]


class WorkerContainer:
    def __init__(
        self,
        cfg: Config,
        repository: RepositoryContract,
        usecase: UsecaseContract,
    ) -> None:
        self.cfg = cfg
        self.repository = repository
        self.usecase = usecase


def new(
    cfg: Config | None,
) -> tuple[WorkerContainer | None, Stopper | None, Exception | None]:
    """Assemble the full dependency container from configuration.

    Returns (container, stopper, error). On failure, container is None and
    error describes why initialization failed (fail-fast).
    """
    if cfg is None:
        return None, None, ValueError("config is required")

    repository_contract = repository_new(cfg)
    usecase_contract = usecase_new(
        api=repository_contract.api,
        omni_base_url=cfg.omni_route.base_url,
        omni_api_key=cfg.omni_route.api_key,
        omni_model=cfg.omni_route.model,
    )

    container = WorkerContainer(
        cfg=cfg,
        repository=repository_contract,
        usecase=usecase_contract,
    )

    stoppers: list[Stopper] = []

    async def _stop_api() -> None:
        await repository_contract.api.aclose()

    stoppers.append(_stop_api)

    async def stopper() -> None:
        # Run every stopper even when one fails.
        for st in stoppers:
            if st is None:
                continue
            try:
                await st()
            except Exception as exc:  # noqa: BLE001 - intentional: log and continue
                logger.warning("stopper_failed", error=str(exc))

    return container, stopper, None
