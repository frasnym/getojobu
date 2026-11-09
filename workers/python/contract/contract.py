"""Aggregate contract: the single dependency container for the worker.

Mirrors apps/backend/internal/contract/contract.go: config → common →
repository → usecase, with a combined stopper for graceful shutdown.
"""

from __future__ import annotations

from typing import Callable, TYPE_CHECKING

from config.config import Config
from contract.common import new as common_new
from contract.repository import new as repository_new
from contract.usecase import new as usecase_new

if TYPE_CHECKING:
    from contract.common import Contract as CommonContract
    from contract.repository import Contract as RepositoryContract
    from contract.usecase import Contract as UsecaseContract


Stopper = Callable[[], None]


class Contract:
    def __init__(
        self,
        cfg: Config,
        common: "CommonContract",
        repository: "RepositoryContract",
        usecase: "UsecaseContract",
    ) -> None:
        self.cfg = cfg
        self.common = common
        self.repository = repository
        self.usecase = usecase


def new(cfg: Config) -> tuple[Contract, Stopper, Exception | None]:
    """Assemble the full contract from configuration.

    Returns (contract, stopper, error). On failure, contract is None and
    error describes why initialization failed (fail-fast).
    """
    if cfg is None:
        return None, None, ValueError("config is required")  # type: ignore[return-value]

    common_contract, common_stopper = common_new(cfg.database.url)
    repository_contract = repository_new(common_contract)
    usecase_contract = usecase_new(
        repository_contract,
        llm_provider=cfg.llm.provider,
        llm_model=cfg.llm.model,
        llm_api_key=cfg.llm.api_key,
    )

    contract = Contract(
        cfg=cfg,
        common=common_contract,
        repository=repository_contract,
        usecase=usecase_contract,
    )

    stoppers: list[Stopper] = [common_stopper]

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
