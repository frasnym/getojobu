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
    // Company and location now appear in both the header and the summary grid
    expect(screen.getAllByText("Datadog")).toHaveLength(2);
    expect(screen.getAllByText("New York, NY")).toHaveLength(2);
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
      evaluation: {
        id: "eval-123",
        user_job_id: "uj-456",
        evaluation_status: "success",
      },
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

  it("shows the same job status tag as the list, with the real status label and blue pending color", async () => {
    const pendingJob: Job = { ...mockJob, status: "analyze_pending" };
    vi.spyOn(endpoints, "getUserJobEvaluation").mockResolvedValue({
      id: "eval-123",
      user_job_id: "uj-456",
      evaluation: {
        id: "eval-123",
        user_job_id: "uj-456",
        evaluation_status: "pending",
      },
    });

    render(
      <JobDetailDrawer
        job={pendingJob}
        userJobId="uj-456"
        isOpen={true}
        onClose={() => {}}
      />
    );

    await waitFor(() => {
      expect(screen.getAllByTestId("evaluation-status-tag").length).toBeGreaterThan(0);
    });

    const statusTags = screen.getAllByTestId("evaluation-status-tag");
    expect(statusTags[0]).toHaveTextContent("Analyze Pending");
    expect(statusTags[0]).toHaveClass("bg-sky-500/10 text-sky-600");
  });

  it("renders the card-style header rows with the hide button after the title", async () => {
    vi.spyOn(endpoints, "getUserJobEvaluation").mockResolvedValue({
      id: "eval-123",
      user_job_id: "uj-456",
      evaluation: {
        id: "eval-123",
        user_job_id: "uj-456",
        evaluation_status: "success",
        eligibility_status: "eligible",
        recommendation_label: "strong_match",
        report_status: "success",
        resume_status: "success",
        cover_letter_status: "success",
      },
    });

    render(
      <JobDetailDrawer
        job={{ ...mockJob, is_remote: true }}
        userJobId="uj-456"
        isOpen={true}
        onClose={() => {}}
        onHideToggle={vi.fn()}
      />
    );

    await waitFor(() => {
      expect(screen.getByTestId("eligibility-tag")).toBeInTheDocument();
    });
    expect(screen.getByTestId("remote-tag")).toBeInTheDocument();
    expect(screen.getByTestId("recommendation-tag")).toBeInTheDocument();
    expect(screen.getAllByTestId("evaluation-status-tag")).toHaveLength(5);
    expect(screen.getByRole("button", { name: /Hide job/i }).previousElementSibling?.tagName).toBe("H2");
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

  it("resets fetched evaluation when switching to an unowned job", async () => {
    const evalSpy = vi.spyOn(endpoints, "getUserJobEvaluation").mockResolvedValue({
      id: "eval-123",
      user_job_id: "uj-456",
      evaluation: {
        id: "eval-123",
        user_job_id: "uj-456",
        evaluation_status: "success",
      },
    });

    const { rerender } = render(
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

    evalSpy.mockClear();

    const anotherJob: Job = { ...mockJob, id: 99, slug: "different-job" };
    rerender(
      <JobDetailDrawer
        job={anotherJob}
        isOpen={true}
        onClose={() => {}}
      />
    );

    expect(evalSpy).not.toHaveBeenCalled();
  });
});
