"""PostgreSQL / Neon connection helper for the worker.

Mirrors apps/backend/internal/utils/database/postgres.go: creates a
connection pool from DATABASE_URL and returns a stopper. All queries use
parameterized statements; no raw interpolation of user-controlled values.
"""

from __future__ import annotations

import asyncio
from typing import Any, Optional, Callable

from psycopg_pool import AsyncConnectionPool


class Database:
    """Async database connection pool wrapper."""

    def __init__(self, pool: AsyncConnectionPool) -> None:
        self._pool = pool

    async def close(self) -> None:
        await self._pool.close()

    async def fetchrow(self, sql: str, *args: Any) -> Optional[dict[str, Any]]:
        """Execute a query returning at most one row."""
        async with self._pool.connection() as conn:
            async with conn.cursor() as cur:
                await cur.execute(sql, args)
                row = await cur.fetchone()
                if row is None:
                    return None
                columns = [desc.name for desc in cur.description]
                return dict(zip(columns, row))

    async def fetchall(self, sql: str, *args: Any) -> list[dict[str, Any]]:
        """Execute a query returning zero or more rows."""
        async with self._pool.connection() as conn:
            async with conn.cursor() as cur:
                await cur.execute(sql, args)
                rows = await cur.fetchall()
                columns = [desc.name for desc in cur.description]
                return [dict(zip(columns, row)) for row in rows]

    async def execute(self, sql: str, *args: Any) -> None:
        """Execute a statement with no expected result rows."""
        async with self._pool.connection() as conn:
            async with conn.cursor() as cur:
                await cur.execute(sql, args)


async def new_database(url: str) -> Database:
    """Create an async connection pool from a DATABASE_URL."""
    pool = AsyncConnectionPool(
        conninfo=url,
        min_size=1,
        max_size=10,
        open=False,
    )
    await pool.open()
    return Database(pool=pool)


Stopper = Callable[[], None]


def new_database_sync(url: str) -> tuple[Database, Stopper]:
    """Synchronous entry point that creates the pool and returns a stopper.

    The pool is opened lazily on first use by async callers; this wrapper
    exists so the contract wiring (which is sync) can hold a stopper.
    """
    pool = AsyncConnectionPool(
        conninfo=url,
        min_size=1,
        max_size=10,
        open=False,
    )
    db = Database(pool=pool)

    def stopper() -> None:
        try:
            asyncio.run(db.close())
        except Exception:  # pragma: no cover - best-effort shutdown
            pass

    return db, stopper
