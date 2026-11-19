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

from temporalio.contrib.opentelemetry import TracingInterceptor
from temporalio.worker import Worker

from config.config import load as config_load
from contract.contract import new as contract_new
from deliveries.consumer.activities.analyze_job import (
    analyze_job_activity,
    init_analyze_job_container,
)
from deliveries.consumer.activities.cover_letter_generation import (
    generate_cover_letter_activity,
    init_cover_letter_generation_container,
    update_cover_letter_status_activity,
)
from deliveries.consumer.activities.evaluate_match import (
    evaluate_match_activity,
    init_evaluate_match_container,
    update_evaluation_status_activity,
)
from deliveries.consumer.activities.job_scrape import (
    enrich_jobs,
    ingest_jobs,
    init_job_scrape_container,
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
from deliveries.consumer.activities.user_job_scrape import (
    fetch_and_extract_job,
    init_user_job_scrape_container,
    mark_scrape_failed,
    mark_scrape_pending,
    queue_scraped_job_analysis,
    update_scraped_job,
)
from utils.logging import BUILD_VERSION, configure, get_logger
from utils.telemetry import setup_otel
from utils.temporal import connect as temporal_connect
from workflows.concurrency_limiter import ConcurrencyLimiter
from workflows.generate_cover_letter import GenerateCoverLetterWorkflow
from workflows.generate_report import GenerateReportWorkflow
from workflows.generate_resume import GenerateResumeWorkflow
from workflows.job_analyze import JobAnalyzeWorkflow
from workflows.job_scrape import JobScrapeWorkflow
from workflows.match_job import MatchJobWorkflow
from workflows.user_job_scrape import UserJobScrapeWorkflow

logger = get_logger("worker.main")


def build_worker_interceptors(otel_enabled: bool) -> list[TracingInterceptor]:
    """Build Temporal interceptors that propagate trace context when OTel is on.

    always_create_workflow_spans keeps a RunWorkflow span for workflows started
    without a client span (cron, CLI, schedules). Without it those workflows have
    no parent context and each activity shows up as an orphan root trace.
    """
    if not otel_enabled:
        return []
    return [TracingInterceptor(always_create_workflow_spans=True)]


async def run() -> int:
    cfg = config_load()
    setup_otel(cfg.otel_endpoint)
    configure(level="info")
    logger.info("worker_started", build=BUILD_VERSION)

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
    init_cover_letter_generation_container(contract)
    init_report_generation_container(contract)
    init_resume_generation_container(contract)
    init_job_scrape_container(contract)
    init_user_job_scrape_container(contract)

    activity_executor = ThreadPoolExecutor(max_workers=10)

    graceful_timeout = timedelta(seconds=30)

    # Propagate trace context through Temporal (child spans of the requesting
    # API call) only when OTel is enabled.
    worker_interceptors = build_worker_interceptors(bool(cfg.otel_endpoint))

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
                generate_cover_letter_activity,
                update_cover_letter_status_activity,
            ],
            workflows=[
                JobAnalyzeWorkflow,
                MatchJobWorkflow,
                GenerateReportWorkflow,
                GenerateResumeWorkflow,
                GenerateCoverLetterWorkflow,
            ],
            activity_executor=activity_executor,
            graceful_shutdown_timeout=graceful_timeout,
            build_id=BUILD_VERSION,
            interceptors=worker_interceptors,
        )
        job_scrape_worker = Worker(
            temporal_client.client,
            task_queue=cfg.temporal.task_queue_job_scrape,
            activities=[
                scrape_scan,
                enrich_jobs,
                ingest_jobs,
                queue_analysis,
                fetch_and_extract_job,
                update_scraped_job,
                queue_scraped_job_analysis,
                mark_scrape_failed,
                mark_scrape_pending,
            ],
            workflows=[JobScrapeWorkflow, UserJobScrapeWorkflow],
            activity_executor=activity_executor,
            graceful_shutdown_timeout=graceful_timeout,
            build_id=BUILD_VERSION,
            interceptors=worker_interceptors,
        )
        job_limit_worker = Worker(
            temporal_client.client,
            task_queue=cfg.temporal.task_queue_job_limit,
            workflows=[ConcurrencyLimiter],
            graceful_shutdown_timeout=graceful_timeout,
            build_id=BUILD_VERSION,
            interceptors=worker_interceptors,
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
