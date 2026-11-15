import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import React from "react";
import { JobDetailDrawer } from "@/components/jobs/job-detail-drawer";
import { Job } from "@/lib/api/types";

const mockJob: Job = {
  id: 11,
  slug: "systems-lead-datadog",
  status: "analyzed",
  title: "Systems Lead",
  company: "Datadog",
  location: "New York, NY",
  is_remote: false,
  is_owned: false,
  apply_url: "https://datadog.com/jobs/systems-lead",
  description: "Detailed description of the systems role.",
  posted_date: "2026-11-13T00:00:00Z",
  created_at: "2026-11-13T00:00:00Z",
};

describe("JobDetailDrawer component", () => {
  it("does not render when isOpen is false", () => {
    const { container } = render(
      <JobDetailDrawer
        job={mockJob}
        isOpen={false}
        onClose={() => {}}
      />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders job information and Save to My Job button when open and unowned", () => {
    const handleClose = vi.fn();
    const handleSave = vi.fn();

    render(
      <JobDetailDrawer
        job={mockJob}
        isOpen={true}
        onClose={handleClose}
        onSave={handleSave}
      />
    );

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByText("Systems Lead")).toBeInTheDocument();
    expect(screen.getByText("Datadog")).toBeInTheDocument();
    expect(screen.getByText("Detailed description of the systems role.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Save to My Job/i })).toBeInTheDocument();
    expect(screen.getByText("Apply Externally")).toBeInTheDocument();

    const saveBtn = screen.getByRole("button", { name: /Save to My Job/i });
    fireEvent.click(saveBtn);
    expect(handleSave).toHaveBeenCalledWith(mockJob);

    const closeBtn = screen.getByRole("button", { name: /Close job details/i });
    fireEvent.click(closeBtn);
    expect(handleClose).toHaveBeenCalled();
  });

  it("renders Saved in My Jobs indicator and workspace link when is_owned is true", () => {
    const ownedJob: Job = { ...mockJob, is_owned: true };

    render(
      <JobDetailDrawer
        job={ownedJob}
        isOpen={true}
        onClose={() => {}}
      />
    );

    expect(screen.getByText("Saved in My Jobs")).toBeInTheDocument();
    expect(screen.getByText(/Open Evaluation Workspace/i)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Save to My Job/i })).not.toBeInTheDocument();
  });
});
