"""Getojobu-owned deterministic resume renderer supporting multiple templates."""

from __future__ import annotations

from typing import Any, Protocol, cast

from models.resume import ResumeDraft
from usecase.generate_resume.templates import (
    DEFAULT_TEMPLATE,
    get_template,
    list_templates,
)


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


def render_pdf(
    draft: ResumeDraft,
    template_name: str | None = None,
    facts: dict[str, Any] | None = None,
) -> bytes:
    """Render a PDF when WeasyPrint is installed, without importing it at startup."""
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


__all__ = ["DEFAULT_TEMPLATE", "list_templates", "render_html", "render_pdf"]
