import React from "react";
import { CheckCircle2, AlertCircle } from "lucide-react";

interface StrengthsGapsProps {
  strengths?: string[];
  gaps?: string[];
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
            {strengths.map((str, idx) => (
              <li key={idx} className="flex items-start gap-2">
                <span className="text-emerald-500 font-bold">•</span>
                <span>{str}</span>
              </li>
            ))}
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
          <ul className="space-y-2 text-xs text-foreground/90">
            {gaps.map((gap, idx) => (
              <li key={idx} className="flex items-start gap-2">
                <span className="text-amber-500 font-bold">•</span>
                <span>{gap}</span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-xs text-muted-foreground italic">No significant gaps detected.</p>
        )}
      </div>
    </div>
  );
}
