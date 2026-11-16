import React from "react";
import { AlertCircle, AlertTriangle, CheckCircle2 } from "lucide-react";

/**
 * Small status tags for a user job's evaluation, rendered on the
 * user-jobs list card. The eligibility / recommendation pills mirror
 * MatchScoreGauge on the user-job-detail page so both surfaces agree.
 */

interface EvaluationStatusTagProps {
  status: string | null | undefined;
}

const evaluationTagStyle = (status: string) => {
  switch (status.toLowerCase()) {
    case "success":
      return {
        icon: <CheckCircle2 className="h-3 w-3" />,
        className: "border-emerald-500/20 bg-emerald-500/10 text-emerald-600",
      };
    case "pending":
      return {
        icon: <AlertTriangle className="h-3 w-3" />,
        className: "border-amber-500/20 bg-amber-500/10 text-amber-600",
      };
    case "failed":
      return {
        icon: <AlertCircle className="h-3 w-3" />,
        className: "border-destructive/20 bg-destructive/10 text-destructive",
      };
    default:
      return {
        icon: null,
        className: "border-border/40 bg-muted/50 text-muted-foreground",
      };
  }
};

export function EvaluationStatusTag({ status }: EvaluationStatusTagProps) {
  if (!status) return null;
  const { icon, className } = evaluationTagStyle(status);
  return (
    <span
      data-testid="evaluation-status-tag"
      className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-bold capitalize ${className}`}
    >
      {icon}
      <span>Evaluation</span>
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

interface RecommendationTagProps {
  label: string | null | undefined;
}

export function RecommendationTag({ label }: RecommendationTagProps) {
  if (!label) return null;
  return (
    <span
      data-testid="recommendation-tag"
      className="rounded-full bg-secondary px-2.5 py-0.5 text-xs font-semibold text-secondary-foreground capitalize"
    >
      {label.replace("_", " ")}
    </span>
  );
}
