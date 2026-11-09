"""Temporal worker entry point.

Mirrors apps/backend/cmd/api/main.go: config.Load() → contract.New() →
delivery → graceful start/stop. Here: config → contract → Temporal worker
→ graceful start/stop.

The worker polls the job-evaluation task queue and executes Step 1B
activities (LLM-powered job description analysis).
"""

from __future__ import annotations

import asyncio
import signal
import sys

from temporalio.worker import Worker

from config.config import load as config_load
from contract.contract import new as contract_new
from deliveries.consumer.activities.analyze_job import (
    analyze_job_activity,
    init_container,
)
from utils.logging import configure, get_logger
from utils.temporal import connect as temporal_connect
from workflows.job_analyze import JobAnalyzeWorkflow

logger = get_logger("worker.main")


async def run() -> int:
    cfg = config_load()
    configure(level="info")

    # Build the dependency container (database + repositories + usecases).
    contract, stopper, err = contract_new(cfg)
    if err is not None:
        logger.error("contract_init_failed", error=str(err))
        return 1

    assert contract is not None

    # Connect to Temporal (reuses existing Hermes Temporal via a dedicated
    # getojobu namespace; no new Temporal deployment needed).
    try:
        temporal_client = await temporal_connect(
            host=cfg.temporal.host, namespace=cfg.temporal.namespace
        )
    except Exception as exc:  # noqa: BLE001
        logger.error("temporal_connect_failed", error=str(exc))
        stopper()
        return 1

    # Initialize the activity container (builds usecase once).
    init_container()

    worker = Worker(
        temporal_client.client,
        task_queue=cfg.temporal.task_queue_job_analyze,
        activities=[analyze_job_activity],
        workflows=[JobAnalyzeWorkflow],
    )

    logger.info(
        "worker_starting",
        task_queue=cfg.temporal.task_queue_job_analyze,
        namespace=cfg.temporal.namespace,
    )

    loop = asyncio.get_running_loop()
    stop_event = asyncio.Event()

    def _handle_signal() -> None:
        logger.info("shutdown_signal_received")
        stop_event.set()

    for sig in (signal.SIGINT, signal.SIGTERM):
        try:
            loop.add_signal_handler(sig, _handle_signal)
        except NotImplementedError:  # pragma: no cover - Windows
            pass

    # Run the worker until a shutdown signal is received.
    async with worker:
        task = asyncio.create_task(worker.run())
        try:
            await stop_event.wait()
        finally:
            task.cancel()
            try:
                await task
            except asyncio.CancelledError:
                pass

    logger.info("worker_stopped")
    stopper()
    return 0


def main() -> None:
    exit_code = asyncio.run(run())
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
