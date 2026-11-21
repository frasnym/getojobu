"""Getojobu-owned deterministic resume renderer supporting multiple templates."""

from __future__ import annotations

import os
import sys
from typing import Any, Protocol, cast

from models.resume import ResumeDraft
from usecase.generate_resume.templates import (
    DEFAULT_TEMPLATE,
    get_template,
    list_templates,
)

MAX_RESUME_CHARS = 2900


class _PDFDocument(Protocol):
    def write_pdf(self) -> bytes | None: ...


def render_html(
    draft: ResumeDraft,
    template_name: str | None = None,
    facts: dict[str, Any] | None = None,
) -> str:
    """Render a ResumeDraft to an HTML string using the chosen template."""
    renderer = get_template(template_name)
    return renderer(draft, facts)


def count_resume_chars(draft: ResumeDraft) -> int:
    """Count the characters of the resume text that is actually rendered."""
    parts: list[str] = [draft.headline, draft.education, draft.contact, *draft.skills]
    for role in draft.jobs.values():
        if not role.selected:
            continue
        parts.extend(value for value in (role.company, role.role, role.period) if value)
        parts.extend(role.bullets)
    return sum(len(part) for part in parts)


def fit_resume_to_char_limit(
    draft: ResumeDraft, limit: int = MAX_RESUME_CHARS
) -> ResumeDraft:
    """Trim the least relevant content until the resume is within the char limit."""
    fitted = draft.model_copy(deep=True)
    if count_resume_chars(fitted) <= limit:
        return fitted

    selected_ids = [rid for rid, role in fitted.jobs.items() if role.selected]
    for index in range(len(selected_ids) - 1, -1, -1):
        role = fitted.jobs[selected_ids[index]]
        min_bullets = 1 if index == 0 else 0
        while len(role.bullets) > min_bullets and count_resume_chars(fitted) > limit:
            del role.bullets[-1]
        if count_resume_chars(fitted) <= limit:
            return fitted
        if index == 0:
            break
        role.selected = False
        role.bullets = []

    while len(fitted.skills) > 1 and count_resume_chars(fitted) > limit:
        del fitted.skills[-1]
    if count_resume_chars(fitted) > limit:
        raise ValueError(f"resume exceeds {limit} characters")
    return fitted


def render_pdf(
    draft: ResumeDraft,
    template_name: str | None = None,
    facts: dict[str, Any] | None = None,
) -> bytes:
    """Render a PDF when WeasyPrint is installed, without importing it at startup."""
    if sys.platform == "darwin":
        cur_fallback = os.environ.get("DYLD_FALLBACK_LIBRARY_PATH", "")
        if "/opt/homebrew/lib" not in cur_fallback:
            os.environ["DYLD_FALLBACK_LIBRARY_PATH"] = (
                f"/opt/homebrew/lib:{cur_fallback}".rstrip(":")
            )
    try:
        from weasyprint import HTML
    except (ImportError, OSError) as exc:
        raise RuntimeError("WeasyPrint is required for PDF resume rendering") from exc
    html_content = render_html(draft, template_name=template_name, facts=facts)
    doc = cast(_PDFDocument, HTML(string=html_content))
    pdf_bytes = doc.write_pdf()
    if not isinstance(pdf_bytes, bytes):
        raise TypeError("WeasyPrint failed to produce PDF bytes")
    return pdf_bytes


__all__ = [
    "DEFAULT_TEMPLATE",
    "MAX_RESUME_CHARS",
    "count_resume_chars",
    "fit_resume_to_char_limit",
    "list_templates",
    "render_html",
    "render_pdf",
]
