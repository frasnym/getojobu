"""Type stubs for boto3."""

from typing import Any

def client(
    service_name: str,
    region_name: str | None = None,
    api_version: str | None = None,
    use_ssl: bool | None = None,
    verify: bool | str | None = None,
    endpoint_url: str | None = None,
    aws_access_key_id: str | None = None,
    aws_secret_access_key: str | None = None,
    aws_session_token: str | None = None,
    config: Any = None,
) -> Any: ...

def resource(
    service_name: str,
    region_name: str | None = None,
    api_version: str | None = None,
    use_ssl: bool | None = None,
    verify: bool | str | None = None,
    endpoint_url: str | None = None,
    aws_access_key_id: str | None = None,
    aws_secret_access_key: str | None = None,
    aws_session_token: str | None = None,
    config: Any = None,
) -> Any: ...
