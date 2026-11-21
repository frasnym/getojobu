"""The generated resume must never exceed the hard character limit."""

from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from models.resume import ResumeDraft, ResumeRole
from usecase.generate_resume.renderer import (
    MAX_RESUME_CHARS,
    count_resume_chars,
    fit_resume_to_char_limit,
)


def _draft(*, skills: int = 3, roles: int = 1, bullets: int = 2) -> ResumeDraft:
    jobs = {}
    for index in range(roles):
        jobs[f"exp-{index}"] = {
            "company": f"Company {index}",
            "role": f"Engineer {index}",
            "period": "2020 - Current",
            "bullets": [f"Built thing {index}-{n}" for n in range(bullets)],
            "selected": True,
        }
    return ResumeDraft.model_validate(
        {
            "headline": "Backend Engineer",
            "skills": [f"skill-{n}" for n in range(skills)],
            "jobs": jobs,
            "education": "BSc Computer Science, Example University",
            "contact": "Alex | alex@example.com",
        }
    )


def test_limit_is_2900() -> None:
    assert MAX_RESUME_CHARS == 2900


def test_count_resume_chars_ignores_unselected_roles() -> None:
    draft = _draft()
    draft.jobs["exp-unselected"] = ResumeRole(
        bullets=["This text is not rendered"], selected=False
    )
    assert count_resume_chars(draft) == count_resume_chars(_draft())


def test_fit_is_noop_when_within_limit() -> None:
    draft = _draft()
    fitted = fit_resume_to_char_limit(draft)
    assert fitted == draft
    assert count_resume_chars(fitted) <= MAX_RESUME_CHARS


def test_fit_trims_least_relevant_role_first() -> None:
    draft = _draft(roles=3, bullets=6)
    long_bullet = "Built a distributed system " * 20
    for role in draft.jobs.values():
        role.bullets = [long_bullet for _ in range(10)]

    assert count_resume_chars(draft) > MAX_RESUME_CHARS
    fitted = fit_resume_to_char_limit(draft)

    assert count_resume_chars(fitted) <= MAX_RESUME_CHARS
    assert fitted.jobs["exp-0"].selected is True
    assert len(fitted.jobs["exp-0"].bullets) >= 1
    for role_id in ("exp-1", "exp-2"):
        assert fitted.jobs[role_id].selected is False
        assert fitted.jobs[role_id].bullets == []


def test_fit_trims_skills_after_roles_are_exhausted() -> None:
    draft = _draft(skills=12, roles=1, bullets=1)
    draft.headline = "Senior Backend Engineer " * 40
    draft.skills = [f"long-skill-name-{n}" * 20 for n in range(12)]

    fitted = fit_resume_to_char_limit(draft)

    assert count_resume_chars(fitted) <= MAX_RESUME_CHARS
    assert len(fitted.skills) < 12


def test_fit_raises_when_content_cannot_fit() -> None:
    with patch(
        "usecase.generate_resume.renderer.count_resume_chars", return_value=99999
    ):
        with pytest.raises(ValueError, match="2900 characters"):
            _ = fit_resume_to_char_limit(_draft())


@pytest.mark.asyncio
async def test_service_applies_char_limit_before_rendering() -> None:
    from usecase.generate_resume.service import GenerateResume

    llm = MagicMock()
    llm.generate_resume = AsyncMock(
        return_value={
            "headline": "Backend Engineer",
            "skills": ["python", "sql"],
            "jobs": {
                "exp-1": {
                    "company": "Acme",
                    "role": "Engineer",
                    "period": "2020 - Current",
                    "bullets": ["Built payment services"],
                    "selected": True,
                }
            },
            "education": "BSc Computer Science, Example University",
            "contact": "Alex | alex@example.com",
        }
    )
    llm.critique_resume = AsyncMock(return_value={"passed": True, "issues": []})
    service = GenerateResume(llm=llm)

    def passthrough(draft: ResumeDraft) -> ResumeDraft:
        return draft

    with (
        patch("usecase.generate_resume.service.render_pdf", return_value=b"%PDF"),
        patch(
            "usecase.generate_resume.service.fit_resume_to_char_limit",
            side_effect=passthrough,
        ) as fit,
    ):
        _ = await service.generate(
            candidate_profile={"skills": ["go"]},
            job_profile={"title": "Engineer"},
            evaluation_results={"strengths": ["payment"]},
            facts={"roles": [{"id": "exp-1"}]},
            overall_score=90,
            eligibility_status="eligible",
            recommendation_label="strong_match",
        )
    fit.assert_called_once()
