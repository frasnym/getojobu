import React from "react";
import { CheckCircle2, AlertTriangle, Sparkles } from "lucide-react";

interface MatchScoreGaugeProps {
  score: number;
  eligibilityStatus?: string;
  recommendationLabel?: string;
}

export function MatchScoreGauge({
  score,
  eligibilityStatus,
  recommendationLabel,
}: MatchScoreGaugeProps) {
  const isEligible = eligibilityStatus === "eligible";
  const recommendation = recommendationLabel?.replace("_", " ");

  return (
    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-6 rounded-xl border border-border/60 bg-card p-6">
      <div className="flex items-center gap-5">
        {/* Radial / Circular Score Display */}
        <div className="relative flex h-20 w-20 shrink-0 items-center justify-center rounded-full border-4 border-emerald-500/20 bg-background shadow-inner">
          <svg className="absolute inset-0 h-full w-full -rotate-90" viewBox="0 0 36 36">
            <path
              className="text-muted/20"
              strokeWidth="3.5"
              stroke="currentColor"
              fill="none"
              d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
            />
            <path
              className={isEligible ? "text-emerald-500" : "text-amber-500"}
              strokeDasharray={`${Math.min(100, Math.max(0, score))}, 100`}
              strokeWidth="3.5"
              strokeLinecap="round"
              stroke="currentColor"
              fill="none"
              d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
            />
          </svg>
          <span className="text-xl font-black tracking-tight text-foreground">
            {Number.isInteger(score) ? score : score.toFixed(1)}%
          </span>
        </div>

        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span
              className={`inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-bold ${
                isEligible
                  ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                  : "bg-amber-500/10 text-amber-600 dark:text-amber-400"
              }`}
            >
              {isEligible ? (
                <CheckCircle2 className="h-3.5 w-3.5" />
              ) : (
                <AlertTriangle className="h-3.5 w-3.5" />
              )}
              <span className="capitalize">{eligibilityStatus || "Eligible"}</span>
            </span>

            {recommendation && (
              <span className="rounded-full bg-secondary px-2.5 py-0.5 text-xs font-semibold text-secondary-foreground capitalize">
                {recommendation}
              </span>
            )}
          </div>
          <h3 className="text-base font-bold text-foreground">Candidate-Job Compatibility</h3>
          <p className="text-xs text-muted-foreground">
            Evaluated against verified experience, technical skills, and role criteria.
          </p>
        </div>
      </div>

      <div className="flex flex-col justify-center sm:items-end border-t sm:border-t-0 sm:border-l border-border/40 pt-4 sm:pt-0 sm:pl-6 text-xs text-muted-foreground space-y-1">
        <span className="font-semibold text-foreground">Scoring Methodology</span>
        <span>Weighted evaluation of core mastery, reliability, and requirements</span>
      </div>
    </div>
  );
}
