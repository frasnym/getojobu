"use client";

import React, { useState, useRef, useEffect } from "react";
import { Search, Globe, ChevronDown, Check, X, Bookmark, Eye, EyeOff, RefreshCw } from "lucide-react";
import {
  StatusFilter,
  OwnedFilter,
  HideFilter,
} from "@/hooks/use-jobs";

interface JobFiltersProps {
  searchQuery: string;
  onSearchChange: (q: string) => void;
  statusFilter: StatusFilter;
  onStatusChange: (status: StatusFilter) => void;
  totalCount: number;
  remoteOnly?: boolean;
  onRemoteChange?: (remote: boolean) => void;
  ownedFilter?: OwnedFilter;
  onOwnedFilterChange?: (owned: OwnedFilter) => void;
  hideFilter?: HideFilter;
  onHideFilterChange?: (filter: HideFilter) => void;
  /** When true, at least one filter differs from its initial state. */
  isFiltersChanged?: boolean;
  /** Query the backend with the current filters (server-side). */
  onQueryAll?: () => void;
  /** Whether a backend query is in flight. */
  loading?: boolean;
  /** When true (job detail drawer open), hide the Query all / Search all button. */
  isDrawerOpen?: boolean;
}

interface FilterOption<T extends string> {
  value: T;
  label: string;
}

