"use client";

import React, { useState, useEffect, useMemo, useCallback } from "react";
import Link from "next/link";
import { useUserJobs } from "@/hooks/use-user-jobs";
import { JobCard } from "@/components/jobs/job-card";
import { JobFilters } from "@/components/jobs/job-filters";
import { PaginationControls } from "@/components/jobs/pagination-controls";
import { JobDetailDrawer } from "@/components/jobs/job-detail-drawer";
import { ReportTab } from "@/components/jobs/job-report-header";
import { UserJob } from "@/lib/api/types";
import { Sparkles, Briefcase, Compass, RefreshCw, CheckSquare, Eye, EyeOff, X } from "lucide-react";

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
		bulkHideJob,
		retryJob,
		refetch,
		isFiltersChanged,
		queryAll,
		evaluationStatusFilter,
		setEvaluationStatusFilter,
		reportStatusFilter,
		setReportStatusFilter,
		eligibilityStatusFilter,
		setEligibilityStatusFilter,
		recommendationLabelFilter,
		setRecommendationLabelFilter,
	} = useUserJobs();

	const [selectedRow, setSelectedRow] = useState<UserJob | null>(null);
	const [activeTab, setActiveTab] = useState<ReportTab>("details");
	const [evalId, setEvalId] = useState<string | null>(null);

	// Bulk selection state (checked job ids on the current page).
	const [checkedIds, setCheckedIds] = useState<Set<string>>(new Set());
	const [bulkBusy, setBulkBusy] = useState(false);

	const visibleIds = useMemo(() => filteredJobs.map((uj) => uj.id).filter(Boolean), [filteredJobs]);
	const allVisibleChecked = visibleIds.length > 0 && visibleIds.every((id) => checkedIds.has(id));

	const toggleCheck = useCallback((id: string, checked: boolean) => {
		setCheckedIds((prev) => {
			const next = new Set(prev);
			if (checked) next.add(id);
			else next.delete(id);
			return next;
		});
	}, []);

	const toggleCheckAll = useCallback(() => {
		setCheckedIds((prev) => {
			const next = new Set(prev);
			if (allVisibleChecked) {
				visibleIds.forEach((id) => next.delete(id));
			} else {
				visibleIds.forEach((id) => next.add(id));
			}
			return next;
		});
	}, [allVisibleChecked, visibleIds]);

	const clearChecks = useCallback(() => setCheckedIds(new Set()), []);

	const runBulkHide = useCallback(
		async (isHidden: boolean) => {
			const ids = Array.from(checkedIds);
			if (ids.length === 0) return;
			setBulkBusy(true);
			try {
				await bulkHideJob(ids, isHidden);
				setCheckedIds(new Set());
			} finally {
				setBulkBusy(false);
			}
		},
		[checkedIds, bulkHideJob]
	);

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

	const handleDrawerHideToggle = useCallback(
		async (userJobId: string, nextHidden: boolean) => {
			await hideJob(userJobId, nextHidden);
			setSelectedRow((prev) =>
				prev?.id === userJobId ? { ...prev, is_hidden: nextHidden } : prev
			);
		},
		[hideJob]
	);

	const handleClose = () => {
		setSelectedRow(null);
		setActiveTab("details");
	};

	return (
		<div className="mx-auto max-w-screen-2xl px-4 py-8 sm:px-6 lg:px-10 space-y-8">
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
					<button
						type="button"
						onClick={() => refetch()}
						disabled={loading}
						className="inline-flex items-center gap-2 rounded-lg border border-border bg-card px-3 py-1.5 text-xs font-semibold text-foreground shadow hover:bg-secondary transition-colors disabled:opacity-50"
						aria-label="Refresh jobs"
					>
						<RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
						<span>Refresh</span>
					</button>
					{filteredJobs.length > 0 && (
						<>
							<button
								type="button"
								onClick={toggleCheckAll}
								disabled={bulkBusy}
								className="ml-3 inline-flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-foreground hover:bg-secondary disabled:opacity-50 disabled:pointer-events-none transition-colors"
							>
								<CheckSquare className="h-3.5 w-3.5" />
								<span>{allVisibleChecked ? "Select none" : "Select all"}</span>
							</button>
							<button
								type="button"
								onClick={() => runBulkHide(true)}
								disabled={bulkBusy || checkedIds.size === 0}
								className={`ml-3 inline-flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-semibold transition-colors disabled:opacity-50 disabled:pointer-events-none ${
									bulkBusy || checkedIds.size === 0
										? "border-border/40 text-muted-foreground bg-muted"
										: "border-red-500/40 text-red-400 dark:text-red-300 hover:bg-red-500/10"
								}`}
								aria-label="Hide selected jobs"
							>
								<EyeOff className="h-3.5 w-3.5" />
								<span>{bulkBusy ? "Hiding..." : "Hide selected"}</span>
							</button>
						</>
					)}
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
				evaluationStatusFilter={evaluationStatusFilter}
				onEvaluationStatusChange={setEvaluationStatusFilter}
				reportStatusFilter={reportStatusFilter}
				onReportStatusChange={setReportStatusFilter}
				eligibilityStatusFilter={eligibilityStatusFilter}
				onEligibilityStatusChange={setEligibilityStatusFilter}
				recommendationLabelFilter={recommendationLabelFilter}
				onRecommendationLabelChange={setRecommendationLabelFilter}
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
						<div key={i} className="h-28 rounded-xl border border-border/40 bg-card/50 p-6 animate-pulse" />
					))}
				</div>
			)}

			{/* Error state */}
			{error && !loading && (
				<div className="rounded-xl border border-red-500/20 bg-red-500/5 p-6 text-center">
					<p className="text-sm text-red-400 dark:text-red-300 font-medium">Failed to load your jobs: {error.message}</p>
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
				<div className="space-y-4 sm:grid-cols-1">
					{filteredJobs.map((userJob) => (
						<JobCard
							key={userJob.id}
							userJob={userJob}
							onSelect={setSelectedRow}
							canOpenDetail={userJob.job.status === "analyzed"}
							isSelected={selectedRow?.id === userJob.id}
							onHideToggle={hideJob}
							onRetry={retryJob}
							selectable
							isChecked={checkedIds.has(userJob.id)}
							onCheckChange={(checked) => toggleCheck(userJob.id, checked)}
						/>
					))}
				</div>
			)}

			{/* Bulk action bar */}
			{checkedIds.size > 0 && (
				<div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 sm:gap-0">
					<div className="inline-flex items-center gap-2 text-sm font-semibold text-foreground">
						<span>{checkedIds.size}</span>
						<span className="text-muted-foreground">selected</span>
					</div>
					<div className="flex items-center gap-2">
						<button
							type="button"
							onClick={clearChecks}
							disabled={bulkBusy}
							className="inline-flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-foreground hover:bg-secondary disabled:opacity-50 disabled:pointer-events-none transition-colors"
						>
							<X className="h-3.5 w-3.5" />
							<span>Clear selection</span>
						</button>
						<button
							type="button"
							onClick={() => runBulkHide(true)}
							disabled={bulkBusy}
							className={`inline-flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-semibold transition-colors disabled:opacity-50 disabled:pointer-events-none ${
								bulkBusy
									? "border-border/40 text-muted-foreground bg-muted"
									: "border-red-500/40 text-red-400 dark:text-red-300 hover:bg-red-500/10"
							}`}
							aria-label="Hide selected jobs"
						>
							{bulkBusy ? (
								<EyeOff className="h-3.5 w-3.5 animate-spin" />
							) : (
								<EyeOff className="h-3.5 w-3.5" />
							)}
							<span>{bulkBusy ? "Hiding..." : "Hide selected"}</span>
						</button>
						<button
							type="button"
							onClick={() => toggleCheckAll()}
							disabled={bulkBusy}
							className="inline-flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-foreground hover:bg-secondary disabled:opacity-50 disabled:pointer-events-none transition-colors"
						>
							{allVisibleChecked ? (
								<>
									<CheckSquare className="h-3.5 w-3.5" />
									<span>Select none</span>
								</>
							) : (
								<>
									<CheckSquare className="h-3.5 w-3.5" />
									<span>Select all</span>
								</>
							)}
						</button>
					</div>
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
				onHideToggle={handleDrawerHideToggle}
			/>
		</div>
	);
}
