import React from "react";
import Link from "next/link";
import { Job } from "@/lib/api/types";
import { MapPin, Globe, Sparkles, Clock, ArrowRight } from "lucide-react";

interface JobCardProps {
  job: Job;
  href?: string;
}

export function JobCard({ job, href }: JobCardProps) {
  const targetHref = href || `/user-jobs/${job.slug}`;
  const evalSummary = job.latest_evaluation;
  const score = evalSummary?.overall_score;
  const recommendation = evalSummary?.recommendation_label?.replace("_", " ");
  const isEligible = evalSummary?.eligibility_status === "eligible";

  return (
    <div className="group relative rounded-xl border border-border/60 bg-card p-5 transition-all duration-200 hover:border-border hover:shadow-md hover:shadow-primary/5">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1.5 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {job.company}
            </span>
            {job.is_remote && (
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2 py-0.5 text-[11px] font-medium text-emerald-500">
                <Globe className="h-3 w-3" />
                Remote
              </span>
            )}
            <span className="rounded-full bg-secondary px-2 py-0.5 text-[11px] font-medium text-secondary-foreground capitalize">
              {job.status}
            </span>
          </div>

          <h3 className="text-lg font-bold tracking-tight text-foreground group-hover:text-primary transition-colors">
            <Link href={targetHref} className="focus:outline-none">
              <span className="absolute inset-0" aria-hidden="true" />
              {job.title}
            </Link>
          </h3>

          <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground pt-1">
            <span className="flex items-center gap-1">
              <MapPin className="h-3.5 w-3.5" />
              {job.location}
            </span>
            {job.posted_date && (
              <span className="flex items-center gap-1">
                <Clock className="h-3.5 w-3.5" />
                {new Date(job.posted_date).toLocaleDateString(undefined, {
                  month: "short",
                  day: "numeric",
                })}
              </span>
            )}
          </div>
        </div>

        {/* Match evaluation score badge */}
        <div className="flex shrink-0 items-center sm:flex-col sm:items-end gap-2">
          {evalSummary && score !== undefined ? (
            <div className="flex items-center gap-2 sm:flex-col sm:items-end">
              <div
                className={`inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1 text-sm font-bold ${
                  isEligible
                    ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20"
                    : "bg-amber-500/15 text-amber-600 dark:text-amber-400 border border-amber-500/20"
                }`}
              >
                <Sparkles className="h-3.5 w-3.5" />
                <span>{score.toFixed(1)}%</span>
              </div>
              {recommendation && (
                <span className="text-[11px] font-medium text-muted-foreground capitalize">
                  {recommendation}
                </span>
              )}
            </div>
          ) : (
            <span className="rounded-md border border-dashed border-border px-2 py-1 text-xs text-muted-foreground">
              Not evaluated
            </span>
          )}

          <div className="ml-auto sm:ml-0 sm:pt-2 text-muted-foreground group-hover:text-foreground transition-transform group-hover:translate-x-0.5">
            <ArrowRight className="h-4 w-4" />
          </div>
        </div>
      </div>
    </div>
  );
}
