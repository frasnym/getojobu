"""Temporal worker entry point.

Mirrors apps/backend/cmd/api/main.go: config.Load() → contract.New() →
delivery → graceful start/stop. Here: config → contract → Temporal worker
→ graceful start/stop.

The worker polls the job-analyze and job-scrape task queues and
executes their respective activities and workflows.
"""

from __future__ import annotations

import asyncio
import signal
import sys
from concurrent.futures import ThreadPoolExecutor
from datetime import timedelta

from temporalio.worker import Worker

from config.config import load as config_load
from contract.contract import new as contract_new
from deliveries.consumer.activities.analyze_job import (
    analyze_job_activity,
    init_analyze_job_container,
)
from deliveries.consumer.activities.evaluate_match import (
    evaluate_match_activity,
    init_evaluate_match_container,
    update_evaluation_status_activity,
)
from deliveries.consumer.activities.job_scrape import (
    enrich_jobs,
    ingest_jobs,
    queue_analysis,
    scrape_scan,
)
from deliveries.consumer.activities.report_generation import (
    generate_report_activity,
    init_report_generation_container,
    update_report_status_activity,
)
from deliveries.consumer.activities.resume_generation import (
    generate_resume_activity,
    init_resume_generation_container,
    update_resume_status_activity,
    upload_resume_activity,
)
from utils.logging import configure, get_logger
from utils.temporal import connect as temporal_connect
from workflows.concurrency_limiter import ConcurrencyLimiter
from workflows.generate_report import GenerateReportWorkflow
from workflows.generate_resume import GenerateResumeWorkflow
from workflows.job_analyze import JobAnalyzeWorkflow
from workflows.job_scrape import JobScrapeWorkflow
from workflows.match_job import MatchJobWorkflow

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
    assert stopper is not None

    # Connect to Temporal (reuses existing Hermes Temporal via a dedicated
    # getojobu namespace; no new Temporal deployment needed).
    try:
        temporal_client = await temporal_connect(
            host=cfg.temporal.host, namespace=cfg.temporal.namespace
        )
    except Exception as exc:  # noqa: BLE001
        logger.error("temporal_connect_failed", error=str(exc))
        await stopper()
        return 1

    # Initialize the activity containers (reuse the contract already built).
    init_analyze_job_container(contract)
    init_evaluate_match_container(contract)
    init_report_generation_container(contract)
    init_resume_generation_container(contract)

    activity_executor = ThreadPoolExecutor(max_workers=10)

    graceful_timeout = timedelta(seconds=30)

    try:
        job_analyze_worker = Worker(
            temporal_client.client,
            task_queue=cfg.temporal.task_queue_job_analyze,
            activities=[
                analyze_job_activity,
                evaluate_match_activity,
                update_evaluation_status_activity,
                generate_report_activity,
                update_report_status_activity,
                generate_resume_activity,
                upload_resume_activity,
                update_resume_status_activity,
            ],
            workflows=[
                JobAnalyzeWorkflow,
                MatchJobWorkflow,
                GenerateReportWorkflow,
                GenerateResumeWorkflow,
            ],
            activity_executor=activity_executor,
            graceful_shutdown_timeout=graceful_timeout,
        )
        job_scrape_worker = Worker(
            temporal_client.client,
            task_queue=cfg.temporal.task_queue_job_scrape,
            activities=[scrape_scan, enrich_jobs, ingest_jobs, queue_analysis],
            workflows=[JobScrapeWorkflow],
            activity_executor=activity_executor,
            graceful_shutdown_timeout=graceful_timeout,
        )
        job_limit_worker = Worker(
            temporal_client.client,
            task_queue=cfg.temporal.task_queue_job_limit,
            workflows=[ConcurrencyLimiter],
            graceful_shutdown_timeout=graceful_timeout,
        )

        logger.info(
            "worker_starting",
            job_analyze_queue=cfg.temporal.task_queue_job_analyze,
            job_scrape_queue=cfg.temporal.task_queue_job_scrape,
            job_limit_queue=cfg.temporal.task_queue_job_limit,
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

        # Run workers until a shutdown signal is received.
        job_analyze_task = asyncio.create_task(job_analyze_worker.run())
        job_scrape_task = asyncio.create_task(job_scrape_worker.run())
        job_limit_task = asyncio.create_task(job_limit_worker.run())
        try:
            await stop_event.wait()
        finally:
            logger.info("initiating_graceful_shutdown")
            await asyncio.gather(
                job_analyze_worker.shutdown(),
                job_scrape_worker.shutdown(),
                job_limit_worker.shutdown(),
                return_exceptions=True,
            )
            for t in (job_analyze_task, job_scrape_task, job_limit_task):
                try:
                    await t
                except asyncio.CancelledError:
                    pass

        logger.info("worker_stopped")
        await stopper()
        return 0
    finally:
        activity_executor.shutdown(wait=True)


def main() -> None:
    exit_code = asyncio.run(run())
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
