"""Common infrastructure contract: shared database connection.

Mirrors apps/backend/internal/contract/common/contract.go.
"""

from __future__ import annotations

from typing import Callable

from utils.database import Database


class Contract:
    def __init__(self, db: Database) -> None:
        self.db = db


Stopper = Callable[[], None]


def new(cfg_db_url: str) -> tuple[Contract, Stopper]:
    """Create the common layer (database pool) and a stopper.

    Fail-fast: if the database cannot be reached at startup, this raises.
    """
    from ..utils.database import new_database_sync

    db, stopper = new_database_sync(cfg_db_url)
    return Contract(db=db), stopper
