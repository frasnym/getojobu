"""Structured logging for the Python worker.

Mirrors apps/backend/internal/utils/logging/logging.go: JSON-structured
logs to stdout with optional request/workflow context enrichment.
"""

from __future__ import annotations

import sys
from typing import Any

import structlog


def configure(level: str = "info") -> None:
    """Configure structlog to emit JSON logs to stdout."""
    structlog.configure(
        processors=[
            structlog.processors.add_log_level,
            structlog.processors.TimeStamper(fmt="iso"),
            structlog.processors.format_exc_info,
            structlog.processors.JSONRenderer(),
        ],
        wrapper_class=structlog.make_filtering_bound_logger(_level_to_int(level)),
        context_class=dict,
        logger_factory=structlog.PrintLoggerFactory(file=sys.stdout),
        cache_logger_on_first_use=True,
    )


def _level_to_int(level: str) -> int:
    mapping = {
        "debug": 10,
        "info": 20,
        "warn": 30,
        "warning": 30,
        "error": 40,
    }
    return mapping.get(level.lower(), 20)


def get_logger(name: str, **bindings: Any) -> structlog.stdlib.BoundLogger:
    """Return a logger bound with the given static attributes."""
    return structlog.get_logger(name).bind(**bindings)


def with_context(
    logger: structlog.stdlib.BoundLogger, **bindings: Any
) -> structlog.stdlib.BoundLogger:
    """Return a logger enriched with per-call context (e.g. workflow_id)."""
    return logger.bind(**bindings)
