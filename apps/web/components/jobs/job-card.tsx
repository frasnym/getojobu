import React from "react";
import Link from "next/link";
import { UserJob } from "@/lib/api/types";
import { AlertCircle, MapPin, Globe, Clock, ArrowRight, Eye, EyeOff, Download } from "lucide-react";
import { EvaluationStatusTag, EligibilityTag, RecommendationTag } from "@/components/jobs/evaluation-tags";

interface JobCardProps {
	userJob: UserJob;
	href?: string;
	onSelect?: (userJob: UserJob) => void;
	isSelected?: boolean;
	onHideToggle?: (userJobId: string, isHidden: boolean) => void | Promise<void>;
	/** Open the drawer on the Resume tab for this job. */
	onResume?: (userJob: UserJob) => void;
	/** When false, the job cannot open the detail view (drawer or link). */
	canOpenDetail?: boolean;
	/** When true, renders a checkbox for bulk selection. */
	selectable?: boolean;
	/** Whether this card's checkbox is checked. */
	isChecked?: boolean;
	/** Called when the checkbox toggles. */
	onCheckChange?: (checked: boolean) => void;
	/** Called when the user retries a failed job analysis. */
	onRetry?: (userJobId: string) => void | Promise<void>;
}

/** Turn a raw status like "analyze_pending" into "Analyze Pending". */
function formatJobStatusLabel(status: string): string {
	return status
		.replace(/_/g, " ")
		.replace(/\b\w/g, (char) => char.toUpperCase());
}

