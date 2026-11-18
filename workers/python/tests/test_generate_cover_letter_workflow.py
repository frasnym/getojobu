"""Tests for GenerateCoverLetterWorkflow, its activities, and limiter mapping."""

from __future__ import annotations

import json
from typing import Any
from unittest.mock import AsyncMock, MagicMock

import httpx
import pytest
from temporalio import activity

from repository.getojobu_api import NewGetojobuAPI
from workflows.concurrency_limiter import LIMITS, QUEUE_BY_TYPE, WORKFLOW_BY_TYPE

FIXED_FAILURE_MESSAGE = "Cover letter generation failed"


def _content(variant: str = "short") -> dict[str, Any]:
    return {
        "text": "Dear Hiring Manager, I am excited to apply. Sincerely, Alex",
        "metadata": {
            "model": "use-combo",
            "prompt_version": f"generate-cover-letter-{variant}-v1",
        },
    }


def _input(variant: str = "short") -> dict[str, Any]:
    """Mirror the exact workflow input keys sent by the Go usecase."""
    return {
        "evaluation_id": "eval-123",
        "job_id": 999,
        "user_id": "user-456",
        "candidate_profile": {"skills": ["go", "postgres"]},
        "facts": {"name": "Alex", "education": "BSc Computer Science"},
        "job_profile": {"title": "Backend Engineer"},
        "evaluation_results": {"strengths": ["distributed systems"]},
        "overall_score": 88.0,
        "eligibility_status": "eligible",
        "recommendation_label": "worth_applying",
        "variant": variant,
    }


async def _run_workflow(
    *,
    generate_impl: Any,
    wf_input: dict[str, Any],
    wf_id: str,
) -> tuple[dict[str, Any], list[dict[str, Any]], list[dict[str, Any]], list[Any]]:
    """Run the workflow in an ephemeral Temporal env with fake activities."""
    from temporalio.testing import WorkflowEnvironment
    from temporalio.worker import Worker

    from workflows.generate_cover_letter import GenerateCoverLetterWorkflow

    try:
        env = await WorkflowEnvironment.start_time_skipping()
    except Exception as exc:  # noqa: BLE001
        pytest.skip(f"Ephemeral temporal server unavailable: {exc}")

    status_updates: list[dict[str, Any]] = []
    generate_inputs: list[dict[str, Any]] = []
    upload_calls: list[Any] = []

    async with env:
        task_queue = "test-generate-cover-letter-queue"

        @activity.defn(name="generate_cover_letter_activity")
        async def fake_generate(input: dict[str, Any]) -> dict[str, Any]:
            generate_inputs.append(input)
            return await generate_impl(input)

        @activity.defn(name="update_cover_letter_status_activity")
        async def fake_update_status(input: dict[str, Any]) -> dict[str, Any]:
            status_updates.append(dict(input))
            return {"cover_letter_status": input.get("cover_letter_status")}

        @activity.defn(name="upload_cover_letter_activity")
        async def fake_upload(input: dict[str, Any]) -> dict[str, Any]:
            upload_calls.append(input)
            return {}

        async with Worker(
            env.client,
            task_queue=task_queue,
            workflows=[GenerateCoverLetterWorkflow],
            activities=[fake_generate, fake_update_status, fake_upload],
        ):
            handle = await env.client.start_workflow(
                GenerateCoverLetterWorkflow.run,
                wf_input,
                id=wf_id,
                task_queue=task_queue,
            )
            result: dict[str, Any] = await handle.result()

    return result, status_updates, generate_inputs, upload_calls


@pytest.mark.asyncio
async def test_workflow_success_writes_content() -> None:
    expected = _content()

    async def generate_impl(input: dict[str, Any]) -> dict[str, Any]:
        return expected

    result, status_updates, _generate_inputs, upload_calls = await _run_workflow(
        generate_impl=generate_impl,
        wf_input=_input(),
        wf_id="test-cover-letter-success",
    )

    assert result["workflow"] == "GenerateCoverLetterWorkflow"
    assert result["evaluation_id"] == "eval-123"
    assert result["cover_letter_status"] == "success"
    assert result["cover_letter"] == expected
    assert [s["cover_letter_status"] for s in status_updates] == ["pending", "success"]

    success_payload = status_updates[-1]
    content = success_payload["cover_letter_content"]
    assert content["text"] == expected["text"]
    assert content["metadata"] == {
        "model": "use-combo",
        "prompt_version": "generate-cover-letter-short-v1",
    }
    assert "cover_letter_failure_message" not in success_payload
    # No R2/upload activity is part of the cover-letter pipeline.
    assert upload_calls == []


