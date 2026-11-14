"""LLM client for Step 4 Match Report Generation via OmniRoute router."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from openai import AsyncOpenAI

from models.report import MatchReportOutput
from utils.json import parse_llm_json

_PROMPTS_DIR = Path(__file__).resolve().parents[2] / "prompts"


class OpenAIReportClient:
    """OmniRoute-backed LLM client for generating match reports."""

    def __init__(
        self,
        *,
        api_key: str,
        base_url: str,
        model: str,
    ) -> None:
        self._client: AsyncOpenAI = AsyncOpenAI(api_key=api_key, base_url=base_url)
        self._model = model

    async def generate_match_report(
        self,
        *,
        candidate_profile: dict[str, Any],
        job_profile: dict[str, Any],
        evaluation_results: dict[str, Any],
        overall_score: float,
        eligibility_status: str,
        recommendation_label: str,
        prompt_version: str = "generate-report-v1",
    ) -> dict[str, Any]:
        prompt = self._render_prompt(
            prompt_version,
            candidate_profile=candidate_profile,
            job_profile=job_profile,
            evaluation_results=evaluation_results,
            overall_score=overall_score,
            eligibility_status=eligibility_status,
            recommendation_label=recommendation_label,
        )

        response = await self._client.chat.completions.create(
            model=self._model,
            messages=[
                {"role": "system", "content": prompt},
                {
                    "role": "user",
                    "content": "Generate the match report based on the candidate profile, job profile, evaluation results, and authoritative facts provided.",
                },
            ],
            temperature=0,
            response_format={
                "type": "json_schema",
                "json_schema": {
                    "name": "match_report",
                    "schema": MatchReportOutput.model_json_schema(),
                    "strict": True,
                },
            },
        )

        content = response.choices[0].message.content
        if not content:
            raise RuntimeError("generate match report: empty LLM response")

        return parse_llm_json(content)

    def _render_prompt(
        self,
        prompt_version: str,
        *,
        candidate_profile: dict[str, Any],
        job_profile: dict[str, Any],
        evaluation_results: dict[str, Any],
        overall_score: float,
        eligibility_status: str,
        recommendation_label: str,
    ) -> str:
        template_path = _PROMPTS_DIR / f"{prompt_version}.txt"
        if not template_path.exists():
            raise FileNotFoundError(f"prompt template not found: {template_path}")
        template = template_path.read_text(encoding="utf-8")

        placeholders = {
            "{{candidate_profile}}": json.dumps(candidate_profile, indent=2),
            "{{job_profile}}": json.dumps(job_profile, indent=2),
            "{{evaluation_results}}": json.dumps(evaluation_results, indent=2),
            "{{overall_score}}": str(overall_score),
            "{{eligibility_status}}": eligibility_status,
            "{{recommendation_label}}": recommendation_label,
        }

        rendered = template
        for placeholder, val in placeholders.items():
            if placeholder not in rendered:
                raise ValueError(
                    f"prompt template {prompt_version} missing placeholder {placeholder}"
                )
            rendered = rendered.replace(placeholder, val)

        return rendered
