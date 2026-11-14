"""Minimal ATS resume template loader and renderer.

Loads the HTML/CSS template from templates/resumes/minimal.html and populates
dynamic data into its placeholders.
"""

from __future__ import annotations

import html as html_mod
from pathlib import Path
from typing import Any

from models.resume import ResumeDraft

_TEMPLATE_PATH = (
    Path(__file__).resolve().parents[3] / "templates" / "resumes" / "minimal.html"
)


def render_minimal_html(
    draft: ResumeDraft, facts: dict[str, Any] | None = None
) -> str:
    template_str = _TEMPLATE_PATH.read_text(encoding="utf-8")
    safe_facts = facts or {}
    name = str(safe_facts.get("name") or "Curriculum Vitae").strip()

    roles: list[str] = []
    for role in draft.jobs.values():
        if not role.selected:
            continue
        bullets = "".join(f"<li>{html_mod.escape(item)}</li>" for item in role.bullets)
        role_title = html_mod.escape(role.role or "")
        company = html_mod.escape(role.company or "")
        period = html_mod.escape(role.period or "")
        roles.append(
            f"<section><h2>{role_title} - {company}</h2>"
            f"<p>{period}</p><ul>{bullets}</ul></section>"
        )
    skills = ", ".join(html_mod.escape(skill) for skill in draft.skills)

    rendered = template_str
    rendered = rendered.replace("{{NAME}}", html_mod.escape(name))
    rendered = rendered.replace("{{HEADLINE}}", html_mod.escape(draft.headline))
    rendered = rendered.replace("{{CONTACT_BLOCK}}", html_mod.escape(draft.contact))
    rendered = rendered.replace("{{EDUCATION_BLOCK}}", html_mod.escape(draft.education))
    rendered = rendered.replace("{{SKILLS_BLOCK}}", skills)
    rendered = rendered.replace("{{EXPERIENCE_BLOCK}}", "".join(roles))
    return rendered
