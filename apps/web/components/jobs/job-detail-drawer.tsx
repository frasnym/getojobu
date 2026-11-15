"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { Job } from "@/lib/api/types";
import { ReportTab } from "@/components/jobs/job-report-header";
import { JobDetailTab } from "@/components/jobs/job-detail-tab";
import { MatchReportTab } from "@/components/matching/match-report-tab";
import { ResumeTab } from "@/components/resume/resume-tab";
import {
  X,
  MapPin,
  Globe,
  Clock,
  Bookmark,
  Check,
  AlertTriangle,
  ExternalLink,
  Loader2,
  Sparkles,
  RefreshCw,
} from "lucide-react";
import { statusBadgeClasses } from "@/hooks/use-jobs";

interface JobDetailDrawerProps {
  job: Job | null;
  isOpen: boolean;
  onClose: () => void;
  onSave?: (job: Job) => void;
  isSaving?: boolean;
  // Workspace mode: render the full tabbed detail (Job Details / Match Report / Tailored Resume)
  activeTab?: ReportTab;
  onTabChange?: (tab: ReportTab) => void;
  evalId?: string | null;
  evaluating?: boolean;
  onAnalyzeClick?: () => void;
  // When false, hide the "Saved in My Jobs" + "Open Evaluation Workspace" strip
  // so only "Apply Externally" remains (used for user-jobs detail context).
  showOwnedActions?: boolean;
}

const TABS: { id: ReportTab; label: string }[] = [
  { id: "details", label: "Job Details" },
  { id: "report", label: "Match Report" },
  { id: "resume", label: "Tailored Resume" },
];

