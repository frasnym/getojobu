import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import React from "react";
import { JobDetailDrawer } from "@/components/jobs/job-detail-drawer";
import * as endpoints from "@/lib/api/endpoints";
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
    // Company appears in summary grid only
    expect(screen.getAllByText("Datadog")).toHaveLength(1);
    // Location appears in summary grid only
    expect(screen.getAllByText("New York, NY")).toHaveLength(1);
    expect(screen.getByText("On-site / Hybrid")).toBeInTheDocument();
    expect(screen.getByText("Detailed description of the systems role.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Save to My Job/i })).toBeInTheDocument();
    expect(screen.getByText("Apply on Company Site")).toBeInTheDocument();

    const saveBtn = screen.getByRole("button", { name: /Save to My Job/i });
    fireEvent.click(saveBtn);
    expect(handleSave).toHaveBeenCalledWith(mockJob);

    const closeBtn = screen.getByRole("button", { name: /Close job details/i });
    fireEvent.click(closeBtn);
    expect(handleClose).toHaveBeenCalled();
  });

  it("falls back to source_url when apply_url is empty", () => {
    const jobWithSourceOnly: Job = { ...mockJob, apply_url: undefined, source_url: "https://example.com/job" };

    render(
      <JobDetailDrawer job={jobWithSourceOnly} isOpen={true} onClose={() => {}} />
    );

    const applyLink = screen.getByText("Apply on Company Site").closest("a");
    expect(applyLink).toHaveAttribute("href", "https://example.com/job");
  });

  it("shows a warning tag when both apply_url and source_url are empty", () => {
    const jobWithNoLinks: Job = { ...mockJob, apply_url: undefined, source_url: undefined };

    render(
      <JobDetailDrawer job={jobWithNoLinks} isOpen={true} onClose={() => {}} />
    );

    expect(screen.getByText("No application link available")).toBeInTheDocument();
    expect(screen.queryByText("Apply on Company Site")).not.toBeInTheDocument();
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

  it("triggers evaluation fetch when userJobId is passed", async () => {
    const evalSpy = vi.spyOn(endpoints, "getUserJobEvaluation").mockResolvedValue({
      id: "eval-123",
      user_job_id: "uj-456",
      evaluation_status: "success",
    });

    render(
      <JobDetailDrawer
        job={mockJob}
        userJobId="uj-456"
        isOpen={true}
        onClose={() => {}}
      />
    );

    await waitFor(() => {
      expect(evalSpy).toHaveBeenCalledWith("uj-456", "mock-clerk-test-token");
    });
  });

  it("does not trigger evaluation fetch when userJobId is omitted", () => {
    const evalSpy = vi.spyOn(endpoints, "getUserJobEvaluation");
    evalSpy.mockClear();

    render(
      <JobDetailDrawer
        job={mockJob}
        isOpen={true}
        onClose={() => {}}
      />
    );

    expect(evalSpy).not.toHaveBeenCalled();
  });
});
