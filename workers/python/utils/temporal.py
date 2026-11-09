"""Temporal client helper for the worker.

Mirrors the Go API's temporal client: a lazy/singleton client used to start
and signal workflows. The worker itself uses the temporalio Worker API.
"""

from __future__ import annotations

from typing import Any

from temporalio.client import Client, WorkflowHandle


class TemporalClient:
    """Thin wrapper around the temporalio SDK client."""

    def __init__(self, client: Client) -> None:
        self.client = client

    async def start_workflow(
        self,
        workflow: type[Any],
        input: dict[str, Any],
        *,
        id: str,
        task_queue: str,
    ) -> WorkflowHandle[Any, Any]:
        """Start a workflow and return its handle (not the result)."""
        return await self.client.start_workflow(
            workflow.run,
            input,
            id=id,
            task_queue=task_queue,
        )

    async def get_handle(
        self,
        workflow_id: str,
        run_id: str | None = None,
    ) -> WorkflowHandle[Any, Any]:
        """Get a handle to an existing workflow run."""
        return self.client.get_workflow_handle(workflow_id, run_id=run_id)


async def connect(host: str, namespace: str) -> TemporalClient:
    """Connect to Temporal and return a wrapped client."""
    client = await Client.connect(host, namespace=namespace)
    return TemporalClient(client=client)
