import React, { useState } from "react";
import { CategoryEvaluation } from "@/lib/api/types";
import { EvidenceList } from "./evidence-list";
import { ChevronDown, ChevronUp } from "lucide-react";

interface CategoryAccordionProps {
  categories: CategoryEvaluation[];
}

export function CategoryAccordion({ categories }: CategoryAccordionProps) {
  const [openIndex, setOpenIndex] = useState<number | null>(0); // First open by default

  if (!categories || categories.length === 0) {
    return null;
  }

  const toggle = (idx: number) => {
    setOpenIndex(openIndex === idx ? null : idx);
  };

  return (
    <div className="space-y-3">
      <h4 className="text-sm font-bold text-foreground">Scoring Categories & Breakdown</h4>
      <div className="space-y-3">
        {categories.map((cat, idx) => {
          const isOpen = openIndex === idx;
          const score = cat.score;
          return (
            <div
              key={idx}
              className="rounded-xl border border-border/60 bg-card transition-all"
            >
              <button
                type="button"
                onClick={() => toggle(idx)}
                className="w-full flex items-center justify-between p-4 text-left focus:outline-none"
              >
                <div className="space-y-1 flex-1 pr-4">
                  <div className="flex items-center gap-3">
                    <span className="text-sm font-bold text-foreground">{cat.category}</span>
                    <span className="text-xs font-semibold text-emerald-500">{score}%</span>
                  </div>
                  {/* Progress bar */}
                  <div className="h-1.5 w-full max-w-xs rounded-full bg-secondary overflow-hidden">
                    <div
                      className="h-full bg-emerald-500 rounded-full transition-all duration-300"
                      style={{ width: `${Math.min(100, Math.max(0, score))}%` }}
                    />
                  </div>
                </div>

                <div className="text-muted-foreground">
                  {isOpen ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                </div>
              </button>

              {isOpen && (
                <div className="px-4 pb-4 pt-1 border-t border-border/30 space-y-3 text-xs">
                  <p className="text-foreground/90 leading-relaxed">{cat.reason}</p>
                  {cat.evidence && <EvidenceList evidence={cat.evidence} />}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
