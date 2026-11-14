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
    task_queue_job_scrape: str = "job-scrape"  # job-scrape
    task_queue_job_limit: str = "job-limit"


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
class R2Config:
    account_id: str | None = None
    access_key_id: str | None = None
    secret_access_key: str | None = None
    bucket_name: str = "getojobu"
    endpoint_url: str | None = None


@dataclass(frozen=True)
class ScrapeConfig:
    vendor_methods: dict[str, str]

    def get_method(self, vendor: str) -> str | None:
        val = self.vendor_methods.get(vendor.strip().lower())
        return val if val else None

    def is_enabled(self, vendor: str) -> bool:
        return self.get_method(vendor) is not None


@dataclass(frozen=True)
class Config:
    temporal: TemporalConfig
    api: APIConfig
    omni_route: OmniRouteConfig
    r2: R2Config = R2Config()
    scrape: ScrapeConfig = ScrapeConfig(vendor_methods={})


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
    TEMPORAL_TASK_QUEUE_JOB_ANALYZE, TEMPORAL_TASK_QUEUE_JOB_SCRAPE, API_BASE_URL,
    API_KEY, OMNIROUTE_BASE_URL, OMNIROUTE_API_KEY, OMNIROUTE_MODEL,
    and SCRAPER_METHODS_BY_BOARD_*.
    """
    vendor_methods: dict[str, str] = {}
    board_prefix = "SCRAPER_METHODS_BY_BOARD_"
    for key, value in os.environ.items():
        if key.startswith(board_prefix):
            board = key[len(board_prefix) :].strip().lower()
            if board:
                vendor_methods[board] = value.strip()

    if not any(bool(method) for method in vendor_methods.values()):
        raise MissingEnvError("no scraper method enabled for any vendor")

    return Config(
        temporal=TemporalConfig(
            host=_optional("TEMPORAL_HOST", "localhost:7233"),
            namespace=_required("TEMPORAL_NAMESPACE"),
            task_queue_job_analyze=_optional(
                "TEMPORAL_TASK_QUEUE_JOB_ANALYZE", "job-evaluation"
            ),
            task_queue_job_scrape=_optional(
                "TEMPORAL_TASK_QUEUE_JOB_SCRAPE", "job-scrape"
            ),
            task_queue_job_limit=_optional(
                "TEMPORAL_TASK_QUEUE_JOB_LIMIT", "job-limit"
            ),
        ),
        api=APIConfig(
            base_url=_required("API_BASE_URL"),
            api_key=_required("API_KEY"),
        ),
        omni_route=OmniRouteConfig(
            base_url=_required("OMNIROUTE_BASE_URL"),
            api_key=_required("OMNIROUTE_API_KEY"),
            model=os.getenv("OMNIROUTE_MODEL") or None,
        ),
        r2=R2Config(
            account_id=os.getenv("R2_ACCOUNT_ID"),
            access_key_id=os.getenv("R2_ACCESS_KEY_ID"),
            secret_access_key=os.getenv("R2_SECRET_ACCESS_KEY"),
            bucket_name=_optional("R2_BUCKET_NAME", "getojobu"),
            endpoint_url=os.getenv("R2_ENDPOINT_URL")
            or (
                f"https://{os.getenv('R2_ACCOUNT_ID')}.r2.cloudflarestorage.com"
                if os.getenv("R2_ACCOUNT_ID")
                else None
            ),
        ),
        scrape=ScrapeConfig(vendor_methods=vendor_methods),
    )
