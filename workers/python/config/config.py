"""Runtime configuration for the Python Temporal worker.

Mirrors apps/backend/internal/config/config.go: env-based, fail-fast on
missing required variables, loaded once at process start.

The worker does NOT access Neon directly. It persists results by calling
the Go API over the internal Docker network.
"""

from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass(frozen=True)
class TemporalConfig:
    host: str
    namespace: str
    task_queue_job_analyze: str


@dataclass(frozen=True)
class APIConfig:
    base_url: str  # e.g. http://api:8080 (internal Docker network)
    api_key: str  # static key for worker->API auth


@dataclass(frozen=True)
class OmniRouteConfig:
    base_url: str  # e.g. http://omniroute:20128 (internal Docker network)
    api_key: str
    model: str | None = None  # e.g. "use-combo"


@dataclass(frozen=True)
class Config:
    temporal: TemporalConfig
    api: APIConfig
    omni_route: OmniRouteConfig


class MissingEnvError(ValueError):
    """Raised when a required environment variable is absent."""


def _required(name: str) -> str:
    value = os.getenv(name)
    if not value:
        raise MissingEnvError(f"{name} is required")
    return value


def _optional(name: str, default: str) -> str:
    return os.getenv(name, default)


def load() -> Config:
    """Load configuration from the environment.

    Reads TEMPORAL_HOST, TEMPORAL_NAMESPACE,
    TEMPORAL_TASK_QUEUE_JOB_ANALYZE, API_BASE_URL, API_KEY,
    OMNIROUTE_BASE_URL, OMNIROUTE_API_KEY, OMNIROUTE_MODEL.
    """
    return Config(
        temporal=TemporalConfig(
            host=_optional("TEMPORAL_HOST", "localhost:7233"),
            namespace=_required("TEMPORAL_NAMESPACE"),
            task_queue_job_analyze=_optional(
                "TEMPORAL_TASK_QUEUE_JOB_ANALYZE", "job-evaluation"
            ),
        ),
        api=APIConfig(
            base_url=_required("API_BASE_URL"),
            api_key=_required("API_KEY"),
        ),
        omni_route=OmniRouteConfig(
            base_url=_required("OMNIROUTE_BASE_URL"),
            api_key=_required("OMNIROUTE_API_KEY"),
            model=os.getenv("OMNIROUTE_MODEL") or os.getenv("LLM_MODEL") or os.getenv("MODEL") or None,
        ),
    )
