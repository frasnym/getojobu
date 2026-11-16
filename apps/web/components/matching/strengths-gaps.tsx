import React from "react";
import { CheckCircle2, AlertCircle } from "lucide-react";
import { ReportGap } from "@/lib/api/types";

interface StrengthsGapsProps {
  strengths?: string[];
  gaps?: ReportGap[];
}

export function StrengthsGaps({ strengths = [], gaps = [] }: StrengthsGapsProps) {
  if (strengths.length === 0 && gaps.length === 0) {
    return null;
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
      {/* Strengths */}
      <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/5 p-5 space-y-3">
        <h4 className="text-sm font-bold text-emerald-600 dark:text-emerald-400 flex items-center gap-2">
          <CheckCircle2 className="h-4 w-4" />
          Key Strengths & Differentiators
        </h4>
        {strengths.length > 0 ? (
          <ul className="space-y-2 text-xs text-foreground/90">
            {strengths.map((str, idx) => {
              const text =
                typeof str === "string"
                  ? str
                  : str && typeof str === "object"
                  ? (str as any).strength || (str as any).title || JSON.stringify(str)
                  : String(str);
              return (
                <li key={idx} className="flex items-start gap-2">
                  <span className="text-emerald-500 font-bold shrink-0 mt-0.5">•</span>
                  <span>{text}</span>
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="text-xs text-muted-foreground italic">No specific strengths highlighted.</p>
        )}
      </div>

      {/* Gaps */}
      <div className="rounded-xl border border-amber-500/20 bg-amber-500/5 p-5 space-y-3">
        <h4 className="text-sm font-bold text-amber-600 dark:text-amber-400 flex items-center gap-2">
          <AlertCircle className="h-4 w-4" />
          Areas to Address or Improve
        </h4>
        {gaps.length > 0 ? (
          <ul className="space-y-2.5 text-xs text-foreground/90">
            {gaps.map((gap, idx) => {
              if (typeof gap === "string") {
                return (
                  <li key={idx} className="flex items-start gap-2">
                    <span className="text-amber-500 font-bold shrink-0 mt-0.5">•</span>
                    <span>{gap}</span>
                  </li>
                );
              }

              const statusLabel =
                gap.status === "not_found_in_cv"
                  ? "Not found in CV"
                  : gap.status === "partially_met"
                  ? "Partially met"
                  : gap.status === "met"
                  ? "Met"
                  : gap.status
                  ? gap.status.replace(/_/g, " ")
                  : null;

              return (
                <li key={idx} className="flex items-start gap-2">
                  <span className="text-amber-500 font-bold shrink-0 mt-0.5">•</span>
                  <div className="space-y-0.5">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-semibold text-foreground">{gap.requirement}</span>
                      {statusLabel && (
                        <span className="rounded bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-300">
                          {statusLabel}
                        </span>
                      )}
                    </div>
                    {gap.suggestion && (
                      <p className="text-[11px] text-muted-foreground leading-relaxed">
                        {gap.suggestion}
                      </p>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="text-xs text-muted-foreground italic">No significant gaps detected.</p>
        )}
      </div>
    </div>
  );
}
