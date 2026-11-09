"""Aggregate contract: the single dependency container for the worker.

Mirrors apps/backend/internal/contract/contract.go: config → api client →
usecase, with a combined stopper for graceful shutdown.

The worker persists results via the Go API (internal Docker network), not
direct database access.
"""

from __future__ import annotations

from typing import Callable, TYPE_CHECKING

from config.config import Config
from deliveries.client.api_client import APIClient, HTTPAPIClient

if TYPE_CHECKING:
    from contract.usecase import Contract as UsecaseContract


Stopper = Callable[[], None]


class Contract:
    def __init__(
        self,
        cfg: Config,
        api_client: APIClient,
        usecase: "UsecaseContract",
    ) -> None:
        self.cfg = cfg
        self.api_client = api_client
        self.usecase = usecase


def new(cfg: Config | None) -> tuple[Contract, Stopper, Exception | None]:
    """Assemble the full contract from configuration.

    Returns (contract, stopper, error). On failure, contract is None and
    error describes why initialization failed (fail-fast).
    """
    if cfg is None:
        return None, None, ValueError("config is required")  # type: ignore[return-value]

    api_client = HTTPAPIClient(
        base_url=cfg.api.base_url,
        api_key=cfg.api.api_key,
    )

    from contract.usecase import new as usecase_new

    usecase_contract = usecase_new(
        api_client=api_client,
        llm_provider=cfg.llm.provider,
        llm_model=cfg.llm.model,
        llm_api_key=cfg.llm.api_key,
    )

    contract = Contract(
        cfg=cfg,
        api_client=api_client,
        usecase=usecase_contract,
    )

    stoppers: list[Stopper] = []

    async def _stop_api_client() -> None:
        await api_client.aclose()

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
