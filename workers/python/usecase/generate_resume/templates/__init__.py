"""Pluggable resume templates registry and loader."""

from __future__ import annotations

from collections.abc import Callable
from pathlib import Path
from typing import Any

from models.resume import ResumeDraft
from usecase.generate_resume.templates.hermes import render_hermes_html
from usecase.generate_resume.templates.minimal import render_minimal_html

TemplateRenderer = Callable[[ResumeDraft, dict[str, Any] | None], str]

_TEMPLATES_DIR = Path(__file__).resolve().parents[3] / "templates" / "resumes"

TEMPLATES: dict[str, TemplateRenderer] = {
    "hermes": render_hermes_html,
    "classic": render_hermes_html,
    "ats_classic": render_hermes_html,
    "minimal": render_minimal_html,
    "standard": render_minimal_html,
}

DEFAULT_TEMPLATE = "hermes"


def get_template(name: str | None = None) -> TemplateRenderer:
    key = (name or DEFAULT_TEMPLATE).lower().strip()
    renderer = TEMPLATES.get(key)
    if renderer is None:
        available = ", ".join(sorted(set(TEMPLATES.keys())))
        raise ValueError(f"unknown resume template: {name!r}. Available templates: {available}")
    return renderer


def list_templates() -> list[str]:
    """Return distinct canonical template names."""
    return sorted(set(TEMPLATES.keys()))


def list_template_files() -> list[str]:
    """Return list of .html template filenames available in templates/resumes/."""
    if not _TEMPLATES_DIR.exists():
        return []
    return sorted(p.stem for p in _TEMPLATES_DIR.glob("*.html"))


__all__ = [
    "DEFAULT_TEMPLATE",
    "TEMPLATES",
    "TemplateRenderer",
    "get_template",
    "list_template_files",
    "list_templates",
    "render_hermes_html",
    "render_minimal_html",
]
