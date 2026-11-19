"""Activities for UserJobScrapeWorkflow.

fetch_and_extract_job      — fetch a user-supplied URL and extract job fields
update_scraped_job         — PATCH the skeleton job with fields and status=new
queue_scraped_job_analysis — start JobAnalyzeWorkflow for the scraped job
mark_scrape_failed         — record a failed scrape on the skeleton job
"""

from __future__ import annotations

import ipaddress
import os
import re
import socket
from typing import Any
from urllib.parse import urljoin, urlparse

import httpx
from temporalio import activity

from config.config import load as config_load
from contract.contract import WorkerContainer
from contract.contract import new as contract_new
from repository.getojobu_api import NewGetojobuAPI
from utils.logging import get_logger

logger = get_logger("activities.user_job_scrape")

_container: WorkerContainer | None = None


def init_user_job_scrape_container(
    container: WorkerContainer | None = None,
) -> WorkerContainer | None:
    """Build (or reuse) the dependency container."""
    global _container
    if container is not None:
        _container = container
        return _container
    if _container is None:
        try:
            cfg = config_load()
            c, _stopper, err = contract_new(cfg)
            if err is None and c is not None:
                _container = c
        except Exception as e:  # noqa: BLE001
            logger.debug("init_user_job_scrape_container_fallback", error=str(e))
    return _container


def _api() -> Any:
    container = _container
    if container is not None and getattr(container, "repository", None) is not None:
        api = getattr(container.repository, "api", None)
        if api is not None:
            return api
    return NewGetojobuAPI(
        base_url=os.environ.get("API_BASE_URL", "http://localhost:8080"),
        api_key=os.environ.get("API_KEY", ""),
    )


def _is_forbidden_ip(ip: ipaddress.IPv4Address | ipaddress.IPv6Address) -> bool:
    return bool(
        ip.is_private
        or ip.is_loopback
        or ip.is_link_local
        or ip.is_reserved
        or ip.is_multicast
        or ip.is_unspecified
    )


def _validate_url(url: str, *, resolve: bool = False) -> None:
    parsed = urlparse(url)
    if parsed.scheme not in {"http", "https"} or not parsed.hostname:
        raise ValueError("source_url must be an http(s) URL with a host")
    host = parsed.hostname.lower()
    if host in {"localhost", "localhost.localdomain"} or host.endswith(".localhost"):
        raise ValueError("source_url must not point to localhost")
    try:
        ip = ipaddress.ip_address(host)
    except ValueError:
        if not resolve:
            return
        try:
            addresses = socket.getaddrinfo(host, None)
        except OSError as exc:
            raise ValueError("source_url host could not be resolved") from exc
        for info in addresses:
            if _is_forbidden_ip(ipaddress.ip_address(info[4][0])):
                raise ValueError("source_url must not point to a private address")
        return
    if _is_forbidden_ip(ip):
        raise ValueError("source_url must not point to a private address")


def _strip_html(html: str) -> str:
    text = re.sub(r"(?is)<(script|style)[^>]*>.*?</\1>", " ", html)
    text = re.sub(r"(?s)<[^>]+>", " ", text)
    return re.sub(r"\s+", " ", text).strip()


def _slugify(s: str) -> str:
    out: list[str] = []
    prev_dash = False
    for ch in s.strip().lower():
        if ch.isalnum():
            out.append(ch)
            prev_dash = False
        elif not prev_dash:
            out.append("-")
            prev_dash = True
    return "".join(out).strip("-")


def build_manual_slug(owner_id: str, title: str, company: str, location: str) -> str:
    """Match Go's BuildManualJobSlug, including its untitled fallback."""
    title_slug = _slugify(title) or "untitled"
    company_slug = _slugify(company) or "unknown-company"
    location_slug = _slugify(location) or "unspecified-location"
    prefix = owner_id[:8] if len(owner_id) >= 8 else owner_id
    return f"m-{prefix}-{title_slug}-{company_slug}-{location_slug}"


