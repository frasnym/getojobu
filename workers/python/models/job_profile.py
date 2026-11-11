"""Deprecated: JobProfile has been renamed to JobAnalysis.

This module re-exports JobAnalysis as JobProfile for backward compatibility.
Use models.job_analysis.JobAnalysis in new code.
"""

from models.job_analysis import JobAnalysis, JobRequirement

# Backward-compatible alias
JobProfile = JobAnalysis

__all__ = ["JobAnalysis", "JobProfile", "JobRequirement"]
