import React from "react";
import { AlertCircle, AlertTriangle, CheckCircle2, Clock } from "lucide-react";

/**
 * Small status tags for a user job's evaluation, rendered on the
 * user-jobs list card. The eligibility / recommendation pills mirror
 * MatchScoreGauge on the user-job-detail page so both surfaces agree.
 *
 * Each status tag is rendered only when its value is non-null.
 */

interface EvaluationStatusTagProps {
  status: string | null | undefined;
  label?: string;
}

const evaluationTagStyle = (status: string) => {
  switch (status.toLowerCase()) {
    case "success":
    case "analyzed":
      return {
        icon: <CheckCircle2 className="h-3 w-3" />,
        className: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
      };
    case "pending":
    case "analyze_pending":
      return {
        icon: <Clock className="h-3 w-3" />,
        className: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
      };
    case "queued":
    case "queued_retry":
      return {
        icon: <Clock className="h-3 w-3" />,
        className: "bg-sky-500/10 text-sky-600 dark:text-sky-400",
      };
    case "failed":
      return {
        icon: <AlertCircle className="h-3 w-3" />,
        className: "bg-destructive/10 text-destructive dark:text-destructive",
      };
    default:
      return {
        icon: null,
        className: "bg-secondary/50 text-secondary-foreground dark:text-secondary-foreground",
      };
  }
};

export function EvaluationStatusTag({ status, label = "Evaluation" }: EvaluationStatusTagProps) {
  if (!status) return null;
  const { icon, className } = evaluationTagStyle(status);
  return (
    <span
      data-testid="evaluation-status-tag"
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-bold capitalize ${className}`}
    >
      {icon}
      <span>{label}</span>
    </span>
  );
}

interface EligibilityTagProps {
  status: string | null | undefined;
}

export function EligibilityTag({ status }: EligibilityTagProps) {
  if (!status) return null;
  const isEligible = status.toLowerCase() === "eligible";
  return (
    <span
      data-testid="eligibility-tag"
      className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-bold capitalize ${isEligible ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400" : "bg-amber-500/10 text-amber-600 dark:text-amber-400"}`}
    >
      {isEligible ? (
        <CheckCircle2 className="h-3 w-3" />
      ) : (
        <AlertTriangle className="h-3 w-3" />
      )}
      {status.replace("_", " ")}
    </span>
  );
}

const recommendationTagStyle = (label: string) => {
  switch (label.toLowerCase()) {
    case "strong_match":
      return { className: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400" };
    case "worth_applying":
      return { className: "bg-blue-500/10 text-blue-600 dark:text-blue-400" };
    case "possible_match":
      return { className: "bg-amber-500/10 text-amber-600 dark:text-amber-400" };
    case "low_match":
      return { className: "bg-destructive/10 text-destructive dark:text-destructive" };
    default:
      return { className: "bg-secondary/50 text-secondary-foreground dark:text-secondary-foreground" };
  }
};

interface RecommendationTagProps {
  label: string | null | undefined;
}

export function RecommendationTag({ label }: RecommendationTagProps) {
  if (!label) return null;
  const { className } = recommendationTagStyle(label);
  return (
    <span
      data-testid="recommendation-tag"
      className={`rounded-full px-2.5 py-0.5 text-xs font-semibold capitalize ${className}`}
    >
      {label.replace("_", " ")}
    </span>
  );
}
