"""Two-pass resume generation and deterministic validation."""

from __future__ import annotations

import json
from typing import Any, Protocol

from models.resume import ResumeCritique, ResumeDraft, validate_resume_draft
from usecase.generate_resume.content import build_resume_content
from usecase.generate_resume.renderer import render_html, render_pdf


class ResumeLLMClient(Protocol):
    async def generate_resume(self, **content: str) -> dict[str, Any]: ...

    async def critique_resume(
        self, *, resume_draft: dict[str, Any], **content: str
    ) -> dict[str, Any]: ...


class GenerateResume:
    def __init__(
        self,
        llm: ResumeLLMClient | None = None,
        *,
        prompt_version: str = "generate-resume-v1",
        critique_version: str = "critique-resume-v1",
    ) -> None:
        self._llm = llm
        self._prompt_version = prompt_version
        self._critique_version = critique_version

    async def generate(self, **kwargs: Any) -> dict[str, Any]:
        template_name: str | None = kwargs.pop("template_name", None)
        facts = kwargs["facts"]
        evidence = kwargs["evaluation_results"]
        content = build_resume_content(**kwargs)
        if self._llm is None:
            raise RuntimeError("llm client is required for resume generation")
        raw_draft = await self._llm.generate_resume(
            **content, prompt_version=self._prompt_version
        )
        draft = ResumeDraft.model_validate(raw_draft)
        validate_resume_draft(draft, facts=facts, match_evidence=evidence)
        critique = ResumeCritique.model_validate(
            await self._llm.critique_resume(
                **content,
                resume_draft=draft.model_dump(),
                prompt_version=self._critique_version,
            )
        )
        if not critique.passed:
            raw_draft = await self._llm.generate_resume(
                **content,
                critique_issues=json.dumps(critique.issues),
                previous_draft=json.dumps(draft.model_dump(), sort_keys=True),
                prompt_version=self._prompt_version,
            )
            draft = ResumeDraft.model_validate(raw_draft)
            validate_resume_draft(draft, facts=facts, match_evidence=evidence)
            critique = ResumeCritique.model_validate(
                await self._llm.critique_resume(
                    **content,
                    resume_draft=draft.model_dump(),
                    prompt_version=self._critique_version,
                )
            )
            if not critique.passed:
                raise ValueError("resume critique failed: " + "; ".join(critique.issues))
        return {
            "resume": draft.model_dump(),
            "critique": critique.model_dump(),
            "html": render_html(draft, template_name=template_name, facts=facts),
            "pdf": render_pdf(draft, template_name=template_name, facts=facts),
            "template": template_name or "hermes",
            "prompt_version": self._prompt_version,
            "critique_version": self._critique_version,
        }
