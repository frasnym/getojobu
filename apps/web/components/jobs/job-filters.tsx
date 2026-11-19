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

function optionLabel<T extends string>(options: FilterOption<T>[], value: T): string {
  return options.find((o) => o.value === value)?.label ?? value;
}

const STATUS_OPTIONS: FilterOption<StatusFilter>[] = [
  { label: "All Statuses", value: "all" },
  { label: "Analyzed", value: "analyzed" },
  { label: "Pending", value: "analyze_pending" },
  { label: "Queued", value: "queued" },
  { label: "Queued Retry", value: "queued_retry" },
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
  { label: "Queued", value: "queued" },
  { label: "Queued Retry", value: "queued_retry" },
  { label: "Failed", value: "failed" },
];

const REPORT_STATUS_OPTIONS: FilterOption<string>[] = [
  { label: "All", value: "all" },
  { label: "Success", value: "success" },
  { label: "Pending", value: "pending" },
  { label: "Queued", value: "queued" },
  { label: "Queued Retry", value: "queued_retry" },
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
  // More-filters popover state
  const [moreFiltersOpen, setMoreFiltersOpen] = useState(false);
  const moreFiltersRef = useRef<HTMLDivElement>(null);

  // Close popover on Escape or click outside
  useEffect(() => {
    if (!moreFiltersOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setMoreFiltersOpen(false);
      }
    };
    const handleClickOutside = (e: MouseEvent) => {
      if (
        moreFiltersRef.current &&
        !moreFiltersRef.current.contains(e.target as Node)
      ) {
        setMoreFiltersOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [moreFiltersOpen]);

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
      label: `Status: ${optionLabel(STATUS_OPTIONS, statusFilter)}`,
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
      label: `Saved: ${optionLabel(OWNED_OPTIONS, ownedFilter)}`,
      value: "owned",
      onRemove: () => onOwnedFilterChange?.("all"),
    });
  }
  if (hideFilter !== "all") {
    activeChips.push({
      label: `Visibility: ${optionLabel(HIDE_OPTIONS, hideFilter)}`,
      value: "hide",
      onRemove: () => onHideFilterChange?.("all"),
    });
  }
  if (evaluationStatusFilter !== "all") {
    activeChips.push({
      label: `Evaluation: ${optionLabel(EVALUATION_STATUS_OPTIONS, evaluationStatusFilter)}`,
      value: "evaluation_status",
      onRemove: () => onEvaluationStatusChange?.("all"),
    });
  }
  if (reportStatusFilter !== "all") {
    activeChips.push({
      label: `Report: ${optionLabel(REPORT_STATUS_OPTIONS, reportStatusFilter)}`,
      value: "report_status",
      onRemove: () => onReportStatusChange?.("all"),
    });
  }
  if (eligibilityStatusFilter !== "all") {
    activeChips.push({
      label: `Eligibility: ${optionLabel(ELIGIBILITY_STATUS_OPTIONS, eligibilityStatusFilter)}`,
      value: "eligibility_status",
      onRemove: () => onEligibilityStatusChange?.("all"),
    });
  }
  if (recommendationLabelFilter !== "all") {
    activeChips.push({
      label: `Recommendation: ${optionLabel(RECOMMENDATION_LABEL_OPTIONS, recommendationLabelFilter)}`,
      value: "recommendation_label",
      onRemove: () => onRecommendationLabelChange?.("all"),
    });
  }

  return (
    <div className="flex flex-col gap-3 py-2">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        {/* Search Input */}
        <div className="relative w-full lg:max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="Filter by role, company, or location..."
            className="w-full rounded-lg border border-border bg-card py-2 pl-9 pr-4 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary transition-colors"
          />
        </div>

        {/* Primary filters stay inline; the rest live in a compact popover */}
        <div className="flex flex-wrap items-center gap-2">
          {onHideFilterChange !== undefined && (
            <div className="w-[168px]">
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
            </div>
          )}
          {onRecommendationLabelChange !== undefined && (
            <div className="w-[168px]">
              <FilterDropdown
                prefix="Recommendation"
                value={recommendationLabelFilter}
                options={RECOMMENDATION_LABEL_OPTIONS}
                onChange={onRecommendationLabelChange}
                icon={
                  <Sparkles
                    className={`h-3 w-3 ${
                      recommendationLabelFilter !== "all"
                        ? "text-primary fill-primary/20"
                        : "text-muted-foreground"
                    }`}
                  />
                }
              />
            </div>
          )}

          <div ref={moreFiltersRef} className="relative">
            <button
              type="button"
              onClick={() => setMoreFiltersOpen((prev) => !prev)}
              aria-expanded={moreFiltersOpen}
              aria-haspopup="dialog"
              aria-label="More filters"
              className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-2 text-xs font-semibold transition-all ${
                moreFiltersOpen || activeChips.length > 0
                  ? "bg-primary/10 text-primary border-primary/25"
                  : "border-border/60 bg-secondary/60 text-muted-foreground hover:bg-secondary hover:text-foreground"
              }`}
            >
              <SlidersHorizontal className="h-3.5 w-3.5" />
              <span>More filters</span>
              {activeChips.length > 0 && (
                <span className="inline-flex h-4 w-4 items-center justify-center rounded-full bg-primary text-[10px] font-bold text-primary-foreground">
                  {activeChips.length}
                </span>
              )}
              <ChevronDown className={`h-3.5 w-3.5 transition-transform duration-150 ${moreFiltersOpen ? "rotate-180" : ""}`} />
            </button>

            {moreFiltersOpen && (
              <div
                role="dialog"
                aria-label="More filters"
                className="absolute right-0 top-full z-50 mt-2 w-[540px] max-w-[calc(100vw-2rem)] rounded-xl border border-border bg-card p-4 shadow-2xl"
              >
                <div className="mb-3 flex items-center justify-between">
                  <span className="text-xs font-bold">More filters</span>
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
                    disabled={activeChips.length === 0}
                    className="text-[11px] font-semibold text-muted-foreground transition-colors hover:text-foreground disabled:pointer-events-none disabled:opacity-50"
                  >
                    Clear all
                  </button>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-3">
                    <h4 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                      Job
                    </h4>
                    <FilterDropdown
                      prefix="Job Status"
                      value={statusFilter}
                      options={STATUS_OPTIONS}
                      onChange={onStatusChange}
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
                    {onOwnedFilterChange !== undefined && (
                      <FilterDropdown
                        prefix="Saved"
                        value={ownedFilter}
                        options={OWNED_OPTIONS}
                        onChange={onOwnedFilterChange}
                        icon={
                          <Bookmark
                            className={`h-3 w-3 ${
                              ownedFilter !== "all"
                                ? "text-primary fill-primary/20"
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
                    {onRemoteChange !== undefined && (
                      <button
                        type="button"
                        aria-pressed={remoteOnly}
                        onClick={() => onRemoteChange(!remoteOnly)}
                        className={`relative inline-flex w-full items-center justify-center rounded-lg border px-3 py-2 text-xs font-semibold transition-all ${
                          remoteOnly
                            ? "bg-primary/10 text-primary border-primary/25 shadow-xs"
                            : "border-border/60 bg-secondary/60 text-muted-foreground hover:bg-secondary hover:text-foreground"
                        }`}
                      >
                        <Globe className={`absolute left-3 h-3.5 w-3.5 ${remoteOnly ? "text-primary" : ""}`} />
                        <span>Remote Only</span>
                      </button>
                    )}
                  </div>

                  <div className="space-y-3">
                    <h4 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                      Evaluation
                    </h4>
                    {onEvaluationStatusChange !== undefined && (
                      <FilterDropdown
                        prefix="Evaluation"
                        value={evaluationStatusFilter}
                        options={EVALUATION_STATUS_OPTIONS}
                        onChange={onEvaluationStatusChange}
                        icon={
                          <CheckCircle2
                            className={`h-3 w-3 ${
                              evaluationStatusFilter !== "all"
                                ? "text-primary fill-primary/20"
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
                        icon={
                          <FileText
                            className={`h-3 w-3 ${
                              reportStatusFilter !== "all"
                                ? "text-primary fill-primary/20"
                                : "text-muted-foreground"
                            }`}
                          />
                        }
                      />
                    )}
                  </div>

                  <div className="space-y-3">
                    <h4 className="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
                      Decision
                    </h4>
                    {onEligibilityStatusChange !== undefined && (
                      <FilterDropdown
                        prefix="Eligibility"
                        value={eligibilityStatusFilter}
                        options={ELIGIBILITY_STATUS_OPTIONS}
                        onChange={onEligibilityStatusChange}
                        icon={
                          <AlertTriangle
                            className={`h-3 w-3 ${
                              eligibilityStatusFilter !== "all"
                                ? "text-primary fill-primary/20"
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
                        icon={
                          <Sparkles
                            className={`h-3 w-3 ${
                              recommendationLabelFilter !== "all"
                                ? "text-primary fill-primary/20"
                                : "text-muted-foreground"
                            }`}
                          />
                        }
                      />
                    )}
                  </div>
                </div>

                <div className="mt-4 flex items-center justify-between border-t border-border/40 pt-3">
                  <span className="text-[11px] text-muted-foreground">
                    {activeChips.length} active filter{activeChips.length === 1 ? "" : "s"}
                  </span>
                  <button
                    type="button"
                    onClick={() => setMoreFiltersOpen(false)}
                    className="inline-flex items-center rounded-lg bg-primary px-4 py-2 text-xs font-bold text-primary-foreground shadow-sm hover:bg-primary/90 transition-colors"
                  >
                    Done
                  </button>
                </div>
              </div>
            )}
          </div>

          {isFiltersChanged && onQueryAll && !isDrawerOpen && (
            <button
              type="button"
              onClick={onQueryAll}
              disabled={loading}
              className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-2 text-xs font-semibold transition-colors ${
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

      {/* Active filter chips */}
      {activeChips.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
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
        </div>
      )}
    </div>
  );
}
