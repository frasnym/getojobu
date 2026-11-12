"""ConcurrencyLimiter workflow.

Singleton dispatcher workflow (stable ID: job-concurrency-limiter) that owns
a shared FIFO queue and concurrency counter (self.active). External callers reach
it via Temporal SignalWithStartWorkflow on the "enqueue" signal.

Enforces concurrency limits:
- Scrape: 1 in flight
- Analyze: 2 in flight
- Hard cap: 3 in flight total
"""

from __future__ import annotations

import asyncio
from collections.abc import Mapping
from dataclasses import dataclass
from typing import Any, cast

from temporalio import workflow
from temporalio.common import WorkflowIDReusePolicy

with workflow.unsafe.imports_passed_through():
    from utils.logging import get_logger

_fallback_logger = get_logger("workflow.concurrency_limiter")


class _SafeLogger:
    """Uses workflow.logger inside Temporal workflow sandbox; falls back to structlog in unit tests."""

    def info(self, msg: str, *args: Any, **kwargs: Any) -> None:
        try:
            workflow.logger.info(msg, *args, **kwargs)
        except Exception:  # noqa: BLE001
            _fallback_logger.info(msg, *args, **kwargs)

    def warning(self, msg: str, *args: Any, **kwargs: Any) -> None:
        try:
            workflow.logger.warning(msg, *args, **kwargs)
        except Exception:  # noqa: BLE001
            _fallback_logger.warning(msg, *args, **kwargs)

    def error(self, msg: str, *args: Any, **kwargs: Any) -> None:
        try:
            workflow.logger.error(msg, *args, **kwargs)
        except Exception:  # noqa: BLE001
            _fallback_logger.error(msg, *args, **kwargs)


logger = _SafeLogger()

LIMITS: dict[str, int] = {
    "scrape": 1,
    "analyze": 2,
}

QUEUE_BY_TYPE: dict[str, str] = {
    "scrape": "job-scrape",
    "analyze": "job-evaluation",
}

WORKFLOW_BY_TYPE: dict[str, str] = {
    "scrape": "JobScrapeWorkflow",
    "analyze": "JobAnalyzeWorkflow",
}


@dataclass
class WorkItem:
    params: dict[str, Any]
    item_id: str
    job_type: str

    @classmethod
    def from_dict(cls, data: Mapping[str, Any]) -> WorkItem:
        params_val = data.get("params")
        params: dict[str, Any] = (
            cast(dict[str, Any], params_val)
            if isinstance(params_val, dict)
            else {}
        )
        item_id: str = str(data.get("item_id", ""))
        job_type: str = str(data.get("job_type", "scrape"))
        return cls(params=params, item_id=item_id, job_type=job_type)

    def to_dict(self) -> dict[str, Any]:
        return {
            "params": self.params,
            "item_id": self.item_id,
            "job_type": self.job_type,
        }


class WorkItemError(Exception):
    """Raised or logged when a child workflow execution fails."""

    def __init__(self, item_id: str, job_type: str, message: str) -> None:
        super().__init__(f"[{job_type}] {item_id}: {message}")
        self.item_id = item_id
        self.job_type = job_type
        self.message = message


@dataclass
class _ActiveEntry:
    item: WorkItem
    task: asyncio.Task[Any] | asyncio.Future[Any]


CONTINUE_AS_NEW_PATCH_ID = "ConcurrencyLimiter.continue-as-new-v1"


def _should_continue_as_new(
    enabled: bool, suggested: bool, active_count: int, handlers_done: bool
) -> bool:
    """Reset the dispatcher history only when permitted and safe.

    - enabled: workflow.patched marker ensuring deterministic replay.
    - suggested: Temporal server indicates history size or count threshold reached.
    - active_count == 0: no child workflow in flight (cannot orphan child tasks).
    - handlers_done: no signal handlers still running (prevents missing concurrent enqueues).
    """
    return enabled and suggested and active_count == 0 and handlers_done