export function JobCard({
	userJob,
	href,
	onSelect,
	isSelected,
	onHideToggle,
	onResume,
	onRetry,
	canOpenDetail = true,
	selectable = false,
	isChecked = false,
	onCheckChange,
}: JobCardProps) {
	const job = userJob.job;
	const targetHref = href || `/user-jobs/${job.slug}`;
	const isClickable = !!onSelect && canOpenDetail;
	const isHidden = userJob.is_hidden === true;
	const canRetry =
		job.status === "failed" || userJob.evaluation?.evaluation_status === "failed";

	return (
		<div
			onClick={isClickable ? () => onSelect(userJob) : undefined}
			className={`group relative rounded-xl border p-6 pb-12 transition-all duration-300 ${
				isSelected
					? "border-primary bg-primary/5 shadow-md shadow-primary/5 ring-1 ring-primary/30"
					: isHidden
						? "border-border/30 bg-muted/40 opacity-60 grayscale hover:border-border/40"
						: isChecked
							? "border-primary bg-primary/5 hover:border-primary"
							: "border-border/60 bg-card hover:border-border hover:shadow-md hover:shadow-primary/5"
			} ${isClickable ? "cursor-pointer" : ""}`}
		>
			<div className="flex flex-col gap-5 sm:flex-row sm:items-start sm:justify-between">
				<div className="space-y-2 flex-1">
					<div className="flex flex-wrap items-center gap-2">
						<span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">{job.company}</span>
						{job.is_remote && (
							<span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2 py-0.5 text-[11px] font-medium text-emerald-500">
								<Globe className="h-3 w-3" />
								Remote
							</span>
						)}
						{isHidden && (
							<span className="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium border-muted-foreground/40 bg-muted/30 text-muted-foreground">
								<EyeOff className="h-2.5 w-2.5" />
								Hidden
							</span>
						)}
					</div>

					<h3 className="text-lg font-bold tracking-tight text-foreground group-hover:text-primary transition-colors">
						{isClickable ? (
							<span className="group-hover:text-primary transition-colors">{job.title}</span>
						) : canOpenDetail ? (
							<Link href={targetHref} className="focus:outline-none">
								<span className="absolute inset-0" aria-hidden="true" />
								{job.title}
							</Link>
						) : (
							<span>{job.title}</span>
						)}
					</h3>

					<div className="flex flex-wrap items-center gap-5 text-xs text-muted-foreground pt-1.5">
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

					{(job.status || userJob.evaluation) && (
						<div className="flex flex-wrap items-center gap-2.5 pt-3 border-t border-border/40">
							<EvaluationStatusTag
								status={job.status}
								label={formatJobStatusLabel(job.status)}
							/>
							{userJob.evaluation && (
								<>
									<EvaluationStatusTag status={userJob.evaluation.evaluation_status} label="Evaluation" />
									<EvaluationStatusTag status={userJob.evaluation.report_status} label="Report" />
									<EvaluationStatusTag status={userJob.evaluation.resume_status} label="Resume" />
									<EligibilityTag status={userJob.evaluation.eligibility_status} />
									<RecommendationTag label={userJob.evaluation.recommendation_label} />
								</>
							)}
						</div>
					)}
				</div>

				{/* Actions */}
				<div className="flex shrink-0 items-center sm:flex-col sm:items-end justify-between pt-2 sm:pt-0">
					<div className="flex items-center gap-2">
						{onHideToggle && userJob.id && (
							<button
								type="button"
								onClick={(e) => {
									e.stopPropagation();
									onHideToggle(userJob.id, !userJob.is_hidden);
								}}
								className="inline-flex items-center gap-1 rounded-lg border border-border/60 bg-card px-2.5 py-1.5 text-xs font-medium text-foreground hover:bg-secondary transition-colors focus:outline-none focus:ring-2 focus:ring-ring"
								aria-label={userJob.is_hidden ? "Unhide job" : "Hide job"}
							>
								{userJob.is_hidden ? (
									<>
										<EyeOff className="h-3.5 w-3.5" />
										<span>Unhide</span>
									</>
								) : (
									<>
										<Eye className="h-3.5 w-3.5" />
										<span>Hide</span>
									</>
								)}
							</button>
						)}
						{canOpenDetail && (
							<div className="flex items-center gap-1 text-xs font-medium text-muted-foreground group-hover:text-foreground group-hover:translate-x-0.5 transition-all">
								<span>Details</span>
								<ArrowRight className="h-3.5 w-3.5" />
							</div>
						)}
						{onResume && userJob.evaluation?.resume_status === "success" && (
							<button
								type="button"
								onClick={(e) => {
									e.stopPropagation();
									onResume(userJob);
								}}
								className="inline-flex items-center gap-1.5 rounded-lg border border-border/60 bg-card px-2.5 py-1.5 text-xs font-medium text-foreground hover:bg-secondary transition-colors focus:outline-none focus:ring-2 focus:ring-ring"
								aria-label="Download tailored resume"
							>
								<Download className="h-3.5 w-3.5" />
								<span>Resume</span>
							</button>
						)}
						{canRetry && onRetry && (
							<button
								type="button"
								onClick={(e) => {
									e.stopPropagation();
									onRetry(userJob.id);
								}}
								className="inline-flex items-center gap-1.5 rounded-lg border border-red-500/40 bg-card px-2.5 py-1.5 text-xs font-medium text-red-400 dark:text-red-300 hover:bg-red-500/10 transition-colors focus:outline-none focus:ring-2 focus:ring-red-500/60"
								aria-label="Retry failed job"
							>
								<AlertCircle className="h-3.5 w-3.5" />
								<span>Retry</span>
							</button>
						)}
					</div>
				</div>

				{selectable && (
					<label
						className="absolute bottom-4 right-4 inline-flex items-center cursor-pointer shrink-0"
						onClick={(e) => e.stopPropagation()}
					>
						<input
							type="checkbox"
							checked={isChecked}
							onChange={(e) => onCheckChange?.(e.target.checked)}
							className="h-4 w-4 rounded border-border text-primary focus:ring-primary cursor-pointer"
							aria-label={`Select ${job.title}`}
						/>
					</label>
				)}
			</div>
		</div>
	);
}
