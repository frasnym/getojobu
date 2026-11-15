import React from "react";
import { CategoryEvidence } from "@/lib/api/types";
import { Check, ArrowRightLeft } from "lucide-react";

interface EvidenceListProps {
  evidence: CategoryEvidence[];
}

export function EvidenceList({ evidence }: EvidenceListProps) {
  if (!evidence || evidence.length === 0) {
    return null;
  }

  return (
    <div className="mt-3 space-y-2.5">
      <h5 className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
        Evidence Citations & Proof
      </h5>
      <div className="space-y-2">
        {evidence.map((item, idx) => {
          const isDirect = item.relationship === "direct";
          return (
            <div
              key={idx}
              className="rounded-lg border border-border/40 bg-background/50 p-3 text-xs space-y-1.5"
            >
              <div className="flex items-center justify-between gap-2">
                <span className="font-semibold text-foreground">
                  Job Requirement: {item.job_requirement}
                </span>
                <span
                  className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wider ${
                    isDirect
                      ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                      : "bg-blue-500/10 text-blue-600 dark:text-blue-400"
                  }`}
                >
                  {isDirect ? <Check className="h-3 w-3" /> : <ArrowRightLeft className="h-3 w-3" />}
                  {item.relationship}
                </span>
              </div>
              <p className="text-muted-foreground pl-3 border-l-2 border-primary/30">
                <span className="font-medium text-foreground/80">Candidate Evidence: </span>
                {item.candidate_evidence}
              </p>
            </div>
          );
        })}
      </div>
    </div>
  );
}
