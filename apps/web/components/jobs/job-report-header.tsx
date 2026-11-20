import React from "react";
import Link from "next/link";
import { Job, MatchReport } from "@/lib/api/types";
import { ArrowLeft, ChevronLeft, ChevronRight, Globe, MapPin, Clock, Eye, EyeOff, RefreshCw } from "lucide-react";
import { EvaluationStatusTag, EligibilityTag, RecommendationTag, StatusTag } from "@/components/jobs/evaluation-tags";

export type ReportTab = "details" | "report" | "resume" | "cover-letter";

interface JobReportHeaderProps {
	job: Job;
	activeTab: ReportTab;
	onTabChange: (tab: ReportTab) => void;
	prevSlug?: string;
	nextSlug?: string;
	basePath?: string;
	/** When true, the whole header greys out (user hidden the job). */
	isHidden?: boolean;
	/** The user↔job link id. Required to show the Hide/Unhide button. */
	userJobId?: string;
	/** Toggle hide on the user↔job link (bulk endpoint). Receives (userJobId, isHidden). */
	onHideToggle?: (userJobId: string, isHidden: boolean) => void | Promise<void>;
	/** Latest evaluation, used for the card-style tags in the header. */
	evaluation?: MatchReport;
	/** Manually refetch the job detail and the latest evaluation. */
	onRefresh?: () => void | Promise<void>;
	/** Disables the Refresh button while a refetch is in progress. */
	refreshing?: boolean;
}

export function JobReportHeader({
	job,
	activeTab,
	onTabChange,
	prevSlug,
	nextSlug,
	basePath = "/user-jobs",
	isHidden = false,
	userJobId,
	onHideToggle,
	evaluation,
	onRefresh,
	refreshing = false,
}: JobReportHeaderProps) {
	const tabs: { id: ReportTab; label: string; badge?: string }[] = [
		{ id: "details", label: "Job Details" },
		{ id: "report", label: "Match Report" },
		{ id: "resume", label: "Tailored Resume" },
		{ id: "cover-letter", label: "Cover Letter" },
	];

	return (
		<div className={`border-b border-border/40 bg-card/60 backdrop-blur pb-0 pt-6 ${isHidden ? "opacity-60 grayscale" : ""}`}>
			<div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 space-y-6">
				{/* Navigation & Controls */}
				<div className="flex items-center justify-between">
					<Link
						href={basePath}
						className="inline-flex items-center gap-1.5 text-xs font-semibold text-muted-foreground transition-colors hover:text-foreground"
					>
						<ArrowLeft className="h-3.5 w-3.5" />
						<span>Back to My Jobs</span>
					</Link>

					{/* Manual Refresh and sequential Prev/Next quick switcher */}
					<div className="flex items-center gap-1 text-xs">
						{onRefresh && (
							<button
								type="button"
								onClick={() => onRefresh()}
								disabled={refreshing}
								aria-label="Refresh"
								className="mr-2 inline-flex items-center gap-1 rounded-md border border-border px-2.5 py-1 font-medium text-foreground transition-colors hover:bg-secondary disabled:opacity-50"
							>
								<RefreshCw className={`h-3.5 w-3.5 ${refreshing ? "animate-spin" : ""}`} />
								<span>{refreshing ? "Refreshing..." : "Refresh"}</span>
							</button>
						)}

						{prevSlug ? (
							<Link
								href={`${basePath}/${prevSlug}`}
								className="inline-flex items-center gap-1 rounded-md border border-border px-2.5 py-1 font-medium text-foreground hover:bg-secondary transition-colors"
								title="Previous job"
							>
								<ChevronLeft className="h-3.5 w-3.5" />
								<span>Prev</span>
							</Link>
						) : (
							<span className="inline-flex items-center gap-1 rounded-md border border-border/40 px-2.5 py-1 text-muted-foreground/40 cursor-not-allowed">
								<ChevronLeft className="h-3.5 w-3.5" />
								<span>Prev</span>
							</span>
						)}

						{nextSlug ? (
							<Link
								href={`${basePath}/${nextSlug}`}
								className="inline-flex items-center gap-1 rounded-md border border-border px-2.5 py-1 font-medium text-foreground hover:bg-secondary transition-colors"
								title="Next job"
							>
								<span>Next</span>
								<ChevronRight className="h-3.5 w-3.5" />
							</Link>
						) : (
							<span className="inline-flex items-center gap-1 rounded-md border border-border/40 px-2.5 py-1 text-muted-foreground/40 cursor-not-allowed">
								<span>Next</span>
								<ChevronRight className="h-3.5 w-3.5" />
							</span>
						)}
					</div>
				</div>

				{/* Same four rows as the job card: tags, title + hide, meta, status tags. */}
				<div className="space-y-2">
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
						<EligibilityTag status={evaluation?.eligibility_status} />
						<RecommendationTag label={evaluation?.recommendation_label} />
					</div>

					<div className="flex flex-wrap items-center gap-x-3 gap-y-2">
						<h1 className={`text-2xl sm:text-3xl font-black tracking-tight ${isHidden ? "text-muted-foreground" : "text-foreground"}`}>
							{job.title}
						</h1>
						{onHideToggle && userJobId && (
							<button
								type="button"
								onClick={() => onHideToggle(userJobId, !isHidden)}
								className="inline-flex items-center gap-1 rounded-lg border border-border/60 bg-card px-2 py-1 text-xs font-medium text-foreground transition-colors hover:bg-secondary focus:outline-none focus:ring-2 focus:ring-ring"
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

					{(job.status || evaluation) && (
						<div className="flex flex-wrap items-center gap-2.5 border-t border-border/40 pt-3">
							<EvaluationStatusTag status={job.status} />
							{evaluation && (
								<>
									<EvaluationStatusTag status={evaluation.evaluation_status} label="Evaluation" />
									<EvaluationStatusTag status={evaluation.report_status} label="Report" />
									<EvaluationStatusTag
										status={evaluation.resume_status}
										label="Resume"
										message={evaluation.resume_failure_message}
									/>
									<EvaluationStatusTag
										status={evaluation.cover_letter_status}
										label="Cover Letter"
										message={evaluation.cover_letter_failure_message}
									/>
								</>
							)}
						</div>
					)}
				</div>

				{/* Segmented Workspace Tabs */}
				<div role="tablist" className="flex gap-2 border-b border-border/40 overflow-x-auto">
					{tabs.map((tab) => {
						const active = activeTab === tab.id;
						return (
							<button
								key={tab.id}
								role="tab"
								aria-selected={active}
								onClick={() => onTabChange(tab.id)}
								className={`flex items-center gap-2 border-b-2 px-4 py-3 text-sm font-semibold transition-colors whitespace-nowrap ${
									active
										? "border-primary text-foreground font-bold"
										: "border-transparent text-muted-foreground hover:border-border hover:text-foreground"
								}`}
							>
								<span>{tab.label}</span>
								{tab.badge && (
									<span
										className={`rounded-full px-1.5 py-0.2 text-[10px] font-bold ${
											active ? "bg-primary text-primary-foreground" : "bg-secondary text-secondary-foreground"
										}`}
									>
										{tab.badge}
									</span>
								)}
							</button>
						);
					})}
				</div>
			</div>
		</div>
	);
}
