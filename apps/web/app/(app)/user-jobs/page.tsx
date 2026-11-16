"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import { useUserJobs } from "@/hooks/use-user-jobs";
import { JobCard } from "@/components/jobs/job-card";
import { JobFilters } from "@/components/jobs/job-filters";
import { PaginationControls } from "@/components/jobs/pagination-controls";
import { JobDetailDrawer } from "@/components/jobs/job-detail-drawer";
import { ReportTab } from "@/components/jobs/job-report-header";
import { UserJob } from "@/lib/api/types";
import { Sparkles, Briefcase, Compass, RefreshCw } from "lucide-react";

export default function UserJobsPage() {
	const {
		filteredJobs,
		pagination,
		page,
		setPage,
		limit,
		setLimit,
		loading,
		error,
		searchQuery,
		setSearchQuery,
		statusFilter,
		setStatusFilter,
		remoteOnly,
		setRemoteOnly,
		hideFilter,
		setHideFilter,
		hideJob,
		refetch,
		isFiltersChanged,
		queryAll,
	} = useUserJobs();

	const [selectedRow, setSelectedRow] = useState<UserJob | null>(null);
	const [activeTab, setActiveTab] = useState<ReportTab>("details");
	const [evalId, setEvalId] = useState<string | null>(null);

	// Reset eval id when a different job is selected so the drawer fetches
	// that job's own evaluation rather than leaking a previous job's id.
	useEffect(() => {
		setEvalId(null);
	}, [selectedRow?.job?.slug]);

	const handleTabChange = (tab: ReportTab) => {
		setActiveTab(tab);
	};

	useEffect(() => {
		if (["details", "report", "resume"].includes(activeTab)) {
			// keep the drawer tab in sync with local state only; no URL navigation needed inside a drawer
		}
	}, [activeTab]);

	const handleClose = () => {
		setSelectedRow(null);
		setActiveTab("details");
	};

	return (
		<div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8 space-y-6">
			{/* Header section */}
			<div className="flex flex-col gap-1 border-b border-border/40 pb-5 sm:flex-row sm:items-center sm:justify-between">
				<div>
					<h1 className="text-2xl font-black tracking-tight text-foreground flex items-center gap-2">
						<span>My Jobs</span>
						<span className="rounded-full bg-emerald-500/10 px-2 py-0.5 text-xs font-bold text-emerald-500 flex items-center gap-1">
							<Sparkles className="h-3 w-3" />
							Connected
						</span>
					</h1>
					<p className="text-sm text-muted-foreground">
						Manage your saved roles, review AI match evaluations, and access tailored resumes.
					</p>
				</div>

				<div className="pt-2 sm:pt-0">
					<Link
						href="/jobs"
						className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-card px-3.5 py-1.5 text-xs font-semibold text-foreground hover:bg-secondary transition-colors"
					>
						<Compass className="h-3.5 w-3.5 text-sky-500" />
						<span>Explore More Jobs</span>
					</Link>
					<button
						type="button"
						onClick={() => refetch()}
						disabled={loading}
						className="ml-3 inline-flex items-center gap-2 rounded-lg border border-border bg-card px-3 py-1.5 text-xs font-semibold text-foreground shadow hover:bg-secondary transition-colors disabled:opacity-50"
						aria-label="Refresh jobs"
					>
						<RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
						<span>Refresh</span>
					</button>
				</div>
			</div>

			{/* Filters & Search */}
			<JobFilters
				searchQuery={searchQuery}
				onSearchChange={setSearchQuery}
				statusFilter={statusFilter}
				onStatusChange={setStatusFilter}
				remoteOnly={remoteOnly}
				onRemoteChange={setRemoteOnly}
				hideFilter={hideFilter}
				onHideFilterChange={setHideFilter}
				totalCount={pagination?.totalItems ?? filteredJobs.length}
				isFiltersChanged={isFiltersChanged}
				onQueryAll={queryAll}
				loading={loading}
				isDrawerOpen={!!selectedRow}
			/>

			{/* Loading state */}
			{loading && (
				<div className="grid gap-4 sm:grid-cols-1">
					{[1, 2, 3].map((i) => (
						<div key={i} className="h-28 rounded-xl border border-border/40 bg-card/50 p-5 animate-pulse" />
					))}
				</div>
			)}

			{/* Error state */}
			{error && !loading && (
				<div className="rounded-xl border border-destructive/20 bg-destructive/5 p-6 text-center">
					<p className="text-sm text-destructive font-medium">Failed to load your jobs: {error.message}</p>
				</div>
			)}

			{/* Empty state */}
			{!loading && !error && filteredJobs.length === 0 && (
				<div className="rounded-2xl border border-dashed border-border/80 bg-card/30 p-12 text-center">
					<div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-muted">
						<Briefcase className="h-6 w-6 text-muted-foreground" />
					</div>
					<h3 className="mt-4 text-base font-bold text-foreground">No connected jobs found</h3>
					<p className="mt-1 text-sm text-muted-foreground max-w-sm mx-auto">
						{searchQuery
							? `No connected jobs matched your search "${searchQuery}".`
							: "You haven't saved any jobs to your list yet. Explore opportunities and save them to review match reports!"}
					</p>
					{!searchQuery && (
						<div className="mt-5">
							<Link
								href="/jobs"
								className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90 transition-colors"
							>
								<Compass className="h-4 w-4" />
								<span>Browse All Jobs</span>
							</Link>
						</div>
					)}
				</div>
			)}

			{/* Job cards list */}
			{!loading && !error && filteredJobs.length > 0 && (
				<div className="grid gap-3 sm:grid-cols-1">
					{filteredJobs.map((userJob) => (
						<JobCard
							key={userJob.id}
							userJob={userJob}
							onSelect={setSelectedRow}
							isSelected={selectedRow?.id === userJob.id}
							onHideToggle={hideJob}
						/>
					))}
				</div>
			)}

			{/* Pagination controls */}
			{!loading && !error && pagination && pagination.totalItems > 0 && (
				<PaginationControls
					page={page}
					limit={limit}
					totalItems={pagination.totalItems}
					totalPages={pagination.totalPages}
					hasNext={pagination.hasNext}
					hasPrev={pagination.hasPrev}
					onPageChange={setPage}
					onLimitChange={setLimit}
					limitOptions={[10, 20, 50]}
				/>
			)}

			{/* Workspace drawer pop-up (opens on card click instead of a new page) */}
			<JobDetailDrawer
				job={selectedRow ? selectedRow.job : null}
				userJobId={selectedRow ? selectedRow.id : undefined}
				isOpen={!!selectedRow}
				onClose={handleClose}
				activeTab={activeTab}
				onTabChange={handleTabChange}
				evalId={evalId}
				showOwnedActions={false}
				isHidden={selectedRow?.is_hidden ?? false}
			/>
		</div>
	);
}
