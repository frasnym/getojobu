"""Unit tests for the ConcurrencyLimiter workflow."""

import asyncio
from unittest.mock import MagicMock, patch

import pytest
from workflows.concurrency_limiter import (
    ConcurrencyLimiter,
    WorkItem,
)


@pytest.fixture
def limiter():
    return ConcurrencyLimiter()


def test_work_item_creation():
    item = WorkItem(params={"slug": "test-job"}, item_id="wf-1", job_type="scrape")
    assert item.item_id == "wf-1"
    assert item.job_type == "scrape"
    assert item.params == {"slug": "test-job"}


def test_enqueue_while_idle_goes_directly_to_active(limiter):
    mock_task = MagicMock(spec=asyncio.Task)
    limiter._create_child_task = MagicMock(return_value=mock_task)

    item = WorkItem(params={"slug": "x"}, item_id="wf-1", job_type="scrape")
    limiter.enqueue(item)

    assert len(limiter.queue) == 0
    assert len(limiter.active) == 1
    assert "wf-1" in limiter.active


def test_enqueue_while_at_capacity_queues(limiter):
    mock_task = MagicMock(spec=asyncio.Task)
    limiter._create_child_task = MagicMock(return_value=mock_task)

    # Scrape capacity is 1. Fill active.
    item1 = WorkItem(params={"slug": "job-1"}, item_id="wf-1", job_type="scrape")
    limiter.enqueue(item1)
    assert len(limiter.queue) == 0
    assert len(limiter.active) == 1

    # Second scrape item should be queued
    item2 = WorkItem(params={"slug": "job-2"}, item_id="wf-2", job_type="scrape")
    limiter.enqueue(item2)
    assert len(limiter.queue) == 1
    assert len(limiter.active) == 1
    assert limiter.queue[0].item_id == "wf-2"


def test_duplicate_enqueue_is_noop(limiter):
    mock_task = MagicMock(spec=asyncio.Task)
    limiter._create_child_task = MagicMock(return_value=mock_task)

    item1 = WorkItem(params={"slug": "job-1"}, item_id="wf-1", job_type="scrape")
    limiter.enqueue(item1)
    assert len(limiter.active) == 1

    # Duplicate enqueue of wf-1 while active
    limiter.enqueue(item1)
    assert len(limiter.active) == 1
    assert len(limiter.queue) == 0

    # Queue an item and test duplicate while queued
    item2 = WorkItem(params={"slug": "job-2"}, item_id="wf-2", job_type="scrape")
    limiter.enqueue(item2)
    assert len(limiter.queue) == 1

    limiter.enqueue(item2)
    assert len(limiter.queue) == 1


def test_queue_status_query(limiter):
    mock_task = MagicMock(spec=asyncio.Task)
    limiter._create_child_task = MagicMock(return_value=mock_task)

    limiter.enqueue(WorkItem(params={}, item_id="wf-1", job_type="scrape"))
    limiter.enqueue(WorkItem(params={}, item_id="wf-2", job_type="scrape"))

    status = limiter.queue_status()
    assert status["queued"] == 1
    assert status["active"] == ["wf-1"]
    assert status["capacity_by_type"] == {"scrape": 1, "analyze": 2}


@pytest.mark.asyncio
async def test_child_failure_logged_and_does_not_crash_limiter(limiter):
    # Simulate child task failing with an exception
    failed_task = asyncio.Future()
    failed_task.set_exception(RuntimeError("Child workflow execution failed"))

    limiter._create_child_task = MagicMock(return_value=failed_task)

    with patch("workflows.concurrency_limiter.logger") as mock_logger:
        item = WorkItem(params={}, item_id="wf-fail", job_type="scrape")
        limiter.enqueue(item)
        assert "wf-fail" in limiter.active

        # Process completion
        await limiter._handle_completed_tasks([failed_task])

        assert "wf-fail" not in limiter.active
        assert len(limiter.failed_items) == 1
        assert limiter.failed_items[0]["item_id"] == "wf-fail"
        mock_logger.error.assert_called_once()


