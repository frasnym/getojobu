import React from "react";
import { AlertCircle, AlertTriangle, CheckCircle2, Clock } from "lucide-react";

/**
 * Small status tags for a user job's evaluation, rendered on the
 * user-jobs list card. The eligibility / recommendation pills mirror
 * MatchScoreGauge on the user-job-detail page so both surfaces agree.
 *
 * Each status tag is rendered only when its value is non-null.
 */

type TagTone = "success" | "pending" | "failed" | "warning" | "info" | "neutral";

const tagToneClass: Record<TagTone, string> = {
	success: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
	pending: "bg-sky-500/10 text-sky-600 dark:text-sky-400",
	failed: "bg-red-500/10 text-red-400 dark:text-red-300",
	warning: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
	info: "bg-blue-500/10 text-blue-600 dark:text-blue-400",
	neutral: "bg-secondary/50 text-secondary-foreground dark:text-secondary-foreground",
};

interface StatusTagProps {
	label: string;
	tone: TagTone;
	icon?: React.ReactNode;
	title?: string;
	testId?: string;
}

/** Single pill style shared by every status, eligibility, recommendation and remote tag. */
export function StatusTag({ label, tone, icon, title, testId }: StatusTagProps) {
	return (
		<span
			data-testid={testId}
			className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-bold capitalize ${tagToneClass[tone]}`}
			title={title}
		>
			{icon}
			{label}
		</span>
	);
}

interface EvaluationStatusTagProps {
	status: string | null | undefined;
	label?: string;
	/** Optional detail shown as a tooltip when the status is "failed". */
	message?: string;
}

/** Turn a raw status like "analyze_pending" into "Analyze Pending". */
export function formatStatusLabel(status: string): string {
	return status.replace(/_/g, " ").replace(/\b\w/g, (char) => char.toUpperCase());
}

const evaluationTagStyle = (status: string): { tone: TagTone; icon: React.ReactNode } => {
	switch (status.toLowerCase()) {
		case "success":
		case "analyzed":
			return { tone: "success", icon: <CheckCircle2 className="h-3 w-3" /> };
		case "pending":
		case "analyze_pending":
			return { tone: "pending", icon: <Clock className="h-3 w-3" /> };
		case "queued":
		case "queued_retry":
			return { tone: "pending", icon: <Clock className="h-3 w-3" /> };
		case "failed":
			return { tone: "failed", icon: <AlertCircle className="h-3 w-3" /> };
		default:
			return { tone: "neutral", icon: null };
	}
};

export function EvaluationStatusTag({ status, label, message }: EvaluationStatusTagProps) {
	if (!status) return null;
	const { tone, icon } = evaluationTagStyle(status);
	const isFailed = status.toLowerCase() === "failed";
	return (
		<StatusTag
			testId="evaluation-status-tag"
			label={label ?? formatStatusLabel(status)}
			tone={tone}
			icon={icon}
			title={isFailed && message ? message : undefined}
		/>
	);
}

interface EligibilityTagProps {
	status: string | null | undefined;
}

export function EligibilityTag({ status }: EligibilityTagProps) {
	if (!status) return null;
	const isEligible = status.toLowerCase() === "eligible";
	return (
		<StatusTag
			testId="eligibility-tag"
			label={status.replace("_", " ")}
			tone={isEligible ? "success" : "warning"}
			icon={isEligible ? <CheckCircle2 className="h-3 w-3" /> : <AlertTriangle className="h-3 w-3" />}
		/>
	);
}

const recommendationTone = (label: string): TagTone => {
	switch (label.toLowerCase()) {
		case "strong_match":
			return "success";
		case "worth_applying":
			return "info";
		case "possible_match":
			return "warning";
		case "low_match":
			return "failed";
		default:
			return "neutral";
	}
};

interface RecommendationTagProps {
	label: string | null | undefined;
}

export function RecommendationTag({ label }: RecommendationTagProps) {
	if (!label) return null;
	return (
		<StatusTag
			testId="recommendation-tag"
			label={label.replace("_", " ")}
			tone={recommendationTone(label)}
		/>
	);
}
