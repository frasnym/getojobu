import React from "react";
import { Job } from "@/lib/api/types";
import { ExternalLink, Building2, MapPin, Globe, Calendar, CheckCircle2 } from "lucide-react";

interface JobDetailTabProps {
  job: Job;
}

export function JobDetailTab({ job }: JobDetailTabProps) {
  return (
    <div className="space-y-6">
      {/* Quick summary grid */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4 rounded-xl border border-border/60 bg-card p-4 sm:p-5">
        <div className="space-y-1">
          <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
            <Building2 className="h-3.5 w-3.5" /> Company
          </span>
          <p className="text-sm font-bold text-foreground">{job.company}</p>
        </div>

        <div className="space-y-1">
          <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
            <MapPin className="h-3.5 w-3.5" /> Location
          </span>
          <p className="text-sm font-bold text-foreground">{job.location}</p>
        </div>

        <div className="space-y-1">
          <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
            <Globe className="h-3.5 w-3.5" /> Workplace
          </span>
          <p className="text-sm font-bold text-foreground">
            {job.is_remote ? "Remote Eligible" : "On-site / Hybrid"}
          </p>
        </div>

        <div className="space-y-1">
          <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-1.5">
            <Calendar className="h-3.5 w-3.5" /> Posted
          </span>
          <p className="text-sm font-bold text-foreground">
            {job.posted_date
              ? new Date(job.posted_date).toLocaleDateString(undefined, {
                  month: "short",
                  day: "numeric",
                  year: "numeric",
                })
              : "Recently"}
          </p>
        </div>
      </div>

      {/* Main role description */}
      <div className="rounded-xl border border-border/60 bg-card p-6 sm:p-8">
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between pb-6 mb-6 border-b border-border/40 gap-4">
          <div>
            <h2 className="text-lg font-bold text-foreground">Role Description</h2>
            <p className="text-xs text-muted-foreground">
              Official job listing text provided by {job.company}.
            </p>
          </div>

          {job.apply_url && (
            <a
              href={job.apply_url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-xs font-bold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors"
            >
              <span>Apply on Company Site</span>
              <ExternalLink className="h-3.5 w-3.5" />
            </a>
          )}
        </div>

        {job.description ? (
          <div className="prose dark:prose-invert max-w-none text-sm text-foreground/90 whitespace-pre-line leading-relaxed font-sans">
            {job.description}
          </div>
        ) : (
          <p className="text-sm text-muted-foreground italic py-6 text-center">
            No detailed description provided for this listing.
          </p>
        )}
      </div>
    </div>
  );
}
