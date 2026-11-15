"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { Job } from "@/lib/api/types";
import {
  X,
  MapPin,
  Globe,
  Clock,
  Bookmark,
  Check,
  ExternalLink,
  Loader2,
  Sparkles,
} from "lucide-react";
import { statusBadgeClasses } from "@/hooks/use-jobs";

interface JobDetailDrawerProps {
  job: Job | null;
  isOpen: boolean;
  onClose: () => void;
  onSave?: (job: Job) => void;
  isSaving?: boolean;
}

export function JobDetailDrawer({
  job,
  isOpen,
  onClose,
  onSave,
  isSaving = false,
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
        className="relative z-50 flex h-full w-full max-w-2xl flex-col bg-card border-l border-border shadow-2xl animate-in slide-in-from-right duration-300"
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

          {/* Action strip */}
          <div className="flex flex-wrap items-center gap-3 pt-2">
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
                  href={`/user-jobs/${job.slug}`}
                  className="inline-flex items-center gap-1.5 text-xs font-semibold text-primary hover:underline"
                >
                  <Sparkles className="h-3.5 w-3.5 text-emerald-500" />
                  <span>Open Evaluation Workspace →</span>
                </Link>
              </div>
            ) : null}

            {job.apply_url && (
              <a
                href={job.apply_url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-background px-4 py-2 text-sm font-medium text-foreground hover:bg-secondary transition-colors"
              >
                <span>Apply Externally</span>
                <ExternalLink className="h-3.5 w-3.5 text-muted-foreground" />
              </a>
            )}
          </div>
        </div>

        {/* Drawer Content */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
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
        </div>
      </aside>
    </div>
  );
}
