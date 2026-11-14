"""Type stubs for botocore.config."""

from typing import Any

class Config:
    def __init__(
        self,
        region_name: str | None = None,
        signature_version: str | None = None,
        s3: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> None: ...
