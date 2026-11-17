import React from "react";
import Link from "next/link";
import { Job } from "@/lib/api/types";
import { ArrowLeft, ChevronLeft, ChevronRight, Globe, MapPin, Eye, EyeOff } from "lucide-react";

export type ReportTab = "details" | "report" | "resume";

interface JobReportHeaderProps {
	job: Job;
	activeTab: ReportTab;
	onTabChange: (tab: ReportTab) => void;
	prevSlug?: string;
	nextSlug?: string;
	basePath?: string;
	/** When true, the job title greys out (user hidden the job). */
	isHidden?: boolean;
	/** The user↔job link id. Required to show the Hide/Unhide button. */
	userJobId?: string;
	/** Toggle hide on the user↔job link (bulk endpoint). Receives (userJobId, isHidden). */
	onHideToggle?: (userJobId: string, isHidden: boolean) => void | Promise<void>;
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
}: JobReportHeaderProps) {
	const tabs: { id: ReportTab; label: string; badge?: string }[] = [
		{ id: "details", label: "Job Details" },
		{ id: "report", label: "Match Report" },
		{ id: "resume", label: "Tailored Resume" },
	];

	return (
		<div className="border-b border-border/40 bg-card/60 backdrop-blur pb-0 pt-6">
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

					{/* Sequential Prev/Next Quick Switcher */}
					<div className="flex items-center gap-1 text-xs">
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

				{/* Title, Metadata & Action Header */}
				<div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
					<div className="space-y-1.5 flex-1">
						<div className="flex flex-wrap items-center gap-2">
							<span className="text-xs font-bold uppercase tracking-wider text-muted-foreground">{job.company}</span>
							{job.is_remote && (
								<span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2 py-0.5 text-[11px] font-semibold text-emerald-500">
									<Globe className="h-3 w-3" />
									Remote
								</span>
							)}
							<span className="flex items-center gap-1 text-xs text-muted-foreground">
								<MapPin className="h-3.5 w-3.5" />
								{job.location}
							</span>
						</div>

						<h1 className={`text-2xl sm:text-3xl font-black tracking-tight ${isHidden ? "text-muted-foreground" : "text-foreground"}`}>
							{job.title}
						</h1>
					</div>

					{onHideToggle && userJobId && (
						<button
							type="button"
							onClick={() => onHideToggle(userJobId, !isHidden)}
							className={`inline-flex shrink-0 items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-semibold transition-colors ${
								isHidden
									? "border-red-500/40 text-red-400 dark:text-red-300 hover:bg-red-500/10"
									: "border-border/60 text-muted-foreground hover:bg-secondary hover:text-foreground"
							}`}
							aria-label={isHidden ? "Unhide job" : "Hide job"}
						>
							{isHidden ? <Eye className="h-3.5 w-3.5" /> : <EyeOff className="h-3.5 w-3.5" />}
							{isHidden ? "Unhide" : "Hide"}
						</button>
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
