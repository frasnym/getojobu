import React from "react";
import { Search, Globe } from "lucide-react";
import { StatusFilter } from "@/hooks/use-jobs";

interface JobFiltersProps {
  searchQuery: string;
  onSearchChange: (q: string) => void;
  statusFilter: StatusFilter;
  onStatusChange: (status: StatusFilter) => void;
  totalCount: number;
  remoteOnly?: boolean;
  onRemoteChange?: (remote: boolean) => void;
}

export function JobFilters({
  searchQuery,
  onSearchChange,
  statusFilter,
  onStatusChange,
  totalCount,
  remoteOnly = false,
  onRemoteChange,
}: JobFiltersProps) {
  const statuses: { label: string; value: StatusFilter }[] = [
    { label: "All", value: "all" },
    { label: "Analyzed", value: "analyzed" },
    { label: "Pending", value: "pending" },
    { label: "Failed", value: "failed" },
  ];

  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between py-2">
      {/* Search Input */}
      <div className="relative flex-1 max-w-md">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <input
          type="text"
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder="Filter by role, company, or location..."
          className="w-full rounded-lg border border-border bg-card py-2 pl-9 pr-4 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary transition-colors"
        />
      </div>

      {/* Filter Options (Remote Toggle + Status Pills) */}
      <div className="flex flex-wrap items-center gap-1.5 overflow-x-auto pb-1 sm:pb-0">
        {onRemoteChange !== undefined && (
          <button
            type="button"
            aria-pressed={remoteOnly}
            onClick={() => onRemoteChange(!remoteOnly)}
            className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-semibold transition-all ${
              remoteOnly
                ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 shadow-sm"
                : "border-border/60 bg-secondary/60 text-muted-foreground hover:bg-secondary hover:text-foreground"
            }`}
          >
            <Globe className={`h-3.5 w-3.5 ${remoteOnly ? "text-emerald-500" : ""}`} />
            <span>Remote Only</span>
          </button>
        )}

        {statuses.map((s) => (
          <button
            key={s.value}
            type="button"
            onClick={() => onStatusChange(s.value)}
            className={`rounded-lg px-3 py-1.5 text-xs font-semibold transition-colors ${
              statusFilter === s.value
                ? "bg-primary text-primary-foreground shadow-sm"
                : "bg-secondary/60 text-muted-foreground hover:bg-secondary hover:text-foreground"
            }`}
          >
            {s.label}
          </button>
        ))}
        <span className="ml-2 text-xs text-muted-foreground whitespace-nowrap">
          {totalCount} {totalCount === 1 ? "job" : "jobs"}
        </span>
      </div>
    </div>
  );
}
