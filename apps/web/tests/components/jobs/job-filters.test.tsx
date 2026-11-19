import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import React from "react";
import { JobFilters } from "@/components/jobs/job-filters";

describe("JobFilters component", () => {
  it("renders search input, Filters button, and count", () => {
    render(
      <JobFilters
        searchQuery=""
        onSearchChange={vi.fn()}
        statusFilter="all"
        onStatusChange={vi.fn()}
        totalCount={42}
        remoteOnly={false}
        onRemoteChange={vi.fn()}
      />
    );

    expect(screen.getByPlaceholderText(/Filter by role/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Filters/i })).toBeInTheDocument();
    expect(screen.getByText("42 jobs")).toBeInTheDocument();
  });

  it("handles search input change", () => {
    const onSearchChange = vi.fn();
    render(
      <JobFilters
        searchQuery=""
        onSearchChange={onSearchChange}
        statusFilter="all"
        onStatusChange={vi.fn()}
        totalCount={10}
      />
    );

    const input = screen.getByPlaceholderText(/Filter by role/i);
    fireEvent.change(input, { target: { value: "developer" } });
    expect(onSearchChange).toHaveBeenCalledWith("developer");
  });

  it("opens the filters drawer and selects a status option", () => {
    const onStatusChange = vi.fn();
    render(
      <JobFilters
        searchQuery=""
        onSearchChange={vi.fn()}
        statusFilter="all"
        onStatusChange={onStatusChange}
        totalCount={10}
      />
    );

    // Open drawer
    fireEvent.click(screen.getByRole("button", { name: /Filters/i }));
    expect(screen.getByRole("dialog", { name: /Filters/i })).toBeInTheDocument();

    // Status dropdown lives inside the drawer
    const trigger = screen.getByRole("button", { name: /Status: All/i });
    fireEvent.click(trigger);

    expect(screen.getByRole("listbox", { name: /Status/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /Analyzed/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /Pending/i })).toBeInTheDocument();

    // Select Analyzed
    fireEvent.click(screen.getByRole("option", { name: /Analyzed/i }));
    expect(onStatusChange).toHaveBeenCalledWith("analyzed");
  });

  it("renders active filter chips and clears them individually", () => {
    const onStatusChange = vi.fn();
    const onEvaluationStatusChange = vi.fn();
    render(
      <JobFilters
        searchQuery=""
        onSearchChange={vi.fn()}
        statusFilter="analyzed"
        onStatusChange={onStatusChange}
        evaluationStatusFilter="success"
        onEvaluationStatusChange={onEvaluationStatusChange}
        totalCount={5}
      />
    );

    expect(screen.getByText("Status: Analyzed")).toBeInTheDocument();
    expect(screen.getByText("Evaluation: Success")).toBeInTheDocument();

    // Clear the status chip
    fireEvent.click(
      screen.getByRole("button", { name: /Remove Status: Analyzed filter/i })
    );
    expect(onStatusChange).toHaveBeenCalledWith("all");
  });

  it("toggles remote from the popover", () => {
    const onRemoteChange = vi.fn();
    render(
      <JobFilters
        searchQuery=""
        onSearchChange={vi.fn()}
        statusFilter="all"
        onStatusChange={vi.fn()}
        totalCount={12}
        remoteOnly={false}
        onRemoteChange={onRemoteChange}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /More filters/i }));
    fireEvent.click(screen.getByRole("button", { name: /Remote Only/i }));
    expect(onRemoteChange).toHaveBeenCalledWith(true);
  });

  it("selects an ownership option in the drawer", () => {
    const onOwnedChange = vi.fn();
    render(
      <JobFilters
        searchQuery=""
        onSearchChange={vi.fn()}
        statusFilter="all"
        onStatusChange={vi.fn()}
        totalCount={12}
        ownedFilter="all"
        onOwnedFilterChange={onOwnedChange}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /Filters/i }));
    const savedTrigger = screen.getByRole("button", { name: /Saved: All/i });
    fireEvent.click(savedTrigger);

    expect(screen.getByRole("listbox", { name: /Saved/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /^Saved$/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /Unsaved/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("option", { name: /^Saved$/i }));
    expect(onOwnedChange).toHaveBeenCalledWith("owned");
  });

  it("clears all filters from the drawer footer", () => {
    const onStatusChange = vi.fn();
    const onEvaluationStatusChange = vi.fn();
    const onRemoteChange = vi.fn();
    render(
      <JobFilters
        searchQuery=""
        onSearchChange={vi.fn()}
        statusFilter="analyzed"
        onStatusChange={onStatusChange}
        evaluationStatusFilter="success"
        onEvaluationStatusChange={onEvaluationStatusChange}
        remoteOnly
        onRemoteChange={onRemoteChange}
        totalCount={3}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /Filters/i }));
    fireEvent.click(screen.getByRole("button", { name: /Clear all/i }));

    expect(onStatusChange).toHaveBeenCalledWith("all");
    expect(onEvaluationStatusChange).toHaveBeenCalledWith("all");
    expect(onRemoteChange).toHaveBeenCalledWith(false);
  });

  it("closes the drawer with the Done button", () => {
    render(
      <JobFilters
        searchQuery=""
        onSearchChange={vi.fn()}
        statusFilter="all"
        onStatusChange={vi.fn()}
        totalCount={10}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /Filters/i }));
    expect(screen.getByRole("dialog", { name: /Filters/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Done/i }));
    expect(screen.queryByRole("dialog", { name: /Filters/i })).not.toBeInTheDocument();
  });
});
