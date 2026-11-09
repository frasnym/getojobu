"""Aggregate contract: the single dependency container for the worker.

Mirrors apps/backend/internal/contract/contract.go: config → repository →
usecase, with a combined stopper for graceful shutdown.

The worker persists results via the Go API (internal Docker network), not
direct database access.
"""

from __future__ import annotations

from typing import Callable, List, TYPE_CHECKING

from config.config import Config
from contract.repository import new as repository_new
from contract.usecase import new as usecase_new

if TYPE_CHECKING:
    from contract.repository import Contract as RepositoryContract
    from contract.usecase import Contract as UsecaseContract


Stopper = Callable[[], None]


class Contract:
    def __init__(
        self,
        cfg: Config,
        repository: "RepositoryContract",
        usecase: "UsecaseContract",
    ) -> None:
        self.cfg = cfg
        self.repository = repository
        self.usecase = usecase


def new(cfg: Config | None) -> tuple[Contract, Stopper, Exception | None]:
    """Assemble the full contract from configuration.

    Returns (contract, stopper, error). On failure, contract is None and
    error describes why initialization failed (fail-fast).
    """
    if cfg is None:
        return None, None, ValueError("config is required")  # type: ignore[return-value]

    repository_contract = repository_new(cfg)
    usecase_contract = usecase_new(
        api_client=repository_contract.api_client,
        omni_base_url=cfg.omni_route.base_url,
        omni_api_key=cfg.omni_route.api_key,
        omni_model=cfg.omni_route.model,
    )

    contract = Contract(
        cfg=cfg,
        repository=repository_contract,
        usecase=usecase_contract,
    )

    stoppers: List[Stopper] = []

    async def _stop_api_client() -> None:
        await repository_contract.api_client.aclose()

    stoppers.append(_stop_api_client)

    def stopper() -> None:
        # Run every stopper even when one fails.
        for st in stoppers:
            if st is None:
                continue
            try:
                st()
            except Exception:
                pass

    return contract, stopper, None
