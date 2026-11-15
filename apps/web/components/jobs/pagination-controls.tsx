"use client";

import React from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

export interface PaginationControlsProps {
  page: number;
  limit: number;
  totalItems: number;
  totalPages: number;
  hasNext: boolean;
  hasPrev: boolean;
  onPageChange: (newPage: number) => void;
  onLimitChange: (newLimit: number) => void;
  limitOptions?: number[];
  disabled?: boolean;
}

export function PaginationControls({
  page,
  limit,
  totalItems,
  totalPages,
  hasNext,
  hasPrev,
  onPageChange,
  onLimitChange,
  limitOptions = [10, 20, 50],
  disabled = false,
}: PaginationControlsProps) {
  if (totalItems <= 0) {
    return null;
  }

  const startItem = totalItems === 0 ? 0 : (page - 1) * limit + 1;
  const endItem = Math.min(page * limit, totalItems);

  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between border-t border-border/40 pt-4 text-xs text-muted-foreground">
      {/* Result range counter */}
      <div className="flex items-center gap-2">
        <span>
          Showing <span className="font-semibold text-foreground">{startItem}</span> to{" "}
          <span className="font-semibold text-foreground">{endItem}</span> of{" "}
          <span className="font-semibold text-foreground">{totalItems}</span> jobs
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-4 sm:gap-6">
        {/* Page size limit selector */}
        <div className="flex items-center gap-2">
          <span>Per page:</span>
          <div className="flex items-center rounded-lg border border-border bg-card p-0.5">
            {limitOptions.map((opt) => {
              const isSelected = limit === opt;
              return (
                <button
                  key={opt}
                  type="button"
                  disabled={disabled}
                  onClick={() => onLimitChange(opt)}
                  className={`rounded-md px-2.5 py-1 text-xs font-semibold transition-colors ${
                    isSelected
                      ? "bg-primary text-primary-foreground shadow-xs"
                      : "text-muted-foreground hover:text-foreground"
                  } ${disabled ? "opacity-50 cursor-not-allowed" : ""}`}
                >
                  {opt}
                </button>
              );
            })}
          </div>
        </div>

        {/* Page navigation controls */}
        <div className="flex items-center gap-2">
          <button
            type="button"
            disabled={!hasPrev || disabled}
            onClick={() => onPageChange(page - 1)}
            aria-label="Previous page"
            className="inline-flex items-center gap-1 rounded-lg border border-border bg-card px-2.5 py-1 text-xs font-semibold text-foreground transition-colors hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-40"
          >
            <ChevronLeft className="h-3.5 w-3.5" />
            <span className="hidden sm:inline">Previous</span>
          </button>

          <span className="px-2 font-medium text-foreground">
            Page <span className="font-bold">{page}</span> of{" "}
            <span className="font-bold">{Math.max(1, totalPages)}</span>
          </span>

          <button
            type="button"
            disabled={!hasNext || disabled}
            onClick={() => onPageChange(page + 1)}
            aria-label="Next page"
            className="inline-flex items-center gap-1 rounded-lg border border-border bg-card px-2.5 py-1 text-xs font-semibold text-foreground transition-colors hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-40"
          >
            <span className="hidden sm:inline">Next</span>
            <ChevronRight className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
    </div>
  );
}
