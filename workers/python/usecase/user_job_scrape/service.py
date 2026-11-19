"""LLM extraction of job fields from a user-supplied job page."""

from __future__ import annotations

from pathlib import Path
from typing import Any

from models.user_job_scrape import ScrapedJobFields
from openai import AsyncOpenAI
from utils.json import parse_llm_json
from utils.llm_errors import LLMError

_PROMPTS_DIR = Path(__file__).resolve().parents[2] / "prompts"
_PROMPT_VERSION = "extract-job-url-v1"
_MAX_PAGE_CHARS = 12000


class ScrapeLLMClient:
    """OmniRoute-backed client for structured job-page extraction."""

    def __init__(self, *, api_key: str, base_url: str, model: str) -> None:
        self._client = AsyncOpenAI(api_key=api_key, base_url=base_url)
        self._model = model

    async def extract(self, page_text: str) -> dict[str, Any]:
        template = (_PROMPTS_DIR / f"{_PROMPT_VERSION}.txt").read_text(encoding="utf-8")
        prompt = template.replace("{{page_text}}", page_text[:_MAX_PAGE_CHARS])
        try:
            response = await self._client.chat.completions.create(
                model=self._model,
                messages=[
                    {"role": "system", "content": prompt},
                    {"role": "user", "content": "Extract the job fields as JSON."},
                ],
                temperature=0,
                response_format={
                    "type": "json_schema",
                    "json_schema": {
                        "name": "scraped_job_fields",
                        "schema": ScrapedJobFields.model_json_schema(),
                        "strict": True,
                    },
                },
            )
            content = response.choices[0].message.content
            if not content:
                raise RuntimeError("extract job URL: empty LLM response")
            parsed = parse_llm_json(content)
            return ScrapedJobFields.model_validate(parsed).model_dump()
        except LLMError:
            raise
        except Exception as exc:  # noqa: BLE001
            raise LLMError(
                "LLM error during job URL extraction", detail=str(exc)
            ) from exc


class UserJobScrape:
    """Service coordinating URL extraction for user-supplied jobs."""

    def __init__(self, llm: ScrapeLLMClient) -> None:
        self._llm = llm

    async def extract(self, page_text: str) -> dict[str, Any]:
        return await self._llm.extract(page_text)
