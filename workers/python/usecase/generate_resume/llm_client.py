"""OmniRoute client for the two resume-generation prompt passes."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any, cast

from openai import AsyncOpenAI

_PROMPTS_DIR = Path(__file__).resolve().parents[2] / "prompts"


class OpenAIResumeClient:
    def __init__(self, *, api_key: str, base_url: str, model: str) -> None:
        self._client = AsyncOpenAI(api_key=api_key, base_url=base_url)
        self._model = model

    async def generate_resume(
        self, *, prompt_version: str, **content: str
    ) -> dict[str, Any]:
        values = dict(content)
        feedback_str = ""
        if "critique_issues" in values:
            raw_issues = values.pop("critique_issues")
            try:
                parsed = json.loads(raw_issues)
                if isinstance(parsed, list) and parsed:
                    issues: list[object] = cast(list[object], parsed)
                    feedback_str = (
                        "# Critique feedback from previous draft (all issues must be fixed):\n"
                        + "\n".join(f"- {issue}" for issue in issues)
                    )
            except (json.JSONDecodeError, TypeError):
                feedback_str = f"# Critique feedback from previous draft:\n{raw_issues}"
        elif "critique_feedback" in values:
            feedback_str = values.pop("critique_feedback")

        values["critique_feedback"] = feedback_str
        return await self._json_call(prompt_version, values, "resume_draft")

    async def critique_resume(
        self, *, prompt_version: str, resume_draft: dict[str, Any], **content: str
    ) -> dict[str, Any]:
        return await self._json_call(
            prompt_version,
            {**content, "resume_draft": json.dumps(resume_draft, indent=2)},
            "resume_critique",
        )

    async def _json_call(
        self, prompt_version: str, values: dict[str, str], schema_name: str
    ) -> dict[str, Any]:
        template_path = _PROMPTS_DIR / f"{prompt_version}.txt"
        if not template_path.exists():
            raise FileNotFoundError(f"prompt template not found: {template_path}")
        template = template_path.read_text(encoding="utf-8")
        rendered = template
        for key, value in values.items():
            placeholder = "{{" + key + "}}"
            if placeholder in rendered:
                rendered = rendered.replace(placeholder, value)

        rendered = rendered.replace("{{critique_feedback}}", "")

        response = await self._client.chat.completions.create(
            model=self._model,
            messages=[
                {"role": "system", "content": rendered},
                {
                    "role": "user",
                    "content": "Generate the output strictly following the schema and instructions provided.",
                },
            ],
            temperature=0,
            response_format={"type": "json_object"},
        )
        content = response.choices[0].message.content
        if not content:
            raise RuntimeError(f"{schema_name}: empty LLM response")
        parsed = json.loads(content)
        if not isinstance(parsed, dict):
            raise TypeError(f"{schema_name}: expected dict, got {type(parsed)}")
        return cast(dict[str, Any], parsed)
