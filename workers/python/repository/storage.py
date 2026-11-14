"""Object storage repository for Cloudflare R2 / S3."""

from __future__ import annotations

from typing import Any, Protocol

import boto3
from botocore.config import Config as BotoConfig

from config.config import R2Config
from utils.logging import get_logger

logger = get_logger(__name__)


class StorageRepository(Protocol):
    def put_object(self, *, key: str, body: bytes, content_type: str) -> None: ...


class R2StorageRepository:
    """Cloudflare R2 implementation using boto3 (S3-compatible)."""

    def __init__(self, cfg: R2Config) -> None:
        self._cfg = cfg
        self._client: Any = None
        if cfg.access_key_id and cfg.secret_access_key and cfg.endpoint_url:
            self._client = boto3.client(
                service_name="s3",
                endpoint_url=cfg.endpoint_url,
                aws_access_key_id=cfg.access_key_id,
                aws_secret_access_key=cfg.secret_access_key,
                region_name="auto",
                config=BotoConfig(
                    s3={"addressing_style": "path"},
                    signature_version="s3v4",
                ),
            )

    def put_object(self, *, key: str, body: bytes, content_type: str) -> None:
        if self._client is None:
            logger.warning(
                "r2_storage_not_configured",
                key=key,
                msg="R2 credentials not set; skipping object upload",
            )
            return
        bucket = self._cfg.bucket_name or "getojobu"
        logger.info("uploading_to_r2", bucket=bucket, key=key, size=len(body))
        self._client.put_object(
            Bucket=bucket,
            Key=key,
            Body=body,
            ContentType=content_type,
        )