def test_analyze_capacity_2(limiter):
    mock_task = MagicMock(spec=asyncio.Task)
    limiter._create_child_task = MagicMock(return_value=mock_task)

    limiter.enqueue(WorkItem(params={}, item_id="ana-1", job_type="analyze"))
    assert len(limiter.active) == 1
    assert len(limiter.queue) == 0

    limiter.enqueue(WorkItem(params={}, item_id="ana-2", job_type="analyze"))
    assert len(limiter.active) == 2
    assert len(limiter.queue) == 0

    # 3rd analyze should be queued because capacity is 2
    limiter.enqueue(WorkItem(params={}, item_id="ana-3", job_type="analyze"))
    assert len(limiter.active) == 2
    assert len(limiter.queue) == 1
    assert limiter.queue[0].item_id == "ana-3"


def test_hard_cap_mixed_concurrency(limiter):
    mock_task = MagicMock(spec=asyncio.Task)
    limiter._create_child_task = MagicMock(return_value=mock_task)

    # 1 scrape (cap=1)
    limiter.enqueue(WorkItem(params={}, item_id="sc-1", job_type="scrape"))
    # 2 analyze (cap=2)
    limiter.enqueue(WorkItem(params={}, item_id="an-1", job_type="analyze"))
    limiter.enqueue(WorkItem(params={}, item_id="an-2", job_type="analyze"))

    # Total active is 3 (1 scrape + 2 analyze)
    assert len(limiter.active) == 3
    assert len(limiter.queue) == 0

    # Additional of each type queues
    limiter.enqueue(WorkItem(params={}, item_id="sc-2", job_type="scrape"))
    limiter.enqueue(WorkItem(params={}, item_id="an-3", job_type="analyze"))
    assert len(limiter.active) == 3
    assert len(limiter.queue) == 2


@pytest.mark.asyncio
async def test_job_analyze_workflow_has_retry_policy():
    from datetime import timedelta

    from workflows.job_analyze import JobAnalyzeWorkflow

    wf = JobAnalyzeWorkflow()
    calls = []

    async def fake_execute_activity(activity, *args, **kwargs):
        calls.append((activity, kwargs))
        return {"status": "ok"}

    with patch(
        "temporalio.workflow.execute_activity", side_effect=fake_execute_activity
    ):
        await wf.run(
            {
                "job_description": "Go engineer",
                "job_id": "1",
                "slug": "go-eng",
            }
        )

    assert len(calls) == 1
    policy = calls[0][1].get("retry_policy")
    assert policy is not None
    assert policy.maximum_attempts == 3
    assert policy.initial_interval == timedelta(seconds=2)
    assert policy.backoff_coefficient == 2.0
    assert policy.maximum_interval == timedelta(seconds=30)


@pytest.mark.asyncio
async def test_limiter_run_wakes_up_from_idle_and_dispatches():
    limiter = ConcurrencyLimiter()
    executed = []

    async def fake_child(item: WorkItem):
        await asyncio.sleep(0.01)
        executed.append(item.item_id)
        return {"status": "ok"}

    limiter._create_child_task = lambda item: asyncio.create_task(fake_child(item))

    run_task = asyncio.create_task(limiter.run())

    # Give limiter a turn to enter idle wait
    await asyncio.sleep(0.01)
    assert len(limiter.active) == 0
    assert len(limiter.queue) == 0

    # Enqueue while idling
    limiter.enqueue(WorkItem(params={}, item_id="wf-idle-1", job_type="scrape"))

    # Give event loop time to wake up, dispatch, and complete
    await asyncio.sleep(0.05)

    assert "wf-idle-1" in executed

    run_task.cancel()
    try:
        await run_task
    except asyncio.CancelledError:
        pass


@pytest.mark.asyncio
async def test_limiter_run_handles_fast_task_while_slow_task_running():
    limiter = ConcurrencyLimiter()
    completed = []

    async def fake_child(item: WorkItem):
        if item.item_id == "slow-scrape":
            await asyncio.sleep(0.15)
        else:
            await asyncio.sleep(0.01)
        completed.append(item.item_id)

    limiter._create_child_task = lambda item: asyncio.create_task(fake_child(item))
    run_task = asyncio.create_task(limiter.run())

    # Start slow scrape
    limiter.enqueue(WorkItem(params={}, item_id="slow-scrape", job_type="scrape"))
    await asyncio.sleep(0.02)
    assert "slow-scrape" in limiter.active

    # Enqueue fast analyze while slow scrape is running
    limiter.enqueue(WorkItem(params={}, item_id="fast-analyze", job_type="analyze"))
    await asyncio.sleep(0.04)

    # Fast analyze must have completed BEFORE slow scrape completes
    assert "fast-analyze" in completed
    assert "slow-scrape" not in completed

    # Wait for slow scrape to finish
    await asyncio.sleep(0.15)
    assert "slow-scrape" in completed

    run_task.cancel()
    try:
        await run_task
    except asyncio.CancelledError:
        pass


