import React from "react";
import { Job } from "@/lib/api/types";
import { MapPin, Globe, Clock, ArrowRight, Bookmark, Check, Loader2 } from "lucide-react";
import { statusBadgeClasses } from "@/hooks/use-jobs";
interface ExploreJobCardProps {
  job: Job;
  isSelected?: boolean;
  onSelect: (job: Job) => void;
  onSave?: (job: Job) => void;
  isSaving?: boolean;
}

export function ExploreJobCard({
  job,
  isSelected,
  onSelect,
  onSave,
  isSaving,
}: ExploreJobCardProps) {
  const isOwned = job.is_owned === true;

  return (
    <div
      onClick={() => onSelect(job)}
      className={`group relative rounded-xl border p-5 cursor-pointer transition-all duration-200 ${
        isSelected
          ? "border-primary bg-primary/5 shadow-md shadow-primary/5 ring-1 ring-primary/30"
          : "border-border/60 bg-card hover:border-border hover:shadow-md hover:shadow-primary/5"
      }`}
    >
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1.5 flex-1 min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {job.company}
            </span>
            {job.is_remote === true && (
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 px-2 py-0.5 text-[11px] font-semibold">
                <Globe className="h-3 w-3" />
                Remote
              </span>
            )}
            <span className={statusBadgeClasses(job.status)}>
              {job.status}
            </span>

            {isOwned && (
              <span className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-semibold text-primary">
                <Check className="h-3 w-3" />
                In My Jobs
              </span>
            )}
          </div>

          <h3 className="text-lg font-bold tracking-tight text-foreground group-hover:text-primary transition-colors truncate">
            {job.title}
          </h3>

          <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground pt-1">
            <span className="flex items-center gap-1">
              <MapPin className="h-3.5 w-3.5 shrink-0" />
              <span>{job.location}</span>
            </span>
            {job.posted_date && (
              <span className="flex items-center gap-1">
                <Clock className="h-3.5 w-3.5 shrink-0" />
                <span>
                  {new Date(job.posted_date).toLocaleDateString(undefined, {
                    month: "short",
                    day: "numeric",
                  })}
                </span>
              </span>
            )}
          </div>
        </div>

        {/* Action button & detail trigger */}
        <div className="flex items-center sm:flex-col sm:items-end justify-between sm:justify-start gap-2 pt-2 sm:pt-0 shrink-0">
          {!isOwned && onSave ? (
            <button
              type="button"
              disabled={isSaving}
              onClick={(e) => {
                e.stopPropagation();
                onSave(job);
              }}
              className="inline-flex items-center gap-1.5 rounded-lg border border-border/80 bg-background px-3 py-1.5 text-xs font-semibold text-foreground hover:bg-secondary transition-colors focus:outline-none focus:ring-2 focus:ring-ring"
            >
              {isSaving ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>Saving...</span>
                </>
              ) : (
                <>
                  <Bookmark className="h-3.5 w-3.5 text-muted-foreground" />
                  <span>Save to My Job</span>
                </>
              )}
            </button>
          ) : isOwned ? (
            <span className="text-xs text-muted-foreground font-medium">Saved</span>
          ) : null}

          <div className="flex items-center gap-1 text-xs font-medium text-muted-foreground group-hover:text-foreground group-hover:translate-x-0.5 transition-all">
            <span>Details</span>
            <ArrowRight className="h-3.5 w-3.5" />
          </div>
        </div>
      </div>
    </div>
  );
}