export function JobDetailDrawer({
  job,
  isOpen,
  onClose,
  onSave,
  isSaving,
  activeTab,
  onTabChange,
  evalId,
  evaluating = false,
  onAnalyzeClick,
  showOwnedActions = true,
}: JobDetailDrawerProps) {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && isOpen) {
        onClose();
      }
    };

    if (isOpen) {
      window.addEventListener("keydown", handleKeyDown);
      document.body.style.overflow = "hidden";
    }

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = "unset";
    };
  }, [isOpen, onClose]);

  if (!isOpen || !job) {
    return null;
  }

  const isOwned = job.is_owned === true;
  const applyUrl = job.apply_url || job.source_url;
  const workspaceMode = activeTab !== undefined;
  const tab = activeTab ?? "details";

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      {/* Backdrop */}
      <div
        className="fixed inset-0 bg-background/80 backdrop-blur-sm transition-opacity animate-in fade-in duration-200"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Slide-over panel */}
      <aside
        role="dialog"
        aria-modal="true"
        aria-label={job.title}
        className="relative z-50 flex h-full w-full max-w-full lg:max-w-[80vw] flex-col bg-card border-l border-border shadow-2xl animate-in slide-in-from-right duration-300"
      >
        {/* Drawer Header */}
        <div className="border-b border-border/60 p-6 space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
                {job.company}
              </span>
              {job.is_remote === true && (
                <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 px-2.5 py-0.5 text-xs font-semibold">
                  <Globe className="h-3 w-3" />
                  Remote
                </span>
              )}
              <span className={statusBadgeClasses(job.status)}>
                {job.status}
              </span>
            </div>

            <button
              type="button"
              onClick={onClose}
              className="rounded-lg p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"
              aria-label="Close job details"
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          <div>
            <h2 className="text-2xl font-black tracking-tight text-foreground">{job.title}</h2>
            <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground pt-2">
              <span className="flex items-center gap-1.5">
                <MapPin className="h-3.5 w-3.5" />
                <span>{job.location}</span>
              </span>
              {job.posted_date && (
                <span className="flex items-center gap-1.5">
                  <Clock className="h-3.5 w-3.5" />
                  <span>
                    Posted on{" "}
                    {new Date(job.posted_date).toLocaleDateString(undefined, {
                      month: "long",
                      day: "numeric",
                      year: "numeric",
                    })}
                  </span>
                </span>
              )}
            </div>
          </div>

          {/* Workspace controls: Analyze + Apply Externally + segmented tabs */}
          {workspaceMode && (
            <div className="pt-2">
              <div className="flex items-center justify-between gap-3 mb-3">
                <div className="flex items-center gap-3">
                  {onAnalyzeClick && (
                    <button
                      type="button"
                      onClick={onAnalyzeClick}
                      disabled={evaluating}
                      className="inline-flex items-center gap-2 rounded-lg bg-primary px-3.5 py-2 text-xs font-semibold text-primary-foreground shadow transition-colors hover:bg-primary/90 disabled:opacity-50"
                    >
                      <RefreshCw
                        className={`h-3.5 w-3.5 ${evaluating ? "animate-spin" : ""}`}
                      />
                      <span>Analyze Compatibility</span>
                    </button>
                  )}
                  {applyUrl ? (
                    <a
                      href={applyUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-background px-4 py-2 text-xs font-medium text-foreground hover:bg-secondary transition-colors"
                    >
                      <span>Apply Externally</span>
                      <ExternalLink className="h-3.5 w-3.5 text-muted-foreground" />
                    </a>
                  ) : null}
                </div>
              </div>
              <div
                role="tablist"
                className="flex gap-2 border-b border-border/40 overflow-x-auto"
              >
                {TABS.map((t) => {
                  const active = tab === t.id;
                  return (
                    <button
                      key={t.id}
                      role="tab"
                      aria-selected={active}
                      onClick={() => onTabChange?.(t.id)}
                      className={`flex items-center gap-2 border-b-2 px-4 py-3 text-sm font-semibold transition-colors whitespace-nowrap ${
                        active
                          ? "border-primary text-foreground font-bold"
                          : "border-transparent text-muted-foreground hover:border-border hover:text-foreground"
                      }`}
                    >
                      <span>{t.label}</span>
                    </button>
                  );
                })}
              </div>
            </div>
          )}

          {/* Action strip */}
          <div className="flex flex-wrap items-center gap-3 pt-2">
            {showOwnedActions ? (
              <>
                {!isOwned && onSave ? (
                  <button
                    type="button"
                    disabled={isSaving}
                    onClick={() => onSave(job)}
                    className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground shadow hover:bg-primary/90 transition-colors focus:outline-none focus:ring-2 focus:ring-ring"
                  >
                    {isSaving ? (
                      <>
                        <Loader2 className="h-4 w-4 animate-spin" />
                        <span>Saving...</span>
                      </>
                    ) : (
                      <>
                        <Bookmark className="h-4 w-4" />
                        <span>Save to My Job</span>
                      </>
                    )}
                  </button>
                ) : isOwned ? (
                  <div className="flex items-center gap-2.5 flex-wrap">
                    <span className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-500/10 px-3 py-1.5 text-xs font-bold text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                      <Check className="h-4 w-4" />
                      Saved in My Jobs
                    </span>
                    <Link
                      href={`/user-jobs/${job.slug}?openDrawer=1`}
                      className="inline-flex items-center gap-1.5 text-xs font-semibold text-primary hover:underline"
                    >
                      <Sparkles className="h-3.5 w-3.5 text-emerald-500" />
                      <span>Open Evaluation Workspace →</span>
                    </Link>
                  </div>
                ) : null}
              </>
            ) : null}

            {/* Apply Externally lives beside Analyze Compatibility in workspace mode,
                 so only show it here in the simple (non-workspace) drawer. */}
            {!workspaceMode && (
              <>
                {applyUrl ? (
                  <a
                    href={applyUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-background px-4 py-2 text-sm font-medium text-foreground hover:bg-secondary transition-colors"
                  >
                    <span>Apply Externally</span>
                    <ExternalLink className="h-3.5 w-3.5 text-muted-foreground" />
                  </a>
                ) : (
                  <span className="inline-flex items-center gap-1.5 rounded-lg border border-amber-200 bg-amber-50 px-4 py-2 text-sm font-medium text-amber-800 dark:border-amber-900/30 dark:bg-amber-900/10 dark:text-amber-300">
                    <AlertTriangle className="h-3.5 w-3.5" />
                    <span>No application link available</span>
                  </span>
                )}
              </>
            )}
          </div>
        </div>

        {/* Drawer Content */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {workspaceMode ? (
            <>
              {tab === "details" && <JobDetailTab job={job} />}
              {tab === "report" && (
                <MatchReportTab
                  evalId={evalId || undefined}
                  onAnalyzeClick={onAnalyzeClick}
                  evaluating={evaluating}
                />
              )}
              {tab === "resume" && (
                <ResumeTab
                  evalId={evalId || undefined}
                  onAnalyzeClick={onAnalyzeClick}
                  evaluating={evaluating}
                />
              )}
            </>
          ) : (
            <div>
              <h3 className="text-sm font-bold uppercase tracking-wider text-muted-foreground mb-3">
                Job Description
              </h3>
              {job.description ? (
                <div className="prose prose-sm dark:prose-invert max-w-none space-y-3 text-sm leading-relaxed text-foreground/90 whitespace-pre-line">
                  {job.description}
                </div>
              ) : (
                <p className="text-sm text-muted-foreground italic">
                  No description provided for this job opportunity.
                </p>
              )}
            </div>
          )}
        </div>
      </aside>
    </div>
  );
}
