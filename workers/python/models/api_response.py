"""API response contract models (matching the Go backend envelope)."""

from __future__ import annotations

from typing import Generic, TypeVar

from pydantic import BaseModel, ConfigDict, Field

T = TypeVar("T")


class Pagination(BaseModel):
    """Pagination metadata for list responses."""

    model_config = ConfigDict(populate_by_name=True)

    page: int
    limit: int
    total_items: int = Field(alias="totalItems")
    total_pages: int = Field(alias="totalPages")
    has_next: bool = Field(alias="hasNext")
    has_prev: bool = Field(alias="hasPrev")


class Meta(BaseModel):
    """Metadata object present on every response."""

    model_config = ConfigDict(populate_by_name=True)

    timestamp: str | None = None
    request_id: str | None = Field(default=None, alias="requestId")
    pagination: Pagination | None = None


class ErrorDetail(BaseModel):
    """Field-level issue description."""

    field: str
    issue: str


class ErrorPayload(BaseModel):
    """Structured error payload returned on failure."""

    code: str
    message: str
    details: list[ErrorDetail] = Field(default_factory=list)


class ApiResponse(BaseModel, Generic[T]):
    """Canonical API response contract."""

    model_config = ConfigDict(populate_by_name=True)

    success: bool
    data: T | None = None
    meta: Meta | None = None
    error: ErrorPayload | None = None