def _safe_is_continue_as_new_suggested() -> bool:
    """Check if Temporal server suggested continue-as-new; False outside workflow runtime."""
    try:
        return workflow.info().is_continue_as_new_suggested()
    except Exception:  # noqa: BLE001
        return False


def _safe_all_handlers_finished() -> bool:
    """Check if all workflow signal/update handlers finished; True outside workflow runtime."""
    try:
        return workflow.all_handlers_finished()
    except Exception:  # noqa: BLE001
        return True


async def _safe_wait_condition(fn: Any) -> None:
    """Wait for condition; falls back to asyncio polling if outside Temporal runtime."""
    try:
        await workflow.wait_condition(fn)
    except Exception:  # noqa: BLE001
        while not fn():
            await asyncio.sleep(0.005)


def _safe_patched(patch_id: str) -> bool:
    """Check workflow patch; falls back to True outside Temporal runtime."""
    try:
        return workflow.patched(patch_id)
    except Exception:  # noqa: BLE001
        return True


def _safe_continue_as_new(*args: Any, **kwargs: Any) -> None:
    """Call continue_as_new; falls back to no-op outside Temporal runtime."""
    try:
        workflow.continue_as_new(*args, **kwargs)
    except Exception:  # noqa: BLE001, S110
        pass


@workflow.defn(name="ConcurrencyLimiter")
class ConcurrencyLimiter:
    def __init__(self) -> None:
        self.queue: list[WorkItem] = []
        # item_id -> _ActiveEntry
        self.active: dict[str, _ActiveEntry] = {}
        self.failed_items: list[dict[str, Any]] = []
        self._running: bool = False
        self._processed_count: int = 0

    def _count_active_of_type(self, job_type: str) -> int:
        return sum(
            1 for entry in self.active.values() if entry.item.job_type == job_type
        )

    def _can_dispatch_any(self) -> bool:
        """Check if any item currently in the queue can be dispatched within capacity limits."""
        return any(
            self._count_active_of_type(item.job_type) < LIMITS.get(item.job_type, 1)
            for item in self.queue
        )

    def _create_child_task(
        self, item: WorkItem
    ) -> asyncio.Task[Any] | asyncio.Future[Any]:
        return asyncio.create_task(self._run_child_workflow(item))

    async def _run_child_workflow(self, item: WorkItem) -> Any:
        wf_name = WORKFLOW_BY_TYPE.get(item.job_type, "JobScrapeWorkflow")
        task_queue = QUEUE_BY_TYPE.get(item.job_type, "job-scrape")

        logger.info(
            "dispatching_child_workflow: item_id=%s job_type=%s workflow=%s task_queue=%s",
            item.item_id,
            item.job_type,
            wf_name,
            task_queue,
        )

        handle = await workflow.start_child_workflow(
            wf_name,
            item.params,
            id=item.item_id,
            task_queue=task_queue,
            id_reuse_policy=WorkflowIDReusePolicy.ALLOW_DUPLICATE,
        )
        return await handle

    def dispatch_pending(self) -> None:
        """Move eligible items from self.queue to self.active while under capacity."""
        idx = 0
        while idx < len(self.queue):
            candidate = self.queue[idx]
            cap = LIMITS.get(candidate.job_type, 1)
            active_count = self._count_active_of_type(candidate.job_type)
            if active_count < cap:
                item = self.queue.pop(idx)
                self._processed_count += 1
                task = self._create_child_task(item)
                self.active[item.item_id] = _ActiveEntry(item=item, task=task)
            else:
                idx += 1

    @workflow.signal(name="enqueue")
    def enqueue(self, item: WorkItem | dict[str, Any], ctx: Any = None) -> None:
        """Enqueue a work item into the limiter queue.

        Deduplicates if item_id is already in active or queue.
        """
        if isinstance(item, dict):
            work_item = WorkItem.from_dict(item)
        else:
            work_item = item

        item_id = work_item.item_id
        if not item_id:
            logger.warning("enqueue_rejected_empty_id")
            return

        # Deduplication check: ignore if already active or queued
        if item_id in self.active or any(i.item_id == item_id for i in self.queue):
            logger.info("enqueue_ignored_duplicate: item_id=%s", item_id)
            return

        self.queue.append(work_item)
        if not self._running:
            self.dispatch_pending()

    @workflow.query(name="queue_status")
    def queue_status(self) -> dict[str, Any]:
        """Return the current queue status and capacity."""
        return {
            "queued": len(self.queue),
            "active": list(self.active.keys()),
            "capacity_by_type": dict(LIMITS),
            "failed_items": list(self.failed_items),
        }

    async def _handle_completed_tasks(
        self, done_tasks: set[asyncio.Task[Any] | asyncio.Future[Any]]
    ) -> None:
        """Inspect completed tasks, record failures without crashing, and cleanup active."""
        for item_id, entry in list(self.active.items()):
            if entry.task in done_tasks:
                del self.active[item_id]
                exc: BaseException | None = None
                try:
                    exc = entry.task.exception()
                except (asyncio.CancelledError, Exception) as e:  # noqa: BLE001
                    exc = e

                if exc is not None:
                    err_msg = str(exc)
                    logger.error(
                        "work_item_failed: item_id=%s job_type=%s error=%s",
                        item_id,
                        entry.item.job_type,
                        err_msg,
                    )
                    self.failed_items.append(
                        {
                            "item_id": item_id,
                            "job_type": entry.item.job_type,
                            "error": err_msg,
                        }
                    )

    def _apply_state(self, state: dict[str, Any] | None) -> None:
        """Seed queue and failed_items from a prior generation's Continue-As-New state."""
        if not state:
            return
        failed_raw = state.get("failed_items")
        if isinstance(failed_raw, list):
            self.failed_items = [
                cast(dict[str, Any], f)
                for f in cast(list[object], failed_raw)
                if isinstance(f, dict)
            ]
        raw_queue = state.get("queue")
        if isinstance(raw_queue, list):
            for raw_item in cast(list[object], raw_queue):
                if isinstance(raw_item, WorkItem):
                    self.queue.append(raw_item)
                elif isinstance(raw_item, dict):
                    self.queue.append(
                        WorkItem.from_dict(cast(dict[str, Any], raw_item))
                    )

    def continuation_state(self) -> dict[str, Any]:
        """Snapshot current queue and recent failed_items for Continue-As-New."""
        return {
            "queue": [item.to_dict() for item in self.queue],
            "failed_items": self.failed_items[-100:],
        }

    @workflow.run
    async def run(self, state: dict[str, Any] | None = None) -> None:
        """Main dispatcher loop."""
        self._running = True
        self._apply_state(state)
        reset_enabled = _safe_patched(CONTINUE_AS_NEW_PATCH_ID)

        while True:
            # Continue-As-New check: safe only when active is 0 and suggested by server.
            # Resetting history before dispatching carries pending queue forward into clean generation.
            if _should_continue_as_new(
                reset_enabled,
                _safe_is_continue_as_new_suggested(),
                len(self.active),
                _safe_all_handlers_finished(),
            ):
                logger.info(
                    "concurrency limiter continue-as-new queued=%s failed=%s",
                    len(self.queue),
                    len(self.failed_items),
                )
                _safe_continue_as_new(self.continuation_state())
                return

            self.dispatch_pending()

            # Idle handling: if queue and active are empty, wait for next enqueue
            if len(self.active) == 0 and len(self.queue) == 0:
                await _safe_wait_condition(lambda: len(self.queue) > 0)
                continue

            # Wait until any active task completes OR a queued item can be dispatched
            await _safe_wait_condition(
                lambda: (
                    any(entry.task.done() for entry in self.active.values())
                    or self._can_dispatch_any()
                )
            )

            done_tasks = {
                entry.task for entry in self.active.values() if entry.task.done()
            }
            if done_tasks:
                await self._handle_completed_tasks(done_tasks)
