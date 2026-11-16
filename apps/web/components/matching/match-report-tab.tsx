import React, { useState, useEffect } from "react";
import { MatchReport } from "@/lib/api/types";
import { MatchScoreGauge } from "./match-score-gauge";
import { StrengthsGaps } from "./strengths-gaps";
import { CategoryAccordion } from "./category-accordion";
import { CheckCircle2, XCircle, Sparkles, AlertCircle, RefreshCw } from "lucide-react";

interface MatchReportTabProps {
  evalId?: string;
  report?: MatchReport | null;
  loading?: boolean;
  onAnalyzeClick?: () => void;
  evaluating?: boolean;
}

export function MatchReportTab({
  evalId,
  report: initialReport,
  loading: initialLoading = false,
  onAnalyzeClick,
  evaluating = false,
}: MatchReportTabProps) {
  const [report, setReport] = useState<MatchReport | null>(initialReport || null);

  useEffect(() => {
    if (initialReport) {
      setReport(initialReport);
      return;
    }
  }, [initialReport]);

  if (initialLoading) {
    return (
      <div className="space-y-6 animate-pulse">
        <div className="h-32 rounded-xl bg-card" />
        <div className="h-48 rounded-xl bg-card" />
      </div>
    );
  }

  // Not evaluated yet state
  if (!evalId || !report) {
    return (
      <div className="rounded-2xl border border-dashed border-border/80 bg-card/40 p-12 text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary mb-4">
          <Sparkles className="h-6 w-6 text-emerald-500" />
        </div>
        <h3 className="text-lg font-bold text-foreground">Compatibility Evaluation Pending</h3>
        <p className="mt-2 text-sm text-muted-foreground max-w-md mx-auto">
          This job has not been evaluated against your career profile yet. Run a match analysis to generate scores, strengths, gaps, and tailored evidence.
        </p>
        {onAnalyzeClick && (
          <div className="mt-6">
            <button
              type="button"
              onClick={onAnalyzeClick}
              disabled={evaluating}
              className="inline-flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-xs font-bold text-primary-foreground shadow hover:bg-primary/90 transition-colors disabled:opacity-50"
            >
              <RefreshCw className={`h-3.5 w-3.5 ${evaluating ? "animate-spin" : ""}`} />
              <span>Analyze Compatibility Now</span>
            </button>
          </div>
        )}
      </div>
    );
  }

  const overallScore = report.overall_score ?? 0;
  const categories = report.evaluation_data?.evaluations || [];
  const mandatoryReqs = report.evaluation_data?.mandatory_requirements || [];
  const strengths = report.report_data?.strengths || [];
  const gaps = report.report_data?.gaps || [];

  return (
    <div className="space-y-6">
      {/* 1. Score Gauge */}
      <MatchScoreGauge
        score={overallScore}
        eligibilityStatus={report.eligibility_status}
        recommendationLabel={report.recommendation_label}
      />

      {/* 2. Executive Summary */}
      {report.report_data?.summary && (
        <div className="rounded-xl border border-border/60 bg-card p-6">
          <h4 className="text-sm font-bold text-foreground mb-2">Evaluation Summary</h4>
          <p className="text-sm text-foreground/90 leading-relaxed font-sans">
            {report.report_data.summary}
          </p>
        </div>
      )}

      {/* 3. Strengths and Gaps */}
      <StrengthsGaps strengths={strengths} gaps={gaps} />

      {/* 4. Mandatory Requirements Checklist */}
      {mandatoryReqs.length > 0 && (
        <div className="rounded-xl border border-border/60 bg-card p-6 space-y-3">
          <h4 className="text-sm font-bold text-foreground">Mandatory Requirements</h4>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {mandatoryReqs.map((req, idx) => {
              const isMet = req.status === "met";
              return (
                <div
                  key={idx}
                  className={`flex items-start gap-2.5 rounded-lg border p-3 text-xs ${
                    isMet
                      ? "border-emerald-500/20 bg-emerald-500/5 text-foreground"
                      : "border-destructive/20 bg-destructive/5 text-destructive"
                  }`}
                >
                  {isMet ? (
                    <CheckCircle2 className="h-4 w-4 text-emerald-500 shrink-0 mt-0.5" />
                  ) : (
                    <XCircle className="h-4 w-4 text-destructive shrink-0 mt-0.5" />
                  )}
                  <div>
                    <span className="font-semibold block">{req.requirement}</span>
                    <span className="text-[11px] text-muted-foreground">{req.reason}</span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* 5. Category Breakdown with Evidence */}
      <CategoryAccordion categories={categories} />
    </div>
  );
}