@pytest.mark.asyncio
async def test_child_workflow_id_reuse_policy_is_allow_duplicate():
    from temporalio.common import WorkflowIDReusePolicy

    limiter = ConcurrencyLimiter()
    captured_kwargs = {}

    async def fake_start_child(name, *args, **kwargs):
        captured_kwargs.update(kwargs)
        future = asyncio.Future()
        future.set_result({"status": "ok"})
        return future

    with patch(
        "temporalio.workflow.start_child_workflow", side_effect=fake_start_child
    ):
        await limiter._run_child_workflow(
            WorkItem(
                params={"board": "glassdoor"},
                item_id="job-scrape-1",
                job_type="scrape",
            )
        )

    assert (
        captured_kwargs.get("id_reuse_policy") == WorkflowIDReusePolicy.ALLOW_DUPLICATE
    )


def test_should_continue_as_new_guards():
    from workflows.concurrency_limiter import _should_continue_as_new

    # All conditions met
    assert _should_continue_as_new(
        enabled=True, suggested=True, active_count=0, handlers_done=True
    )

    # Patch disabled (pre-patch history)
    assert not _should_continue_as_new(
        enabled=False, suggested=True, active_count=0, handlers_done=True
    )

    # Server hasn't suggested reset
    assert not _should_continue_as_new(
        enabled=True, suggested=False, active_count=0, handlers_done=True
    )

    # Active child in flight (cannot orphan child tasks)
    assert not _should_continue_as_new(
        enabled=True, suggested=True, active_count=1, handlers_done=True
    )

    # In-flight signal handler running
    assert not _should_continue_as_new(
        enabled=True, suggested=True, active_count=0, handlers_done=False
    )


def test_limiter_apply_state_and_continuation_state():
    limiter = ConcurrencyLimiter()
    initial_state = {
        "queue": [
            {"params": {"slug": "job-1"}, "item_id": "wf-1", "job_type": "scrape"},
            {"params": {"slug": "job-2"}, "item_id": "wf-2", "job_type": "analyze"},
        ],
        "failed_items": [{"item_id": "wf-old-fail", "job_type": "scrape", "error": "timeout"}],
    }

    limiter._apply_state(initial_state)

    assert len(limiter.queue) == 2
    assert limiter.queue[0].item_id == "wf-1"
    assert limiter.queue[0].job_type == "scrape"
    assert limiter.queue[1].item_id == "wf-2"
    assert limiter.queue[1].job_type == "analyze"
    assert len(limiter.failed_items) == 1
    assert limiter.failed_items[0]["item_id"] == "wf-old-fail"

    snapshot = limiter.continuation_state()
    assert snapshot["queue"] == initial_state["queue"]
    assert snapshot["failed_items"] == initial_state["failed_items"]


@pytest.mark.asyncio
async def test_limiter_run_triggers_continue_as_new_when_suggested():
    limiter = ConcurrencyLimiter()
    continue_as_new_calls = []

    def fake_continue_as_new(state):
        continue_as_new_calls.append(state)

    with (
        patch("workflows.concurrency_limiter._safe_patched", return_value=True),
        patch(
            "workflows.concurrency_limiter._safe_is_continue_as_new_suggested",
            return_value=True,
        ),
        patch(
            "workflows.concurrency_limiter._safe_all_handlers_finished",
            return_value=True,
        ),
        patch(
            "workflows.concurrency_limiter._safe_continue_as_new",
            side_effect=fake_continue_as_new,
        ),
    ):
        initial_state = {
            "queue": [
                {"params": {"k": "v"}, "item_id": "pending-1", "job_type": "scrape"}
            ],
            "failed_items": [],
        }
        await limiter.run(initial_state)

    assert len(continue_as_new_calls) == 1
    saved_state = continue_as_new_calls[0]
    # Pending items were preserved and passed to the next generation
    assert any(item["item_id"] == "pending-1" for item in saved_state["queue"])

