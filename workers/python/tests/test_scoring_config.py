"""Tests for ScoringConfig and CategoryConfig models."""

from typing import Any

from models.scoring_config import CategoryConfig, ScoringConfig


def test_scoring_config_parse_valid():
    raw: dict[str, Any] = {
        "id": "e0b8d3a1-1234-5678-90ab-cdef12345678",
        "family": "technology",
        "version": 1,
        "categories": [
            {"name": "technical_skills", "weight": 0.35, "description": "skills"},
            {"name": "responsibilities", "weight": 0.25, "description": "resp"},
            {"name": "experience", "weight": 0.20, "description": "exp"},
            {"name": "seniority", "weight": 0.15, "description": "sen"},
            {"name": "domain", "weight": 0.05, "description": "dom"},
        ],
        "is_active": True,
        "created_at": "2026-11-11T12:00:00Z",
        "updated_at": "2026-11-11T12:00:00Z",
    }

    config = ScoringConfig.model_validate(raw)
    assert config.id == "e0b8d3a1-1234-5678-90ab-cdef12345678"
    assert config.family == "technology"
    assert config.version == 1
    assert config.is_active is True
    assert len(config.categories) == 5
    assert config.categories[0].name == "technical_skills"
    assert config.categories[0].weight == 0.35


def test_category_config_weights_sum_helper():
    cats = [
        CategoryConfig(name="skills", weight=0.6, description="skills"),
        CategoryConfig(name="exp", weight=0.4, description="experience"),
    ]
    total = sum(c.weight for c in cats)
    assert round(total, 4) == 1.0
