"""OpenAI LLM client for Step 1B.

Implements the LLMClient protocol used by the analyze_job usecase.
Renders the extract-job-v1 prompt template and parses the JSON response.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from openai import AsyncOpenAI

from usecase.analyze_job.service import LLMClient

_PROMPTS_DIR = Path(__file__).resolve().parents[2] / "prompts"


class OpenAIClient:
    def __init__(self, *, api_key: str, model: str) -> None:
        self._client = AsyncOpenAI(api_key=api_key)
        self._model = model

    async def extract_job_profile(
        self, *, job_description: str, prompt_version: str
    ) -> dict[str, Any]:
        prompt = self._render_prompt(prompt_version, job_description)
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
            response_format={"type": "json_object"},
        )
        content = response.choices[0].message.content
        if not content:
            raise RuntimeError("extract job profile: empty LLM response")
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
