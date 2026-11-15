"use client";

import React from "react";
import { useJobs } from "@/hooks/use-jobs";
import { JobCard } from "@/components/jobs/job-card";
import { JobFilters } from "@/components/jobs/job-filters";
import { Sparkles, Briefcase, RefreshCw } from "lucide-react";

export default function JobsPage() {
  const {
    filteredJobs,
    loading,
    error,
    searchQuery,
    setSearchQuery,
    statusFilter,
    setStatusFilter,
  } = useJobs();

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8 space-y-6">
      {/* Header section */}
      <div className="flex flex-col gap-1 border-b border-border/40 pb-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-black tracking-tight text-foreground flex items-center gap-2">
            <span>Job Opportunities</span>
            <span className="rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs font-bold text-emerald-500">
              Live Feed
            </span>
          </h1>
          <p className="text-sm text-muted-foreground">
            Explore curated roles and review your AI match evaluation reports and tailored resumes.
          </p>
        </div>
      </div>

      {/* Filters & Search */}
      <JobFilters
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        statusFilter={statusFilter}
        onStatusChange={setStatusFilter}
        totalCount={filteredJobs.length}
      />

      {/* Loading state */}
      {loading && (
        <div className="grid gap-4 sm:grid-cols-1">
          {[1, 2, 3].map((i) => (
            <div
              key={i}
              className="h-28 rounded-xl border border-border/40 bg-card/50 p-5 animate-pulse"
            />
          ))}
        </div>
      )}

      {/* Error state */}
      {error && !loading && (
        <div className="rounded-xl border border-destructive/20 bg-destructive/5 p-6 text-center">
          <p className="text-sm text-destructive font-medium">
            Failed to load jobs: {error.message}
          </p>
        </div>
      )}

      {/* Empty state */}
      {!loading && !error && filteredJobs.length === 0 && (
        <div className="rounded-2xl border border-dashed border-border/80 bg-card/30 p-12 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-muted">
            <Briefcase className="h-6 w-6 text-muted-foreground" />
          </div>
          <h3 className="mt-4 text-base font-bold text-foreground">No matching jobs found</h3>
          <p className="mt-1 text-sm text-muted-foreground max-w-sm mx-auto">
            {searchQuery
              ? `No job listings matched your search "${searchQuery}". Try adjusting your filters.`
              : "No jobs are currently available in this view."}
          </p>
        </div>
      )}

      {/* Job cards list */}
      {!loading && !error && filteredJobs.length > 0 && (
        <div className="grid gap-3 sm:grid-cols-1">
          {filteredJobs.map((job) => (
            <JobCard key={job.id} job={job} />
          ))}
        </div>
      )}
    </div>
  );
}
