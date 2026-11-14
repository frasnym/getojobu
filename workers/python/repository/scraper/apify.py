"""Shared Apify HTTP helpers and base ApifyScraper."""

from __future__ import annotations

import asyncio
import urllib.parse
from abc import abstractmethod
from typing import Any, cast

import httpx
from temporalio import activity

from repository.scraper import apify_accounts
from repository.scraper.base import ScraperRepository
from utils.logging import get_logger

logger = get_logger("scraper.apify")

APIFY_BASE = "https://api.apify.com/v2"


class ApifyScraper(ScraperRepository):
    """Base class for scrapers executing an Apify actor."""

    actor_id: str
    timeout_seconds: float = 300.0
    poll_interval: float = 5.0

    def __init__(self, name: str, vendor: str, actor_id: str) -> None:
        self.name = name
        self.vendor = vendor
        self.actor_id = actor_id

    @abstractmethod
    def build_run_input(self, keyword: str, location: str) -> dict[str, Any]:
        """Build actor-specific input payload."""
        raise NotImplementedError

    @abstractmethod
    def parse_items(self, items: list[dict[str, Any]]) -> list[dict[str, Any]]:
        """Parse raw dataset items into standard job dictionaries."""
        raise NotImplementedError

    async def scrape(self, keyword: str, location: str) -> list[dict[str, Any]]:
        token, label, _stats = await asyncio.to_thread(apify_accounts.pick_token)
        if not token:
            raise RuntimeError(
                f"no Apify account has enough credit for scraper {self.name}"
            )

        logger.info(
            "apify_run_start",
            scraper=self.name,
            actor=self.actor_id,
            account=label,
            keyword=keyword,
            location=location,
        )

        run_input = self.build_run_input(keyword, location)
        dataset_id = await self._start_run_and_wait(token, run_input)
        raw_items = await self._fetch_dataset_items(token, dataset_id)

        # Update remaining credit after run
        try:
            usage = await asyncio.to_thread(apify_accounts.get_usage, token)
            if usage:
                await asyncio.to_thread(apify_accounts.record_balance, token, usage)
        except Exception as e:  # noqa: BLE001
            logger.debug("apify_balance_record_failed", error=str(e))

        parsed = self.parse_items(raw_items)
        logger.info(
            "apify_run_complete",
            scraper=self.name,
            found_raw=len(raw_items),
            parsed=len(parsed),
        )
        return parsed

    async def _start_run_and_wait(self, token: str, run_input: dict[str, Any]) -> str:
        actor_path = urllib.parse.quote(self.actor_id, safe="~")
        url = f"{APIFY_BASE}/acts/{actor_path}/runs"

        headers = {
            "Authorization": f"Bearer {token}",
            "Content-Type": "application/json",
        }
        async with httpx.AsyncClient(timeout=60.0) as client:
            resp = await client.post(url, json=run_input, headers=headers)
            if resp.status_code >= 400:
                detail = resp.text[:400]
                raise RuntimeError(
                    f"Apify actor {self.actor_id} run start failed ({resp.status_code}): {detail}"
                )

            data = resp.json().get("data", {})
            run_id = data.get("id")
            default_dataset_id = data.get("defaultDatasetId")
            if not run_id:
                raise RuntimeError(f"no run id returned for actor {self.actor_id}")

            poll_url = f"{APIFY_BASE}/actor-runs/{run_id}"
            elapsed = 0.0
            while elapsed < self.timeout_seconds:
                if activity.in_activity():
                    try:
                        activity.heartbeat(
                            {
                                "status": "waiting_for_apify",
                                "actor": self.actor_id,
                                "run_id": run_id,
                                "elapsed": elapsed,
                            }
                        )
                    except Exception:  # noqa: BLE001, S110
                        pass

                await asyncio.sleep(self.poll_interval)
                elapsed += self.poll_interval

                run_resp = await client.get(poll_url, headers=headers)
                if run_resp.status_code >= 400:
                    continue

                run_data = run_resp.json().get("data", {})
                status = run_data.get("status")

                if status == "SUCCEEDED":
                    dataset_id = run_data.get("defaultDatasetId") or default_dataset_id
                    if not dataset_id:
                        raise RuntimeError(
                            f"no defaultDatasetId found in completed run {run_id} for {self.actor_id}"
                        )
                    return str(dataset_id)
                if status in ("FAILED", "ABORTED", "TIMED-OUT"):
                    raise RuntimeError(
                        f"Apify actor {self.actor_id} run {run_id} ended with status: {status}"
                    )

            raise RuntimeError(
                f"Apify actor {self.actor_id} run {run_id} timed out after {int(self.timeout_seconds)}s"
            )

    async def _fetch_dataset_items(
        self, token: str, dataset_id: str, page_size: int = 1000
    ) -> list[dict[str, Any]]:
        headers = {"Authorization": f"Bearer {token}"}
        items: list[dict[str, Any]] = []
        offset = 0

        async with httpx.AsyncClient(timeout=60.0) as client:
            while True:
                url = (
                    f"{APIFY_BASE}/datasets/{dataset_id}/items"
                    f"?clean=true&limit={page_size}&offset={offset}"
                )
                resp = await client.get(url, headers=headers)
                if resp.status_code >= 400:
                    detail = resp.text[:400]
                    raise RuntimeError(
                        f"Apify dataset {dataset_id} fetch failed ({resp.status_code}): {detail}"
                    )

                raw_data: Any = resp.json()
                if not isinstance(raw_data, list) or not raw_data:
                    break

                batch = cast(list[object], raw_data)
                batch_items: list[dict[str, Any]] = [
                    cast(dict[str, Any], item)
                    for item in batch
                    if isinstance(item, dict)
                ]
                items.extend(batch_items)
                if len(batch) < page_size:
                    break
                offset += len(batch)

        return items
