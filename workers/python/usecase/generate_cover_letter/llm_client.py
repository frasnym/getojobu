"""OmniRoute client for cover-letter generation (plain-text output)."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from openai import AsyncOpenAI

from utils.llm_errors import LLMError
from utils.prompt_sanitizer import (
    sanitize_evaluation_results,
    sanitize_job_profile,
)

_PROMPTS_DIR = Path(__file__).resolve().parents[2] / "prompts"


class OpenAICoverLetterClient:
    def __init__(self, *, api_key: str, base_url: str, model: str) -> None:
        self._client = AsyncOpenAI(api_key=api_key, base_url=base_url)
        self._model = model

    async def generate_cover_letter(
        self, *, prompt_version: str, **content: str
    ) -> str:
        prompt = self._render_prompt(prompt_version, **content)

        try:
            response = await self._client.chat.completions.create(
                model=self._model,
                messages=[
                    {"role": "system", "content": prompt},
                    {
                        "role": "user",
                        "content": "Write the cover letter now using only the supplied data.",
                    },
                ],
                temperature=0,
            )
            text = response.choices[0].message.content
            if not text:
                raise RuntimeError("cover letter generation: empty LLM response")
            return text
        except LLMError:
            raise
        except Exception as exc:
            raise LLMError(
                "LLM error during cover letter generation", detail=str(exc)
            ) from exc

    def _render_prompt(self, prompt_version: str, **content: Any) -> str:
        template_path = _PROMPTS_DIR / f"{prompt_version}.txt"
        if not template_path.exists():
            raise FileNotFoundError(f"prompt template not found: {template_path}")
        template = template_path.read_text(encoding="utf-8")

        values = dict(content)
        if isinstance(values.get("job_profile"), dict):
            values["job_profile"] = sanitize_job_profile(values["job_profile"])
        if isinstance(values.get("match_evidence"), dict):
            values["match_evidence"] = sanitize_evaluation_results(
                values["match_evidence"]
            )

        rendered = template
        for key, value in values.items():
            placeholder = "{{" + key + "}}"
            if placeholder not in rendered:
                continue
            serialized = (
                json.dumps(value, indent=2, sort_keys=True)
                if isinstance(value, (dict, list))
                else str(value)
            )
            rendered = rendered.replace(placeholder, serialized)

        return rendered
