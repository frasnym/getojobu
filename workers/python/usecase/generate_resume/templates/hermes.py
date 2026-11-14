"""Hermes ATS resume template loader and renderer.

Loads the HTML/CSS template from templates/resumes/hermes.html and populates
dynamic data into its placeholders.
"""

from __future__ import annotations

import html as html_mod
import re
from pathlib import Path
from typing import Any, cast

from models.resume import ResumeDraft

_TEMPLATE_PATH = (
    Path(__file__).resolve().parents[3] / "templates" / "resumes" / "hermes.html"
)


def _escape(text: str) -> str:
    return html_mod.escape(str(text), quote=True)


def _parse_inline_formatting(text: str) -> str:
    """Convert markdown-style formatting: ***bold-italic***, **bold**, *italic*."""
    escaped = html_mod.escape(str(text), quote=True)
    escaped = re.sub(r"\*\*\*([^*]+)\*\*\*", r"<strong><em>\1</em></strong>", escaped)
    escaped = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", escaped)
    escaped = re.sub(r"\*([^*]+)\*", r"<em>\1</em>", escaped)
    return escaped


def render_hermes_html(
    draft: ResumeDraft, facts: dict[str, Any] | None = None
) -> str:
    template_str = _TEMPLATE_PATH.read_text(encoding="utf-8")
    safe_facts = facts or {}

    name = str(safe_facts.get("name") or "Curriculum Vitae").strip()
    headline = str(draft.headline or safe_facts.get("headline") or "").strip()

    # Contact line rendering
    contact_p_tags: list[str] = []
    line1_parts: list[str] = []
    phone = safe_facts.get("phone")
    if phone:
        line1_parts.append(f"<b>Phone:</b> {_escape(str(phone))}")
    email = safe_facts.get("email")
    if email:
        line1_parts.append(f"<b>Email:</b> {_escape(str(email))}")
    if line1_parts:
        contact_p_tags.append(f"<p>{' | '.join(line1_parts)}</p>")

    line2_parts: list[str] = []
    linkedin = safe_facts.get("linkedin")
    if linkedin:
        line2_parts.append(f"<b>LinkedIn:</b> {_escape(str(linkedin))}")
    github = safe_facts.get("github")
    if github:
        line2_parts.append(f"<b>GitHub:</b> {_escape(str(github))}")
    portfolio = safe_facts.get("portfolio")
    if portfolio:
        line2_parts.append(f"<b>Portfolio:</b> {_escape(str(portfolio))}")
    if line2_parts:
        contact_p_tags.append(f"<p>{' | '.join(line2_parts)}</p>")

    if not contact_p_tags:
        contact_p_tags.append(f"<p>{_escape(draft.contact)}</p>")
    contact_html = "\n    ".join(contact_p_tags)

    # Skills line
    skills_content = " | ".join(_parse_inline_formatting(s) for s in draft.skills)

    # Experience section
    jobs_html_list: list[str] = []
    for role in draft.jobs.values():
        if not role.selected:
            continue
        role_title = str(role.role or "").strip()
        company = str(role.company or "").strip()
        period = str(role.period or "").strip()

        header_parts: list[str] = []
        if role_title and company:
            header_parts.append(f"{role_title} | {company}")
        elif role_title or company:
            header_parts.append(role_title or company)
        if period:
            header_parts.append(f"({period})")
        header_text = " ".join(header_parts)

        bullets_html = "".join(
            f"<li>{_parse_inline_formatting(b)}</li>"
            for b in role.bullets
            if b.strip()
        )
        jobs_html_list.append(
            f'<div class="job"><div class="job-head">{_escape(header_text)}</div>'
            f"<ul>{bullets_html}</ul></div>"
        )
    jobs_html = "\n  ".join(jobs_html_list)

    # Education section
    raw_edu_rows = safe_facts.get("education_rows")
    edu_rows: list[object] = (
        cast(list[object], raw_edu_rows) if isinstance(raw_edu_rows, list) else []
    )
    edu_html_list: list[str] = []
    for row in edu_rows:
        if isinstance(row, dict):
            row_dict = cast(dict[str, object], row)
            inst = str(row_dict.get("institution") or "").strip()
            deg = str(row_dict.get("degree") or "").strip()
            field = str(row_dict.get("field_of_study") or "").strip()
            start_y = row_dict.get("start_year")
            end_y = row_dict.get("end_year")
            gpa = str(row_dict.get("gpa") or "").strip()

            year_str = ""
            if start_y and end_y:
                year_str = f"{start_y} - {end_y}"
            elif end_y:
                year_str = str(end_y)
            elif start_y:
                year_str = str(start_y)

            deg_field = f"{deg} in {field}" if deg and field else (deg or field)
            top_line = ", ".join(p for p in [deg_field, inst] if p)
            detail_parts = [p for p in [year_str, f"GPA: {gpa}" if gpa else ""] if p]
            detail_line = " | ".join(detail_parts)

            if top_line:
                edu_html_list.append(f'<div class="edu">{_escape(top_line)}</div>')
            if detail_line:
                edu_html_list.append(
                    f'<div class="edu-detail">{_escape(detail_line)}</div>'
                )

    if not edu_html_list:
        edu_html_list.append(f'<div class="edu">{_escape(draft.education)}</div>')
    edu_html = "\n  ".join(edu_html_list)

    # Substitute placeholders into template
    rendered = template_str
    rendered = rendered.replace("{{NAME}}", _escape(name))
    rendered = rendered.replace("{{HEADLINE}}", _escape(headline))
    rendered = rendered.replace("{{CONTACT_BLOCK}}", contact_html)
    rendered = rendered.replace("{{SKILLS_BLOCK}}", skills_content)
    rendered = rendered.replace("{{EXPERIENCE_BLOCK}}", jobs_html)
    rendered = rendered.replace("{{EDUCATION_BLOCK}}", edu_html)
    return rendered
