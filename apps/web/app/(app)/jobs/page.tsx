"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useJobs } from "@/hooks/use-jobs";
import { ExploreJobCard } from "@/components/jobs/explore-job-card";
import { JobDetailDrawer } from "@/components/jobs/job-detail-drawer";
import { JobFilters } from "@/components/jobs/job-filters";
import { saveUserJob } from "@/lib/api/endpoints";
import { Job } from "@/lib/api/types";
import { Compass, Sparkles, Briefcase } from "lucide-react";

export default function ExploreJobsPage() {
  const { getToken } = useAuth();
  const {
    filteredJobs,
    loading,
    error,
    searchQuery,
    setSearchQuery,
    statusFilter,
    setStatusFilter,
    setJobOwned,
  } = useJobs();

  const [selectedJob, setSelectedJob] = useState<Job | null>(null);
  const [savingSlug, setSavingSlug] = useState<string | null>(null);

  const handleSaveJob = async (job: Job) => {
    try {
      setSavingSlug(job.slug);
      const token = await getToken();
      await saveUserJob(job.slug, token);
      // Optimistically update ownership in jobs list
      setJobOwned(job.slug, true);
      if (selectedJob?.slug === job.slug) {
        setSelectedJob((prev) => (prev ? { ...prev, is_owned: true } : prev));
      }
    } catch (err) {
      console.error("Failed to save job to user list:", err);
    } finally {
      setSavingSlug(null);
    }
  };

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8 space-y-6">
      {/* Header section */}
      <div className="flex flex-col gap-1 border-b border-border/40 pb-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-black tracking-tight text-foreground flex items-center gap-2">
            <span>Explore Jobs</span>
            <span className="rounded-full bg-sky-500/10 px-2 py-0.5 text-xs font-bold text-sky-500 flex items-center gap-1">
              <Compass className="h-3 w-3" />
              All Roles
            </span>
          </h1>
          <p className="text-sm text-muted-foreground">
            Browse all accessible job opportunities. Select a role to preview full details and save it to your personal workspace.
          </p>
        </div>

        <div className="pt-2 sm:pt-0">
          <Link
            href="/user-jobs"
            className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-card px-3.5 py-1.5 text-xs font-semibold text-foreground hover:bg-secondary transition-colors"
          >
            <Sparkles className="h-3.5 w-3.5 text-emerald-500" />
            <span>Go to My Jobs</span>
          </Link>
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
            <ExploreJobCard
              key={job.id}
              job={job}
              isSelected={selectedJob?.slug === job.slug}
              onSelect={setSelectedJob}
              onSave={handleSaveJob}
              isSaving={savingSlug === job.slug}
            />
          ))}
        </div>
      )}

      {/* Slide-over right detail drawer */}
      <JobDetailDrawer
        job={selectedJob}
        isOpen={!!selectedJob}
        onClose={() => setSelectedJob(null)}
        onSave={handleSaveJob}
        isSaving={savingSlug === selectedJob?.slug}
      />
    </div>
  );
}
