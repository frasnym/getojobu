import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import React from "react";
import { ExploreJobCard } from "@/components/jobs/explore-job-card";
import { Job } from "@/lib/api/types";

const mockJob: Job = {
  id: 10,
  slug: "staff-engineer-figma",
  status: "analyzed",
  title: "Staff Systems Engineer",
  company: "Figma",
  location: "San Francisco, CA",
  is_remote: true,
  is_owned: false,
  posted_date: "2026-11-14T00:00:00Z",
  created_at: "2026-11-14T00:00:00Z",
};

describe("ExploreJobCard component", () => {
  it("renders job title, company, location, and Save to My Job button when not owned", () => {
    const handleSelect = vi.fn();
    const handleSave = vi.fn();

    render(
      <ExploreJobCard
        job={mockJob}
        onSelect={handleSelect}
        onSave={handleSave}
      />
    );

    expect(screen.getByText("Staff Systems Engineer")).toBeInTheDocument();
    expect(screen.getByText("Figma")).toBeInTheDocument();
    expect(screen.getByText("San Francisco, CA")).toBeInTheDocument();
    expect(screen.getByText("Remote")).toBeInTheDocument();
    expect(screen.getByText("Save to My Job")).toBeInTheDocument();

    const card = screen.getByText("Staff Systems Engineer").closest("div");
    if (card) {
      fireEvent.click(card);
      expect(handleSelect).toHaveBeenCalledWith(mockJob);
    }

    const saveButton = screen.getByRole("button", { name: /Save to My Job/i });
    fireEvent.click(saveButton);
    expect(handleSave).toHaveBeenCalledWith(mockJob);
  });

  it("shows saved state and does not show save button when is_owned is true", () => {
    const ownedJob: Job = { ...mockJob, is_owned: true };
    render(
      <ExploreJobCard
        job={ownedJob}
        onSelect={() => {}}
      />
    );

    expect(screen.getByText("In My Jobs")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Save to My Job/i })).not.toBeInTheDocument();
  });

  it("renders Remote green tag when is_remote is true and hides it when false", () => {
    const { rerender } = render(
      <ExploreJobCard
        job={{ ...mockJob, is_remote: true }}
        onSelect={() => {}}
      />
    );
    expect(screen.getByText("Remote")).toBeInTheDocument();

    rerender(
      <ExploreJobCard
        job={{ ...mockJob, is_remote: false }}
        onSelect={() => {}}
      />
    );
    expect(screen.queryByText("Remote")).not.toBeInTheDocument();
  });
});
