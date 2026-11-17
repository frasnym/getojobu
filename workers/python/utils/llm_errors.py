"""Shared error type for failures from the configured LLM provider."""

from __future__ import annotations


class LLMError(RuntimeError):
    """A clear, two-part failure from an LLM call.

    ``message`` is a short general summary; ``detail`` carries the underlying
    provider or SDK cause. ``str(exc)`` combines both so the stored
    ``failure_message`` stays readable without losing the root cause.
    """

    def __init__(self, message: str, *, detail: str) -> None:
        self.message = message
        self.detail = detail
        super().__init__(f"{message}: {detail}")
