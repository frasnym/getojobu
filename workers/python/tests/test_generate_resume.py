from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from models.resume import ResumeDraft, validate_resume_draft
from usecase.generate_resume.content import build_resume_content
from usecase.generate_resume.service import GenerateResume


def _draft() -> dict:
    return {
        "headline": "Backend Engineer",
        "skills": ["go", "postgres", "kafka", "grpc", "redis", "python"],
        "jobs": {
            "exp-1": {
                "company": "Acme",
                "role": "Engineer",
                "period": "2022 - Current",
                "bullets": ["Built payment services"],
                "selected": True,
            }
        },
        "education": "BSc Computer Science, Example University",
        "contact": "Alex | alex@example.com",
    }


def test_resume_validation_requires_factual_roles_and_strength() -> None:
    draft = ResumeDraft.model_validate(_draft())
    with pytest.raises(ValueError, match="demonstrated match strength"):
        validate_resume_draft(
            draft,
            facts={"roles": [{"id": "exp-1"}]},
            match_evidence={"strengths": ["distributed systems"]},
        )


def test_resume_content_is_structured_data() -> None:
    content = build_resume_content(
        candidate_profile={"skills": ["go"]},
        job_profile={"title": "Engineer"},
        evaluation_results={"strengths": ["go"]},
        facts={"roles": [{"id": "exp-1"}]},
        overall_score=90,
        eligibility_status="eligible",
        recommendation_label="strong_match",
    )
    assert content["candidate_profile"] == '{"skills": ["go"]}'
    assert '"recommendation_label": "strong_match"' in content["match_evidence"]


@pytest.mark.asyncio
async def test_generate_resume_runs_critique_before_rendering() -> None:
    llm = MagicMock()
    llm.generate_resume = AsyncMock(return_value=_draft())
    llm.critique_resume = AsyncMock(return_value={"passed": True, "issues": []})
    service = GenerateResume(llm=llm)
    with patch(
        "usecase.generate_resume.service.render_pdf", return_value=b"%PDF"
    ) as render:
        result = await service.generate(
            candidate_profile={"skills": ["go"]},
            job_profile={"title": "Engineer"},
            evaluation_results={"strengths": ["payment"]},
            facts={"roles": [{"id": "exp-1"}]},
            overall_score=90,
            eligibility_status="eligible",
            recommendation_label="strong_match",
        )
    assert result["pdf"] == b"%PDF"
    assert result["critique"]["passed"] is True
    render.assert_called_once()
    llm.critique_resume.assert_awaited_once()


@pytest.mark.asyncio
async def test_generate_resume_retries_on_critique_failure() -> None:
    llm = MagicMock()
    llm.generate_resume = AsyncMock(return_value=_draft())
    llm.critique_resume = AsyncMock(
        side_effect=[
            {"passed": False, "issues": ["Repeated verb"]},
            {"passed": True, "issues": []},
        ]
    )
    service = GenerateResume(llm=llm)
    with patch(
        "usecase.generate_resume.service.render_pdf", return_value=b"%PDF"
    ) as render:
        result = await service.generate(
            candidate_profile={"skills": ["go"]},
            job_profile={"title": "Engineer"},
            evaluation_results={"strengths": ["payment"]},
            facts={"roles": [{"id": "exp-1"}]},
            overall_score=90,
            eligibility_status="eligible",
            recommendation_label="strong_match",
        )
    assert result["pdf"] == b"%PDF"
    assert result["critique"]["passed"] is True
    assert llm.generate_resume.await_count == 2
    assert llm.critique_resume.await_count == 2
    render.assert_called_once()


def test_templates_registry_and_rendering() -> None:
    from usecase.generate_resume.renderer import list_templates, render_html

    available = list_templates()
    assert "hermes" in available
    assert "minimal" in available

    draft = ResumeDraft.model_validate(_draft())
    facts = {
        "name": "Jane Doe",
        "phone": "+123456789",
        "email": "jane@example.com",
        "linkedin": "linkedin.com/in/janedoe",
        "github": "github.com/janedoe",
        "education_rows": [
            {
                "institution": "MIT",
                "degree": "BSc",
                "field_of_study": "CS",
                "start_year": 2018,
                "end_year": 2022,
                "gpa": "3.9",
            }
        ],
    }

    # Test hermes template
    html_hermes = render_html(draft, template_name="hermes", facts=facts)
    assert "Jane Doe" in html_hermes
    assert "Backend Engineer" in html_hermes
    assert "jane@example.com" in html_hermes
    assert "linkedin.com/in/janedoe" in html_hermes
    assert "Acme" in html_hermes
    assert "Built payment services" in html_hermes
    assert "BSc in CS, MIT" in html_hermes
    assert "2018 - 2022 | GPA: 3.9" in html_hermes

    # Test minimal template
    html_minimal = render_html(draft, template_name="minimal", facts=facts)
    assert "Jane Doe" in html_minimal
    assert "Acme" in html_minimal

    # Test invalid template
    with pytest.raises(ValueError, match="unknown resume template"):
        render_html(draft, template_name="nonexistent_template")


def test_r2_storage_repository_handles_unconfigured() -> None:
    from config.config import R2Config
    from repository.storage import R2StorageRepository

    repo = R2StorageRepository(R2Config())
    # Should not raise exception when credentials not set
    repo.put_object(key="test.pdf", body=b"data", content_type="application/pdf")


def test_r2_storage_repository_uploads_when_configured() -> None:
    from config.config import R2Config
    from repository.storage import R2StorageRepository

    cfg = R2Config(
        account_id="test-acc",
        access_key_id="test-key",
        secret_access_key="test-secret",
        bucket_name="getojobu",
        endpoint_url="https://test-acc.r2.cloudflarestorage.com",
    )
    with patch("boto3.client") as mock_boto:
        mock_s3 = MagicMock()
        mock_boto.return_value = mock_s3
        repo = R2StorageRepository(cfg)
        repo.put_object(
            key="123/456/eval_resume.pdf",
            body=b"%PDF-1.4",
            content_type="application/pdf",
        )
        mock_s3.put_object.assert_called_once_with(
            Bucket="getojobu",
            Key="123/456/eval_resume.pdf",
            Body=b"%PDF-1.4",
            ContentType="application/pdf",
        )


@pytest.mark.asyncio
async def test_upload_resume_activity_builds_correct_keys() -> None:
    import base64

    from deliveries.consumer.activities.resume_generation import (
        init_resume_generation_container,
        upload_resume_activity,
    )

    container = MagicMock()
    mock_storage = MagicMock()
    container.repository.storage = mock_storage
    init_resume_generation_container(container)

    pdf_bytes = b"%PDF-dummy"
    pdf_b64 = base64.b64encode(pdf_bytes).decode("ascii")

    result = await upload_resume_activity(
        {
            "evaluation_id": "eval-123",
            "job_id": 999,
            "user_id": "user-456",
            "pdf_base64": pdf_b64,
            "resume": {"headline": "Engineer"},
        }
    )

    assert result["pdf_key"] == "999/user-456/eval-123_resume.pdf"
    assert result["content_key"] == "999/user-456/eval-123_resume.json"

    assert mock_storage.put_object.call_count == 2
    mock_storage.put_object.assert_any_call(
        key="999/user-456/eval-123_resume.pdf",
        body=pdf_bytes,
        content_type="application/pdf",
    )


