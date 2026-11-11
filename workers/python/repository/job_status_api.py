"""Deprecated: JobStatusAPI has been merged into GetojobuAPI.

This module re-exports GetojobuAPI for backward compatibility.
Use repository.getojobu_api.GetojobuAPI in new code.
"""

from repository.getojobu_api import GetojobuAPI as JobStatusAPI
from repository.getojobu_api import NewGetojobuAPI as NewJobStatusAPI

__all__ = ["JobStatusAPI", "NewJobStatusAPI"]
