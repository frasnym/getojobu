"""Business logic for Step 1B: Analyze Job Description.

Mirrors apps/backend/internal/usecase/users/users.go: a thin service that
validates input, delegates to an LLM, and persists via the Go API.

This is the "actual flow" the user asked to pick: Step 1B.
"""

from __future__ import annotations

import uuid
from typing import Any, Protocol, runtime_checkable

from models.job_profile import JobProfile
from deliveries.client.api_client import APIClient


class LLMClient(Protocol):
    """Protocol for an LLM client used by Step 1B."""

    async def extract_job_profile(
        self, *, job_description: str, prompt_version: str
    ) -> dict[str, Any]: ...


class AnalyzeJob:
    def __init__(
        self,
        api_client: APIClient,
        llm: LLMClient,
        *,
        prompt_version: str = "extract-job-v1",
        provider: str = "openai",
        model: str = "gpt-4o-mini",
    ) -> None:
        self._api_client = api_client
        self._llm = llm
        self._prompt_version = prompt_version
        self._provider = provider
        self._model = model

    async def execute(
        self,
        *,
        job_description: str,
        job_id: str | None = None,
        idempotency_key: str | None = None,
    ) -> JobProfile:
        """Run Step 1B end to end.

        1. Validate the JD input is non-empty.
        2. Call the LLM with the extract-job-v1 prompt to obtain a
           normalized job profile.
        3. Parse and validate the LLM output into JobProfile.
        4. Persist via the Go API (internal Docker network).
        5. Return the stored profile.
        """
        if not job_description or not job_description.strip():
            raise ValueError("job_description is required")

        # 2. Ask the LLM to extract and normalize the job description.
        raw = await self._llm.extract_job_profile(
            job_description=job_description,
            prompt_version=self._prompt_version,
        )

        # 3. Parse into the domain model (validates family, seniority, etc.).
        profile = JobProfile.model_validate(raw)

        # Override metadata fields the caller should control.
        if job_id is not None:
            profile.job_id = job_id
        profile.provider = self._provider
        profile.model = self._model
        profile.prompt_version = self._prompt_version

        # 4. Persist via the Go API (not direct DB access).
        key = idempotency_key or str(uuid.uuid4())
        stored = await self._api_client.create_job_profile(
            job_description=job_description,
            job_id=job_id,
            profile=profile.model_dump(),
            idempotency_key=key,
        )

        # 5. Return the stored profile as returned by the API.
        return JobProfile.model_validate(stored)
