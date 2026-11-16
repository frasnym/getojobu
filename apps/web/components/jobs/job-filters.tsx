"use client";

import React, { useState, useRef, useEffect } from "react";
import { Search, Globe, ChevronDown, Check, X, Bookmark, Eye, EyeOff, RefreshCw, CheckCircle2, FileText, AlertTriangle, Sparkles, SlidersHorizontal, Briefcase } from "lucide-react";
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
  /** Evaluation status filter value ("all" | "success" | "pending" | "failed") */
  evaluationStatusFilter?: string;
  onEvaluationStatusChange?: (value: string) => void;
  /** Report status filter value ("all" | "success" | "pending" | "failed") */
  reportStatusFilter?: string;
  onReportStatusChange?: (value: string) => void;
  /** Eligibility status filter value ("all" | "eligible" | "not_eligible" | ...) */
  eligibilityStatusFilter?: string;
  onEligibilityStatusChange?: (value: string) => void;
  /** Recommendation label filter value ("all" | "strong_match" | ...) */
  recommendationLabelFilter?: string;
  onRecommendationLabelChange?: (value: string) => void;
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
    <div ref={containerRef} className="relative w-full text-left">
      <div
        className={`inline-flex w-full items-center rounded-lg border text-xs font-semibold transition-colors ${
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
          className="inline-flex w-full items-center justify-between gap-1.5 px-3 py-1.5 focus:outline-none"
        >
          {icon ?? <span className="h-3 w-3 shrink-0" aria-hidden="true" />}
          <span className="whitespace-nowrap">{displayLabel}</span>
          <ChevronDown
            className={`h-3 w-3 shrink-0 text-muted-foreground transition-transform duration-150 ${
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
            className="mr-1.5 shrink-0 rounded-sm p-0.5 hover:bg-foreground/10 transition-colors"
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
          className="absolute left-0 top-full mt-1.5 z-50 w-full min-w-[11rem] overflow-hidden rounded-lg border border-border bg-card p-1 shadow-lg ring-1 ring-black/5 dark:ring-white/10 animate-in fade-in-50 zoom-in-95 duration-100"
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
                className={`flex w-full items-center justify-between gap-2 rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors whitespace-nowrap ${
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

const EVALUATION_STATUS_OPTIONS: FilterOption<string>[] = [
  { label: "All", value: "all" },
  { label: "Success", value: "success" },
  { label: "Pending", value: "pending" },
  { label: "Failed", value: "failed" },
];

const REPORT_STATUS_OPTIONS: FilterOption<string>[] = [
  { label: "All", value: "all" },
  { label: "Success", value: "success" },
  { label: "Pending", value: "pending" },
  { label: "Failed", value: "failed" },
];

const ELIGIBILITY_STATUS_OPTIONS: FilterOption<string>[] = [
  { label: "All", value: "all" },
  { label: "Eligible", value: "eligible" },
  { label: "Not Eligible", value: "not_eligible" },
];

const RECOMMENDATION_LABEL_OPTIONS: FilterOption<string>[] = [
  { label: "All", value: "all" },
  { label: "Strong Match", value: "strong_match" },
  { label: "Worth Applying", value: "worth_applying" },
  { label: "Possible Match", value: "possible_match" },
  { label: "Low Match", value: "low_match" },
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
  evaluationStatusFilter = "all",
  onEvaluationStatusChange,
  reportStatusFilter = "all",
  onReportStatusChange,
  eligibilityStatusFilter = "all",
  onEligibilityStatusChange,
  recommendationLabelFilter = "all",
  onRecommendationLabelChange,
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

  // Filters drawer state
  const [filtersDrawerOpen, setFiltersDrawerOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Close drawer on Escape or click outside
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && filtersDrawerOpen) {
        setFiltersDrawerOpen(false);
      }
    };
    const handleClickOutside = (e: MouseEvent) => {
      if (
        filtersDrawerOpen &&
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        setFiltersDrawerOpen(false);
      }
    };
    if (filtersDrawerOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      window.addEventListener("keydown", handleKeyDown);
      document.body.style.overflow = "hidden";
    }
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      window.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = "unset";
    };
  }, [filtersDrawerOpen]);

  // Build list of active filters for chips
  const activeChips: { label: string; value: string; onRemove: () => void }[] = [];
  if (searchQuery) {
    activeChips.push({
      label: `Search: "${searchQuery}"`,
      value: "search",
      onRemove: () => onSearchChange(""),
    });
  }
  if (statusFilter !== "all") {
    activeChips.push({
      label: `Status: ${statusFilter}`,
      value: "status",
      onRemove: () => onStatusChange("all"),
    });
  }
  if (remoteOnly) {
    activeChips.push({
      label: "Remote Only",
      value: "remote",
      onRemove: () => onRemoteChange?.(false),
    });
  }
  if (ownedFilter !== "all") {
    activeChips.push({
      label: `Saved: ${ownedFilter}`,
      value: "owned",
      onRemove: () => onOwnedFilterChange?.("all"),
    });
  }
  if (hideFilter !== "all") {
    activeChips.push({
      label: `Visibility: ${hideFilter}`,
      value: "hide",
      onRemove: () => onHideFilterChange?.("all"),
    });
  }
  if (evaluationStatusFilter !== "all") {
    activeChips.push({
      label: `Evaluation: ${evaluationStatusFilter}`,
      value: "evaluation_status",
      onRemove: () => onEvaluationStatusChange?.("all"),
    });
  }
  if (reportStatusFilter !== "all") {
    activeChips.push({
      label: `Report: ${reportStatusFilter}`,
      value: "report_status",
      onRemove: () => onReportStatusChange?.("all"),
    });
  }
  if (eligibilityStatusFilter !== "all") {
    activeChips.push({
      label: `Eligibility: ${eligibilityStatusFilter}`,
      value: "eligibility_status",
      onRemove: () => onEligibilityStatusChange?.("all"),
    });
  }
  if (recommendationLabelFilter !== "all") {
    activeChips.push({
      label: `Recommendation: ${recommendationLabelFilter}`,
      value: "recommendation_label",
      onRemove: () => onRecommendationLabelChange?.("all"),
    });
  }

  const hasActiveFilters = activeChips.length > 0;

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

      {/* Filter Options: Filters button + active chips + drawer */}
      <div className="flex flex-wrap items-center gap-2 pb-1 sm:pb-0">
        <button
          type="button"
          onClick={() => setFiltersDrawerOpen(true)}
          className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-semibold transition-all ${
            hasActiveFilters
              ? "bg-primary/10 text-primary border-primary/25"
              : "border-border/60 bg-secondary/60 text-muted-foreground hover:bg-secondary hover:text-foreground"
          }`}
          aria-label="Open filters"
        >
          <SlidersHorizontal className="h-3.5 w-3.5" />
          <span>Filters</span>
          {hasActiveFilters && (
            <span className="inline-flex h-4 w-4 items-center justify-center rounded-full bg-primary text-[10px] font-bold text-primary-foreground">
              {activeChips.length}
            </span>
          )}
        </button>

        {/* Active filter chips */}
        {activeChips.map((chip) => (
          <span
            key={chip.value}
            className="inline-flex items-center gap-1 rounded-full border border-border/60 bg-card px-2.5 py-1 text-[11px] font-medium text-foreground"
          >
            {chip.label}
            <button
              type="button"
              onClick={chip.onRemove}
              className="rounded-full p-0.5 hover:bg-secondary transition-colors"
              aria-label={`Remove ${chip.label} filter`}
            >
              <X className="h-3 w-3 text-muted-foreground" />
            </button>
          </span>
        ))}

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

      {/* Filters Drawer */}
      {filtersDrawerOpen && (
        <div className="fixed inset-0 z-50">
          {/* Backdrop */}
          <div
            className="absolute inset-0 bg-black/40 backdrop-blur-sm"
            onClick={() => setFiltersDrawerOpen(false)}
          />
          {/* Panel */}
          <div
            ref={containerRef}
            className="absolute right-0 top-0 flex h-full w-full max-w-lg flex-col bg-card shadow-2xl border-l border-border/60 animate-in slide-in-from-right duration-200"
            role="dialog"
            aria-modal="true"
            aria-label="Filters"
          >
            {/* Drawer Header */}
            <div className="flex items-center justify-between border-b border-border/40 px-5 py-4">
              <h3 className="text-base font-bold text-foreground">Filters</h3>
              <button
                type="button"
                onClick={() => setFiltersDrawerOpen(false)}
                className="rounded-lg p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors"
                aria-label="Close filters"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {/* Drawer Content */}
            <div className="flex-1 overflow-y-auto px-5 py-4 space-y-6">
              {/* Job group */}
              <div className="space-y-3">
                <h4 className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
                  Job
                </h4>
                <div className="grid grid-cols-2 gap-3">
                  <FilterDropdown
                    prefix="Job Status"
                    value={statusFilter}
                    options={STATUS_OPTIONS}
                    onChange={onStatusChange}
                    activeColorClass={statusColorClass}
                    icon={
                      <Briefcase
                        className={`h-3 w-3 ${
                          statusFilter !== "all"
                            ? "text-primary"
                            : "text-muted-foreground"
                        }`}
                      />
                    }
                  />
                  {onRemoteChange !== undefined && (
                    <button
                      type="button"
                      aria-pressed={remoteOnly}
                      onClick={() => onRemoteChange(!remoteOnly)}
                      className={`inline-flex w-full items-center justify-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-semibold transition-all ${
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
                </div>
                <div className="grid grid-cols-2 gap-3">
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
                </div>
              </div>

              {/* Evaluation group */}
              <div className="space-y-3">
                <h4 className="text-xs font-bold uppercase tracking-wider text-muted-foreground">
                  Evaluation
                </h4>
                <div className="grid grid-cols-2 gap-3">
                  {onEvaluationStatusChange !== undefined && (
                    <FilterDropdown
                      prefix="Evaluation"
                      value={evaluationStatusFilter}
                      options={EVALUATION_STATUS_OPTIONS}
                      onChange={onEvaluationStatusChange}
                      activeColorClass="bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30"
                      icon={
                        <CheckCircle2
                          className={`h-3 w-3 ${
                            evaluationStatusFilter !== "all"
                              ? "text-emerald-500 fill-emerald-500/20"
                              : "text-muted-foreground"
                          }`}
                        />
                      }
                    />
                  )}
                  {onReportStatusChange !== undefined && (
                    <FilterDropdown
                      prefix="Report"
                      value={reportStatusFilter}
                      options={REPORT_STATUS_OPTIONS}
                      onChange={onReportStatusChange}
                      activeColorClass="bg-blue-500/15 text-blue-600 dark:text-blue-400 border-blue-500/30"
                      icon={
                        <FileText
                          className={`h-3 w-3 ${
                            reportStatusFilter !== "all"
                              ? "text-blue-500 fill-blue-500/20"
                              : "text-muted-foreground"
                          }`}
                        />
                      }
                    />
                  )}
                </div>
                <div className="grid grid-cols-2 gap-3">
                  {onEligibilityStatusChange !== undefined && (
                    <FilterDropdown
                      prefix="Eligibility"
                      value={eligibilityStatusFilter}
                      options={ELIGIBILITY_STATUS_OPTIONS}
                      onChange={onEligibilityStatusChange}
                      activeColorClass="bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30"
                      icon={
                        <AlertTriangle
                          className={`h-3 w-3 ${
                            eligibilityStatusFilter !== "all"
                              ? "text-amber-500 fill-amber-500/20"
                              : "text-muted-foreground"
                          }`}
                        />
                      }
                    />
                  )}
                  {onRecommendationLabelChange !== undefined && (
                    <FilterDropdown
                      prefix="Recommendation"
                      value={recommendationLabelFilter}
                      options={RECOMMENDATION_LABEL_OPTIONS}
                      onChange={onRecommendationLabelChange}
                      activeColorClass="bg-destructive/15 text-destructive border-destructive/30"
                      icon={
                        <Sparkles
                          className={`h-3 w-3 ${
                            recommendationLabelFilter !== "all"
                              ? "text-destructive fill-destructive/20"
                              : "text-muted-foreground"
                          }`}
                        />
                      }
                    />
                  )}
                </div>
              </div>
            </div>

            {/* Drawer Footer */}
            <div className="flex items-center justify-between gap-2 border-t border-border/40 px-5 py-4">
              <button
                type="button"
                onClick={() => {
                  onStatusChange("all");
                  onRemoteChange?.(false);
                  onOwnedFilterChange?.("all");
                  onHideFilterChange?.("all");
                  onEvaluationStatusChange?.("all");
                  onReportStatusChange?.("all");
                  onEligibilityStatusChange?.("all");
                  onRecommendationLabelChange?.("all");
                }}
                disabled={!hasActiveFilters}
                className="inline-flex items-center gap-1.5 rounded-lg border border-border/60 px-3 py-2 text-xs font-semibold text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors disabled:opacity-50 disabled:pointer-events-none"
              >
                <X className="h-3.5 w-3.5" />
                Clear all
              </button>
              <button
                type="button"
                onClick={() => setFiltersDrawerOpen(false)}
                className="inline-flex items-center rounded-lg bg-primary px-4 py-2 text-xs font-bold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors"
              >
                Done
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