function FilterDropdown<T extends string>({
  prefix,
  value,
  allValue = "all" as T,
  options,
  onChange,
  activeColorClass = "bg-primary/10 text-primary border-primary/25",
  icon,
}: {
  prefix: string;
  value: T;
  allValue?: T;
  options: FilterOption<T>[];
  onChange: (value: T) => void;
  activeColorClass?: string;
  icon?: React.ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const isFiltered = value !== allValue;
  const selectedOption = options.find((o) => o.value === value);
  const displayLabel = isFiltered
    ? selectedOption?.label || value
    : `${prefix}: All`;

  useEffect(() => {
    if (!open) return;

    const handleClickOutside = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    };

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [open]);

  return (
    <div ref={containerRef} className="relative inline-block text-left">
      <div
        className={`inline-flex items-center rounded-lg border text-xs font-semibold transition-colors ${
          isFiltered
            ? activeColorClass
            : "border-border/60 bg-secondary/60 text-muted-foreground hover:bg-secondary hover:text-foreground"
        }`}
      >
        <button
          type="button"
          onClick={() => setOpen((prev) => !prev)}
          aria-expanded={open}
          aria-haspopup="listbox"
          className="inline-flex items-center gap-1.5 px-3 py-1.5 focus:outline-none"
        >
          {icon}
          <span>{displayLabel}</span>
          <ChevronDown
            className={`h-3 w-3 text-muted-foreground transition-transform duration-150 ${
              open ? "rotate-180" : ""
            }`}
          />
        </button>

        {isFiltered && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onChange(allValue);
              setOpen(false);
            }}
            className="mr-1.5 rounded-sm p-0.5 hover:bg-foreground/10 transition-colors"
            aria-label={`Clear ${prefix} filter`}
            title={`Clear ${prefix} filter`}
          >
            <X className="h-3 w-3" />
          </button>
        )}
      </div>

      {open && (
        <div
          role="listbox"
          aria-label={prefix}
          className="absolute left-0 top-full mt-1.5 z-50 min-w-[8.5rem] overflow-hidden rounded-lg border border-border bg-card p-1 shadow-lg ring-1 ring-black/5 dark:ring-white/10 animate-in fade-in-50 zoom-in-95 duration-100"
        >
          {options.map((opt) => {
            const isSelected = value === opt.value;
            return (
              <button
                key={opt.value}
                type="button"
                role="option"
                aria-selected={isSelected}
                onClick={() => {
                  onChange(opt.value);
                  setOpen(false);
                }}
                className={`flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors ${
                  isSelected
                    ? "bg-primary/10 text-primary font-semibold"
                    : "text-foreground/80 hover:bg-secondary hover:text-foreground"
                }`}
              >
                <span>{opt.label}</span>
                {isSelected && (
                  <Check className="h-3 w-3 text-primary shrink-0" />
                )}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

const STATUS_OPTIONS: FilterOption<StatusFilter>[] = [
  { label: "All Statuses", value: "all" },
  { label: "Analyzed", value: "analyzed" },
  { label: "Pending", value: "analyze_pending" },
  { label: "Failed", value: "failed" },
];

const OWNED_OPTIONS: FilterOption<OwnedFilter>[] = [
  { label: "All", value: "all" },
  { label: "Saved", value: "owned" },
  { label: "Unsaved", value: "not_owned" },
];

const HIDE_OPTIONS: FilterOption<HideFilter>[] = [
  { label: "All", value: "all" },
  { label: "Visible", value: "visible" },
  { label: "Hidden", value: "hidden" },
];

export function JobFilters({
  searchQuery,
  onSearchChange,
  statusFilter,
  onStatusChange,
  totalCount,
  remoteOnly = false,
  onRemoteChange,
  ownedFilter = "all",
  onOwnedFilterChange,
  hideFilter = "all",
  onHideFilterChange,
  isFiltersChanged = false,
  onQueryAll,
  loading = false,
  isDrawerOpen = false,
}: JobFiltersProps) {
  // Determine status active color
  let statusColorClass = "bg-primary/10 text-primary border-primary/25";
  if (statusFilter === "analyzed") {
    statusColorClass =
      "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30";
  } else if (statusFilter === "analyze_pending") {
    statusColorClass =
      "bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30";
  } else if (statusFilter === "failed") {
    statusColorClass =
      "bg-destructive/15 text-destructive border-destructive/30";
  }

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

      {/* Filter Options (Remote Toggle + Dropdowns) */}
      <div className="flex flex-wrap items-center gap-2 pb-1 sm:pb-0">
        {onRemoteChange !== undefined && (
          <button
            type="button"
            aria-pressed={remoteOnly}
            onClick={() => onRemoteChange(!remoteOnly)}
            className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-semibold transition-all ${
              remoteOnly
                ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30 shadow-xs"
                : "border-border/60 bg-secondary/60 text-muted-foreground hover:bg-secondary hover:text-foreground"
            }`}
          >
            <Globe
              className={`h-3.5 w-3.5 ${
                remoteOnly ? "text-emerald-500" : ""
              }`}
            />
            <span>Remote Only</span>
          </button>
        )}

        {/* Ownership / Saved Dropdown */}
        {onOwnedFilterChange !== undefined && (
          <FilterDropdown
            prefix="Saved"
            value={ownedFilter}
            options={OWNED_OPTIONS}
            onChange={onOwnedFilterChange}
            activeColorClass="bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30"
            icon={
              <Bookmark
                className={`h-3 w-3 ${
                  ownedFilter !== "all"
                    ? "text-emerald-500 fill-emerald-500/20"
                    : "text-muted-foreground"
                }`}
              />
            }
          />
        )}

        {/* Hide/Show Dropdown */}
        {onHideFilterChange !== undefined && (
          <FilterDropdown
            prefix="Visibility"
            value={hideFilter}
            options={HIDE_OPTIONS}
            onChange={onHideFilterChange}
            icon={
              hideFilter === "hidden" ? (
                <EyeOff className="h-3 w-3 text-muted-foreground" />
              ) : (
                <Eye className="h-3 w-3 text-muted-foreground" />
              )
            }
          />
        )}

        {/* Status Dropdown */}
        <FilterDropdown
          prefix="Status"
          value={statusFilter}
          options={STATUS_OPTIONS}
          onChange={onStatusChange}
          activeColorClass={statusColorClass}
        />

        {/* Query all / Search all — only shown when filters differ from initial state and no drawer is open */}
        {isFiltersChanged && onQueryAll && !isDrawerOpen && (
          <button
            type="button"
            onClick={onQueryAll}
            disabled={loading}
            className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-semibold transition-colors ${
              loading
                ? "border-border/40 bg-secondary/50 text-muted-foreground"
                : "bg-primary text-primary-foreground border-primary hover:bg-primary/90"
            }`}
            aria-label={searchQuery ? "Search all jobs on backend" : "Query all jobs on backend"}
          >
            {loading ? (
              <>
                <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                <span>Searching...</span>
              </>
            ) : searchQuery ? (
              <span>Search all</span>
            ) : (
              <span>Query all</span>
            )}
          </button>
        )}

        <span className="ml-1 text-xs text-muted-foreground whitespace-nowrap">
          {totalCount} {totalCount === 1 ? "job" : "jobs"}
        </span>
      </div>
    </div>
  );
}
