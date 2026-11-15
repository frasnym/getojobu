import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import React from "react";
import { JobFilters } from "@/components/jobs/job-filters";

describe("JobFilters component", () => {
  it("renders search input, remote toggle, status dropdown, and count", () => {
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
    expect(screen.getByRole("button", { name: /Remote Only/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Status: All/i })).toBeInTheDocument();
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

  it("opens status dropdown and selects an option", () => {
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

    const trigger = screen.getByRole("button", { name: /Status: All/i });
    fireEvent.click(trigger);

    // Listbox should be visible
    expect(screen.getByRole("listbox", { name: /Status/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /Analyzed/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /Pending/i })).toBeInTheDocument();

    // Select Analyzed
    fireEvent.click(screen.getByRole("option", { name: /Analyzed/i }));
    expect(onStatusChange).toHaveBeenCalledWith("analyzed");
  });

  it("renders active status state with clear button and clears on click", () => {
    const onStatusChange = vi.fn();
    render(
      <JobFilters
        searchQuery=""
        onSearchChange={vi.fn()}
        statusFilter="analyzed"
        onStatusChange={onStatusChange}
        totalCount={5}
      />
    );

    expect(screen.getByText("Analyzed")).toBeInTheDocument();
    const clearBtn = screen.getByRole("button", { name: /Clear Status filter/i });
    expect(clearBtn).toBeInTheDocument();

    fireEvent.click(clearBtn);
    expect(onStatusChange).toHaveBeenCalledWith("all");
  });

  it("renders and toggles ownership dropdown when onOwnedFilterChange is provided", () => {
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

    const savedTrigger = screen.getByRole("button", { name: /Saved: All/i });
    fireEvent.click(savedTrigger);

    expect(screen.getByRole("listbox", { name: /Saved/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /^Saved$/i })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: /Unsaved/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("option", { name: /^Saved$/i }));
    expect(onOwnedChange).toHaveBeenCalledWith("owned");
  });

  it("clears ownership filter via clear button", () => {
    const onOwnedChange = vi.fn();
    render(
      <JobFilters
        searchQuery=""
        onSearchChange={vi.fn()}
        statusFilter="all"
        onStatusChange={vi.fn()}
        totalCount={12}
        ownedFilter="owned"
        onOwnedFilterChange={onOwnedChange}
      />
    );

    const clearBtn = screen.getByRole("button", { name: /Clear Saved filter/i });
    expect(clearBtn).toBeInTheDocument();

    fireEvent.click(clearBtn);
    expect(onOwnedChange).toHaveBeenCalledWith("all");
  });
});
