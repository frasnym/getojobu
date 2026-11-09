"""LLM client for Step 1B via OmniRoute router.

Calls OmniRoute's OpenAI-compatible chat completions endpoint with
structured output (JSON Schema) so the model's response is guaranteed
to match the JobProfile schema. No manual json.loads() needed.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from openai import AsyncOpenAI

from models.job_profile import JobProfile
from usecase.analyze_job.service import LLMClient

_PROMPTS_DIR = Path(__file__).resolve().parents[2] / "prompts"


class OpenAIClient:
    """OmniRoute-backed LLM client (OpenAI-compatible API)."""

    def __init__(
        self,
        *,
        api_key: str,
        base_url: str,
        model: str,
    ) -> None:
        self._client: AsyncOpenAI = AsyncOpenAI(api_key=api_key, base_url=base_url)
        self._model = model

    async def extract_job_profile(
        self, *, job_description: str, prompt_version: str
    ) -> dict[str, Any]:
        prompt = self._render_prompt(prompt_version, job_description)

        # Structured output: JSON Schema guarantees the response matches JobProfile.
        response = await self._client.chat.completions.create(
            model=self._model,
            messages=[
                {"role": "system", "content": prompt},
                {
                    "role": "user",
                    "content": f"Job Description:\n{job_description}",
                },
            ],
            temperature=0,
            response_format={
                "type": "json_schema",
                "json_schema": {
                    "name": "job_profile",
                    "schema": JobProfile.model_json_schema(),
                    "strict": True,
                },
            },
        )

        content = response.choices[0].message.content
        if not content:
            raise RuntimeError("extract job profile: empty LLM response")

        # OmniRoute returns structured output as a JSON string.
        return json.loads(content)

    def _render_prompt(self, prompt_version: str, job_description: str) -> str:
        template_path = _PROMPTS_DIR / f"{prompt_version}.txt"
        if not template_path.exists():
            raise FileNotFoundError(f"prompt template not found: {template_path}")
        template = template_path.read_text(encoding="utf-8")
        if "{{job_description}}" not in template:
            raise ValueError(
                f"prompt template {prompt_version} missing {{job_description}}"
            )
        # Replace the placeholder so the JD is part of the system prompt,
        # matching the implementation reference's example prompt assembly.
        return template.replace("{{job_description}}", job_description)