@pytest.mark.asyncio
async def test_workflow_failure_uses_fixed_message() -> None:
    async def generate_impl(input: dict[str, Any]) -> dict[str, Any]:
        raise RuntimeError(
            "leaked CV: Jane Doe, BSc Computer Science, secret prompt body"
        )

    result, status_updates, _generate_inputs, upload_calls = await _run_workflow(
        generate_impl=generate_impl,
        wf_input=_input(),
        wf_id="test-cover-letter-failure",
    )

    assert result["cover_letter_status"] == "failed"
    fail_payload = status_updates[-1]
    assert fail_payload["cover_letter_status"] == "failed"
    assert fail_payload["cover_letter_failure_message"] == FIXED_FAILURE_MESSAGE
    # A failed regeneration must not clear the previously stored letter.
    assert "cover_letter_content" not in fail_payload
    assert "Jane Doe" not in str(status_updates)
    assert "secret prompt" not in str(status_updates)
    assert upload_calls == []


@pytest.mark.asyncio
async def test_workflow_passes_variant_to_activity() -> None:
    async def generate_impl(input: dict[str, Any]) -> dict[str, Any]:
        return _content(variant=input["variant"])

    _result, _status_updates, generate_inputs, _upload_calls = await _run_workflow(
        generate_impl=generate_impl,
        wf_input=_input(variant="full"),
        wf_id="test-cover-letter-variant",
    )

    assert generate_inputs[0]["variant"] == "full"


def test_limiter_maps_cover_letter() -> None:
    assert WORKFLOW_BY_TYPE["cover_letter"] == "GenerateCoverLetterWorkflow"
    assert QUEUE_BY_TYPE["cover_letter"] == "job-evaluation"
    assert LIMITS["cover_letter"] == 2


@pytest.mark.asyncio
async def test_generate_activity_validates_and_calls_usecase() -> None:
    from deliveries.consumer.activities.cover_letter_generation import (
        generate_cover_letter_activity,
        init_cover_letter_generation_container,
    )

    container = MagicMock()
    expected = _content()
    container.usecase.generate_cover_letter.generate = AsyncMock(return_value=expected)
    init_cover_letter_generation_container(container)

    data = _input(variant="full")
    result = await generate_cover_letter_activity(data)
    assert result == expected

    kwargs = container.usecase.generate_cover_letter.generate.await_args.kwargs
    assert kwargs["variant"] == "full"
    assert kwargs["candidate_profile"] == data["candidate_profile"]
    assert kwargs["job_profile"] == data["job_profile"]
    assert kwargs["evaluation_results"] == data["evaluation_results"]
    assert kwargs["facts"] == data["facts"]
    assert kwargs["overall_score"] == 88.0
    assert kwargs["eligibility_status"] == "eligible"
    assert kwargs["recommendation_label"] == "worth_applying"


@pytest.mark.asyncio
async def test_generate_activity_requires_variant() -> None:
    from deliveries.consumer.activities.cover_letter_generation import (
        generate_cover_letter_activity,
    )

    data = _input()
    del data["variant"]
    with pytest.raises(ValueError, match="variant is required"):
        await generate_cover_letter_activity(data)


@pytest.mark.asyncio
async def test_update_activity_omits_content_on_failure() -> None:
    from deliveries.consumer.activities.cover_letter_generation import (
        init_cover_letter_generation_container,
        update_cover_letter_status_activity,
    )

    container = MagicMock()
    container.repository.getojobu_api.patch_matching_evaluation_cover_letter = (
        AsyncMock(return_value={})
    )
    init_cover_letter_generation_container(container)

    await update_cover_letter_status_activity(
        {
            "evaluation_id": "eval-123",
            "cover_letter_status": "failed",
            "cover_letter_failure_message": FIXED_FAILURE_MESSAGE,
        }
    )

    patch_mock = (
        container.repository.getojobu_api.patch_matching_evaluation_cover_letter
    )
    kwargs = patch_mock.await_args.kwargs
    assert kwargs["evaluation_id"] == "eval-123"
    assert kwargs["cover_letter_status"] == "failed"
    assert kwargs["cover_letter_failure_message"] == FIXED_FAILURE_MESSAGE
    assert kwargs["cover_letter_content"] is None


@pytest.mark.asyncio
async def test_patch_matching_evaluation_cover_letter_sends_payload() -> None:
    eval_id = "eval-123"
    expected_content = _content()

    def handler(request: httpx.Request) -> httpx.Response:
        assert request.method == "PATCH"
        assert request.url.path == f"/v1/matching-evaluations/{eval_id}/cover-letter"
        body = json.loads(request.content.decode("utf-8"))
        assert body["cover_letter_status"] == "success"
        assert body["cover_letter_content"] == expected_content
        assert "cover_letter_failure_message" not in body
        return httpx.Response(
            200,
            json={
                "success": True,
                "data": {
                    "id": eval_id,
                    "cover_letter_status": "success",
                },
                "error": None,
            },
        )

    api = NewGetojobuAPI(base_url="http://testserver", api_key="secret-key")
    api._client = httpx.AsyncClient(
        transport=httpx.MockTransport(handler),
        base_url="http://testserver",
        headers={
            "Authorization": "Bearer secret-key",
            "Content-Type": "application/json",
        },
    )

    res = await api.patch_matching_evaluation_cover_letter(
        eval_id,
        cover_letter_status="success",
        cover_letter_content=expected_content,
    )
    assert res["id"] == eval_id
    assert res["cover_letter_status"] == "success"
    await api.aclose()