@activity.defn
async def fetch_and_extract_job(input_data: dict[str, Any]) -> dict[str, Any]:
    url = input_data.get("source_url")
    if not isinstance(url, str) or not url:
        raise ValueError("source_url is required")
    _validate_url(url, resolve=True)

    async with httpx.AsyncClient(
        timeout=30.0,
        headers={"User-Agent": "getojobu-manual-scrape/1.0"},
        follow_redirects=False,
    ) as client:
        current_url = url
        response: httpx.Response | None = None
        for _ in range(4):
            response = await client.get(current_url)
            location = response.headers.get("location")
            if response.status_code in {301, 302, 303, 307, 308} and location:
                current_url = urljoin(current_url, location)
                _validate_url(current_url, resolve=True)
                continue
            break
        else:
            raise ValueError("source_url redirected too many times")

        if response is None:
            raise ValueError("source_url fetch failed")
        response.raise_for_status()
        page_text = _strip_html(response.text)

    container = _container
    service = getattr(getattr(container, "usecase", None), "user_job_scrape", None)
    if service is None:
        raise RuntimeError("user job scrape service is not configured")
    fields = await service.extract(page_text)
    return dict(fields)


@activity.defn
async def update_scraped_job(input_data: dict[str, Any]) -> dict[str, Any]:
    old_slug = input_data.get("old_slug")
    owner_id = input_data.get("owner_id")
    extracted = input_data.get("extracted")
    if not isinstance(old_slug, str) or not old_slug:
        raise ValueError("old_slug is required")
    if not isinstance(owner_id, str) or not owner_id:
        raise ValueError("owner_id is required")
    if not isinstance(extracted, dict):
        raise ValueError("extracted is required")

    new_slug = build_manual_slug(
        owner_id,
        str(extracted.get("title") or ""),
        str(extracted.get("company") or ""),
        str(extracted.get("location") or ""),
    )
    try:
        result = await _api().patch_job_status(
            old_slug,
            "new",
            new_slug=new_slug,
            title=extracted.get("title") or None,
            company=extracted.get("company") or None,
            location=extracted.get("location") or None,
            description=extracted.get("description") or None,
            source_url=extracted.get("source_url") or None,
            apply_url=extracted.get("apply_url") or None,
            posted_date=extracted.get("posted_date") or None,
            is_remote=extracted.get("is_remote"),
            visa_relocation=extracted.get("visa_relocation"),
        )
    except httpx.HTTPStatusError as exc:
        if exc.response.status_code != 404:
            raise
        # A retry after a lost response finds the old slug already replaced;
        # the update has been applied, so treat it as success.
        result = {"slug": new_slug, "already_applied": True}
    return {"slug": new_slug, "job": result}


@activity.defn
async def queue_scraped_job_analysis(input_data: dict[str, Any]) -> dict[str, Any]:
    slug = input_data.get("slug")
    if not isinstance(slug, str) or not slug:
        raise ValueError("slug is required")
    response = await _api().post(
        "/v1/jobs/bulk-queue-analyze", json=[{"slug": slug, "force": True}]
    )
    workflow_ids: list[str] = []
    if isinstance(response, dict) and isinstance(response.get("workflow_ids"), list):
        workflow_ids = [str(i) for i in response["workflow_ids"]]
    return {"workflow_ids": workflow_ids}


@activity.defn
async def mark_scrape_pending(input_data: dict[str, Any]) -> dict[str, Any]:
    slug = input_data.get("slug")
    if not isinstance(slug, str) or not slug:
        raise ValueError("slug is required")
    result = await _api().patch_job_status(slug, "scrape_pending")
    return {"status": "scrape_pending", "job": result}


@activity.defn
async def mark_scrape_failed(input_data: dict[str, Any]) -> dict[str, Any]:
    slug = input_data.get("slug")
    message = input_data.get("failure_message")
    if not isinstance(slug, str) or not slug:
        raise ValueError("slug is required")
    result = await _api().patch_job_status(
        slug,
        "scrape_failed",
        failure_message=str(message)[:2000] if message else "scrape failed",
    )
    return {"job": result}
