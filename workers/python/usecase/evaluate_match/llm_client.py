"""LLM client for Step 2 Compatibility Evaluation via OmniRoute router.

Calls OmniRoute's OpenAI-compatible chat completions endpoint with structured
output (JSON Schema) matching MatchEvaluationOutput schema.
"""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from models.evaluation import MatchEvaluationOutput
from openai import AsyncOpenAI
from utils.json import parse_llm_json

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

    async def extract_match_evaluation(
        self,
        *,
        candidate_profile: dict[str, Any],
        job_profile: dict[str, Any],
        evaluation_config: dict[str, Any],
        prompt_version: str = "evaluate-match-v1",
    ) -> dict[str, Any]:
        prompt = self._render_prompt(
            prompt_version,
            candidate_profile=candidate_profile,
            job_profile=job_profile,
            evaluation_config=evaluation_config,
        )

        categories = evaluation_config.get("categories", [])
        category_names = [
            c["name"]
            for c in categories
            if isinstance(c, dict) and "name" in c and isinstance(c["name"], str)
        ]

        if category_names:
            cats_str = ", ".join(f"'{name}'" for name in category_names)
            user_content = (
                f"Evaluate the candidate profile against the job profile and evaluation configuration according to your instructions. "
                f"You MUST include an evaluation for each of the following {len(category_names)} categories: {cats_str}. "
                f"Do not omit any category."
            )
        else:
            user_content = "Evaluate the candidate profile against the job profile and evaluation configuration according to your instructions."

        schema = MatchEvaluationOutput.model_json_schema()
        if (
            category_names
            and "$defs" in schema
            and "CategoryEvaluation" in schema["$defs"]
        ):
            cat_props = schema["$defs"]["CategoryEvaluation"].get("properties", {})
            if "category" in cat_props:
                cat_props["category"]["enum"] = category_names

        response = await self._client.chat.completions.create(
            model=self._model,
            messages=[
                {"role": "system", "content": prompt},
                {
                    "role": "user",
                    "content": user_content,
                },
            ],
            temperature=0,
            response_format={
                "type": "json_schema",
                "json_schema": {
                    "name": "match_evaluation",
                    "schema": schema,
                    "strict": True,
                },
            },
        )

        content = response.choices[0].message.content
        if not content:
            raise RuntimeError("extract match evaluation: empty LLM response")

        return parse_llm_json(content)

    def _render_prompt(
        self,
        prompt_version: str,
        *,
        candidate_profile: dict[str, Any],
        job_profile: dict[str, Any],
        evaluation_config: dict[str, Any],
    ) -> str:
        template_path = _PROMPTS_DIR / f"{prompt_version}.txt"
        if not template_path.exists():
            raise FileNotFoundError(f"prompt template not found: {template_path}")
        template = template_path.read_text(encoding="utf-8")

        for _, placeholder in [
            ("candidate_profile", "{{candidate_profile}}"),
            ("job_profile", "{{job_profile}}"),
            ("evaluation_config", "{{evaluation_config}}"),
        ]:
            if placeholder not in template:
                raise ValueError(
                    f"prompt template {prompt_version} missing {placeholder}"
                )

        rendered = template.replace(
            "{{candidate_profile}}", json.dumps(candidate_profile, indent=2)
        )
        rendered = rendered.replace(
            "{{job_profile}}", json.dumps(job_profile, indent=2)
        )
        rendered = rendered.replace(
            "{{evaluation_config}}", json.dumps(evaluation_config, indent=2)
        )
        return rendered
