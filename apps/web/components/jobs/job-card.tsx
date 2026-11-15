import React from "react";
import Link from "next/link";
import { Job } from "@/lib/api/types";
import { MapPin, Globe, Clock, ArrowRight } from "lucide-react";
import { statusBadgeClasses } from "@/hooks/use-jobs";

interface JobCardProps {
  job: Job;
  href?: string;
}

export function JobCard({ job, href }: JobCardProps) {
  const targetHref = href || `/user-jobs/${job.slug}`;

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
            <span className={statusBadgeClasses(job.status)}>
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

        {/* Detail trigger */}
        <div className="flex shrink-0 items-center sm:flex-col sm:items-end gap-2 pt-2 sm:pt-0">
          <div className="flex items-center gap-1 text-xs font-medium text-muted-foreground group-hover:text-foreground group-hover:translate-x-0.5 transition-all">
            <span>Details</span>
            <ArrowRight className="h-3.5 w-3.5" />
          </div>
        </div>
      </div>
    </div>
  );
}
