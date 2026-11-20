"use client";

import React, { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { Job } from "@/lib/api/types";
import { ReportTab } from "@/components/jobs/job-report-header";
import { JobDetailTab } from "@/components/jobs/job-detail-tab";
import { MatchReportTab } from "@/components/matching/match-report-tab";
import { ResumeTab } from "@/components/resume/resume-tab";
import { CoverLetterTab } from "@/components/cover-letter/cover-letter-tab";
import { ApplicationChatTab } from "@/components/jobs/application-chat-tab";
import { getUserJobEvaluation } from "@/lib/api/endpoints";
import { EvaluationDetail } from "@/lib/api/types";
import { EvaluationStatusTag, EligibilityTag, RecommendationTag, StatusTag } from "@/components/jobs/evaluation-tags";
import {
	X,
	MapPin,
	Globe,
	Building2,
	Calendar,
	Clock,
	Bookmark,
	Check,
	AlertTriangle,
	ExternalLink,
	Loader2,
	Sparkles,
	Eye,
	EyeOff,
	RefreshCw,
} from "lucide-react";

interface JobDetailDrawerProps {
	job: Job | null;
	userJobId?: string;
	isOpen: boolean;
	onClose: () => void;
	onSave?: (job: Job) => void;
	isSaving?: boolean;
	// Workspace mode: render the full tabbed detail (Job Details / Match Report / Tailored Resume)
	activeTab?: ReportTab;
	onTabChange?: (tab: ReportTab) => void;
	evalId?: string | null;
	showOwnedActions?: boolean;
	/** When true (job is hidden by the user), grey out the whole drawer header. */
	isHidden?: boolean;
	/** Toggle hide on the user↔job link (bulk endpoint). Receives (userJobId, isHidden). */
	onHideToggle?: (userJobId: string, isHidden: boolean) => void | Promise<void>;
}

const TABS: { id: ReportTab; label: string }[] = [
	{ id: "details", label: "Job Details" },
	{ id: "report", label: "Match Report" },
	{ id: "resume", label: "Tailored Resume" },
	{ id: "cover-letter", label: "Cover Letter" },
	{ id: "chat", label: "Application Helper" },
];

export function JobDetailDrawer({
	job,
	userJobId: userJobIdProp,
	isOpen,
	onClose,
	onSave,
	isSaving,
	activeTab,
	onTabChange,
	evalId: evalIdProp,
	showOwnedActions = true,
	isHidden = false,
	onHideToggle,
}: JobDetailDrawerProps) {
	const { getToken } = useAuth();
	const [fetchedEvaluation, setFetchedEvaluation] = useState<EvaluationDetail | null>(null);
	const [fetchingEvaluation, setFetchingEvaluation] = useState(false);
	const userJobId = userJobIdProp ?? job?.user_job?.id;

	// Resolve evaluation id: prop (from analyze flow) takes precedence,
	// otherwise fall back to the id from the fetched evaluation record.
	const resolvedEvalId = evalIdProp ?? fetchedEvaluation?.id ?? null;

	// Resolve the report payload: fetched evaluation (full data, one fetch)
	// takes precedence; otherwise MatchReportTab falls back to evalId fetch.
	const resolvedReport = fetchedEvaluation?.evaluation ?? undefined;

	// Clear fetched evaluation when switching jobs or userJob links to avoid
	// ghosting a previous job's evaluation into another job's drawer.
	useEffect(() => {
		setFetchedEvaluation(null);
	}, [job?.slug, userJobId]);

	useEffect(() => {
		const handleKeyDown = (e: KeyboardEvent) => {
			if (e.key === "Escape" && isOpen) {
				onClose();
			}
		};

		if (isOpen) {
			window.addEventListener("keydown", handleKeyDown);
			document.body.style.overflow = "hidden";
		}

		return () => {
			window.removeEventListener("keydown", handleKeyDown);
			document.body.style.overflow = "unset";
		};
	}, [isOpen, onClose]);

	// Fetch the latest evaluation for this user_job when the drawer opens.
	// The /evaluation response already includes evaluation_data and
	// report_data, so one fetch is enough to render the match report and to
	// expose the resume generation status (for the presigned download button).
	const [evalRefreshTick, setEvalRefreshTick] = useState(0);
	const refreshEvaluation = useCallback(() => setEvalRefreshTick((t) => t + 1), []);

	useEffect(() => {
		if (!isOpen || !job || !userJobId) return;

		let cancelled = false;
		const load = async () => {
			setFetchingEvaluation(true);
			try {
				const token = await getToken();
				const evalRec = await getUserJobEvaluation(userJobId, token);
				if (!cancelled && evalRec.id) {
					setFetchedEvaluation(evalRec);
				}
			} catch (err) {
				// Evaluation may not exist yet — that's fine, the Analyze button
				// creates one. Silently ignore so the drawer still opens.
				console.debug("No existing evaluation for user job", userJobId, err);
			} finally {
				if (!cancelled) setFetchingEvaluation(false);
			}
		};

		load();
		return () => {
			cancelled = true;
		};
	}, [isOpen, job, userJobId, evalIdProp, getToken, evalRefreshTick]);

	if (!isOpen || !job) {
		return null;
	}

	const isOwned = job.is_owned === true;
	const applyUrl = job.apply_url || job.source_url;
	const workspaceMode = activeTab !== undefined;
	const tab = activeTab ?? "details";

	return (
		<div className="fixed inset-0 z-50">
			{/* Backdrop */}
			<div
				className="fixed inset-0 bg-background/80 backdrop-blur-sm transition-opacity animate-in fade-in duration-200"
				onClick={onClose}
				aria-hidden="true"
			/>

			{/* Slide-over panel */}
			<aside
				role="dialog"
				aria-modal="true"
				aria-label={job.title}
				className="fixed z-50 flex top-0 right-0 bottom-0 w-full max-w-full lg:max-w-[80vw] flex-col bg-card border-l border-border shadow-2xl animate-in slide-in-from-right duration-300"
			>
				{/* Drawer Header — same four rows as the job card */}
				<div className={`border-b border-border/60 p-6 space-y-4 ${isHidden ? "opacity-60 grayscale" : ""}`}>
					<div className="flex flex-wrap items-center gap-2">
						<span className="text-xs font-bold uppercase tracking-wider text-muted-foreground">{job.company}</span>
						{job.is_remote && (
							<StatusTag
								label="Remote"
								tone="success"
								icon={<Globe className="h-3 w-3" />}
								testId="remote-tag"
							/>
						)}
						<EligibilityTag status={fetchedEvaluation?.evaluation.eligibility_status} />
						<RecommendationTag label={fetchedEvaluation?.evaluation.recommendation_label} />
						<div className="ml-auto flex items-center gap-1">
							{userJobId && (
								<button
									type="button"
									onClick={refreshEvaluation}
									disabled={fetchingEvaluation}
									className="rounded-lg p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors disabled:opacity-50"
									aria-label="Refresh evaluation"
									title="Refresh evaluation"
								>
									<RefreshCw className={`h-5 w-5 ${fetchingEvaluation ? "animate-spin" : ""}`} />
								</button>
							)}
							<button
								type="button"
								onClick={onClose}
								className="rounded-lg p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
								aria-label="Close job details"
							>
								<X className="h-5 w-5" />
							</button>
						</div>
					</div>

					<div className="flex flex-wrap items-center gap-x-3 gap-y-2">
						<h2 className={`text-2xl font-black tracking-tight ${isHidden ? "text-muted-foreground" : "text-foreground"}`}>
							{job.title}
						</h2>
						{onHideToggle && userJobId && (
							<button
								type="button"
								onClick={() => onHideToggle(userJobId, !isHidden)}
								disabled={isSaving}
								className="inline-flex items-center gap-1 rounded-lg border border-border/60 bg-card px-2 py-1 text-xs font-medium text-foreground transition-colors hover:bg-secondary focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
								aria-label={isHidden ? "Unhide job" : "Hide job"}
								title={isHidden ? "Unhide job" : "Hide job"}
							>
								{isHidden ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
								<span>{isHidden ? "Unhide" : "Hide"}</span>
							</button>
						)}
					</div>

					<div className="flex flex-wrap items-center gap-5 text-xs text-muted-foreground">
						<span className="flex items-center gap-1">
							<MapPin className="h-3.5 w-3.5" />
							{job.location}
						</span>
						{job.posted_date && (
							<span className="flex items-center gap-1">
								<Clock className="h-3.5 w-3.5" />
								{new Date(job.posted_date).toLocaleDateString(undefined, {
									month: "short",
									day: "numeric",
								})}
							</span>
						)}
					</div>

					{(job.status || fetchedEvaluation) && (
						<div className="flex flex-wrap items-center gap-2.5 border-t border-border/40 pt-3">
							<EvaluationStatusTag status={job.status} />
							<EvaluationStatusTag status={fetchedEvaluation?.evaluation.evaluation_status} label="Evaluation" />
							<EvaluationStatusTag status={fetchedEvaluation?.evaluation.report_status} label="Report" />
							<EvaluationStatusTag
								status={fetchedEvaluation?.evaluation.resume_status}
								label="Resume"
								message={fetchedEvaluation?.evaluation.resume_failure_message}
							/>
							<EvaluationStatusTag
								status={fetchedEvaluation?.evaluation.cover_letter_status}
								label="Cover Letter"
								message={fetchedEvaluation?.evaluation.cover_letter_failure_message}
							/>
						</div>
					)}

					{/* Workspace controls: segmented tabs */}
					{workspaceMode && (
						<div className="pt-2">
							<div role="tablist" className="flex gap-2 border-b border-border/40 overflow-x-auto">
								{TABS.map((t) => {
									const active = tab === t.id;
									return (
										<button
											key={t.id}
											role="tab"
											aria-selected={active}
											onClick={() => onTabChange?.(t.id)}
											className={`flex items-center gap-2 border-b-2 px-4 py-3 text-sm font-semibold transition-colors whitespace-nowrap ${
												active
													? "border-primary text-foreground font-bold"
													: "border-transparent text-muted-foreground hover:border-border hover:text-foreground"
											}`}
										>
											<span>{t.label}</span>
										</button>
									);
								})}
							</div>
						</div>
					)}

					{/* Action strip */}
					<div className="flex flex-wrap items-center gap-3 pt-2">
						{showOwnedActions ? (
							<>
								{!isOwned && onSave ? (
									<button
										type="button"
										disabled={isSaving}
										onClick={() => onSave(job)}
										className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground shadow hover:bg-primary/90 transition-colors focus:outline-none focus:ring-2 focus:ring-ring"
									>
										{isSaving ? (
											<>
												<Loader2 className="h-4 w-4 animate-spin" />
												<span>Saving...</span>
											</>
										) : (
											<>
												<Bookmark className="h-4 w-4" />
												<span>Save to My Job</span>
											</>
										)}
									</button>
								) : isOwned ? (
									<div className="flex items-center gap-2.5 flex-wrap">
										<span className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-500/10 px-3 py-1.5 text-xs font-bold text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
											<Check className="h-4 w-4" />
											Saved in My Jobs
										</span>
										<Link
											href={`/user-jobs/${job.slug}?openDrawer=1`}
											className="inline-flex items-center gap-1.5 text-xs font-semibold text-primary hover:underline"
										>
											<Sparkles className="h-3.5 w-3.5 text-emerald-500" />
											<span>Open Evaluation Workspace →</span>
										</Link>
									</div>
								) : null}
							</>
						) : null}
					</div>
				</div>

				{/* Drawer Content */}
				<div className="flex-1 overflow-y-auto p-6 space-y-6">
					{workspaceMode ? (
						<>
							{tab === "details" && <JobDetailTab job={job} />}
							{tab === "report" && (
								<MatchReportTab report={resolvedReport} evalId={resolvedEvalId || undefined} loading={fetchingEvaluation} />
							)}
							{tab === "resume" && (
								<ResumeTab
									evalId={resolvedEvalId || undefined}
									resumeStatus={fetchedEvaluation?.evaluation.resume_status ?? undefined}
									reportStatus={fetchedEvaluation?.evaluation.report_status ?? undefined}
									cvImprovements={fetchedEvaluation?.evaluation.report_data?.cv_improvements ?? []}
									onRetried={refreshEvaluation}
								/>
							)}
							{tab === "cover-letter" && (
								<CoverLetterTab
									evalId={resolvedEvalId || undefined}
									status={fetchedEvaluation?.evaluation.cover_letter_status ?? undefined}
									text={fetchedEvaluation?.evaluation.cover_letter_text ?? undefined}
									failureMessage={fetchedEvaluation?.evaluation.cover_letter_failure_message ?? undefined}
									fileName={fetchedEvaluation?.evaluation.cover_letter_file_name ?? undefined}
									reportStatus={fetchedEvaluation?.evaluation.report_status ?? undefined}
									onQueued={refreshEvaluation}
								/>
							)}
							{tab === "chat" && <ApplicationChatTab job={job} />}
						</>
					) : (
						<div className="space-y-6">
							{/* Quick summary grid */}
							<div className="grid grid-cols-2 gap-4 sm:grid-cols-4 rounded-xl border border-border/60 bg-card p-4 sm:p-5">
								<div className="space-y-1">
									<span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
										<Building2 className="h-3.5 w-3.5" /> Company
									</span>
									<p className="text-sm font-bold text-foreground">{job.company}</p>
								</div>
								<div className="space-y-1">
									<span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
										<MapPin className="h-3.5 w-3.5" /> Location
									</span>
									<p className="text-sm font-bold text-foreground">{job.location}</p>
								</div>
								<div className="space-y-1">
									<span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
										<Globe className="h-3.5 w-3.5" /> Workplace
									</span>
									<p className="text-sm font-bold text-foreground">{job.is_remote ? "Remote Eligible" : "On-site / Hybrid"}</p>
								</div>
								<div className="space-y-1">
									<span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
										<Calendar className="h-3.5 w-3.5" /> Posted
									</span>
									<p className="text-sm font-bold text-foreground">
										{job.posted_date
											? new Date(job.posted_date).toLocaleDateString(undefined, {
													month: "short",
													day: "numeric",
													year: "numeric",
												})
											: "Recently"}
									</p>
								</div>
							</div>

							{(job.source === "manual" || job.source === "user_scrape" || job.source_url) && (
								<div className="flex flex-wrap items-center gap-3 rounded-xl border border-border/60 bg-card p-4 text-xs text-muted-foreground">
									<StatusTag
										label={job.source === "manual" ? "Manual" : job.source === "user_scrape" ? "Scraped" : job.source || "Board"}
										tone="info"
										testId="job-source-tag"
									/>
									{job.source_url && (
										<a
											href={job.source_url}
											target="_blank"
											rel="noreferrer"
											className="inline-flex items-center gap-1 underline"
										>
											<ExternalLink className="h-3.5 w-3.5" />
											Source listing
										</a>
									)}
								</div>
							)}

							{/* Role Description */}
							<div className="rounded-xl border border-border/60 bg-card p-6 sm:p-8">
								<div className="flex flex-col sm:flex-row sm:items-center sm:justify-between pb-6 mb-6 border-b border-border/40 gap-4">
									<div>
										<h2 className="text-lg font-bold text-foreground">Role Description</h2>
										<p className="text-xs text-muted-foreground">Official job listing text provided by {job.company}.</p>
									</div>

									{applyUrl ? (
										<a
											href={applyUrl}
											target="_blank"
											rel="noopener noreferrer"
											className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-xs font-bold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors"
										>
											<span>Apply on Company Site</span>
											<ExternalLink className="h-3.5 w-3.5" />
										</a>
									) : (
										<span className="inline-flex items-center gap-1.5 rounded-lg border border-amber-200 bg-amber-50 px-4 py-2 text-xs font-medium text-amber-800 dark:border-amber-900/30 dark:bg-amber-900/10 dark:text-amber-300">
											<AlertTriangle className="h-3 w-3" />
											<span>No application link available</span>
										</span>
									)}
								</div>

								{job.description ? (
									<div className="prose dark:prose-invert max-w-none text-sm text-foreground/90 whitespace-pre-line leading-relaxed font-sans">
										{job.description}
									</div>
								) : (
									<p className="text-sm text-muted-foreground italic py-6 text-center">
										No detailed description provided for this listing.
									</p>
								)}
							</div>
						</div>
					)}
				</div>
			</aside>
		</div>
	);
}
