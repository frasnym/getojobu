import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import React from "react";
import { JobReportHeader } from "@/components/jobs/job-report-header";
import { Job, MatchReport } from "@/lib/api/types";

const mockJob: Job = {
  id: 1,
  slug: "senior-go-engineer-stripe-remote",
  status: "analyzed",
  title: "Senior Go Engineer",
  company: "Stripe",
  location: "Remote",
  is_remote: true,
  created_at: "2026-11-13T10:00:00Z",
};

describe("JobReportHeader component", () => {
  it("renders job title, company, back link", () => {
    render(
      <JobReportHeader
        job={mockJob}
        activeTab="report"
        onTabChange={() => {}}
        prevSlug="prev-job"
        nextSlug="next-job"
      />
    );

    expect(screen.getByText("Senior Go Engineer")).toBeInTheDocument();
    expect(screen.getByText("Stripe")).toBeInTheDocument();
    expect(screen.getByText("Back to My Jobs")).toBeInTheDocument();
  });

  const mockEvaluation: MatchReport = {
    id: "eval-1",
    user_job_id: "user-job-1",
    evaluation_status: "success",
    eligibility_status: "eligible",
    recommendation_label: "strong_match",
    report_status: "success",
    resume_status: "success",
    cover_letter_status: "success",
  };

  it("renders the card-style tag rows with the hide button after the title", () => {
    render(
      <JobReportHeader
        job={{ ...mockJob, posted_date: "2026-11-13T10:00:00Z" }}
        activeTab="details"
        onTabChange={() => {}}
        evaluation={mockEvaluation}
        userJobId="user-job-1"
        onHideToggle={vi.fn()}
      />
    );

    expect(screen.getByTestId("remote-tag")).toBeInTheDocument();
    expect(screen.getByTestId("eligibility-tag")).toBeInTheDocument();
    expect(screen.getByTestId("recommendation-tag")).toBeInTheDocument();
    expect(screen.getAllByTestId("evaluation-status-tag")).toHaveLength(5);
    expect(screen.getByRole("button", { name: /Hide job/i }).previousElementSibling?.tagName).toBe("H1");
  });

  it("calls onTabChange when segmented tabs are clicked", () => {
    const handleTabChange = vi.fn();
    render(
      <JobReportHeader
        job={mockJob}
        activeTab="details"
        onTabChange={handleTabChange}
      />
    );

    const reportTab = screen.getByRole("tab", { name: /Match Report/i });
    fireEvent.click(reportTab);
    expect(handleTabChange).toHaveBeenCalledWith("report");

    const resumeTab = screen.getByRole("tab", { name: /Tailored Resume/i });
    fireEvent.click(resumeTab);
    expect(handleTabChange).toHaveBeenCalledWith("resume");

    const coverLetterTab = screen.getByRole("tab", { name: /Cover Letter/i });
    fireEvent.click(coverLetterTab);
    expect(handleTabChange).toHaveBeenCalledWith("cover-letter");
  });

  it("calls onRefresh from the Refresh button and disables it while refreshing", () => {
    const handleRefresh = vi.fn();
    const { rerender } = render(
      <JobReportHeader
        job={mockJob}
        activeTab="cover-letter"
        onTabChange={() => {}}
        onRefresh={handleRefresh}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /Refresh/i }));
    expect(handleRefresh).toHaveBeenCalledTimes(1);

    rerender(
      <JobReportHeader
        job={mockJob}
        activeTab="cover-letter"
        onTabChange={() => {}}
        onRefresh={handleRefresh}
        refreshing
      />
    );

    expect(screen.getByRole("button", { name: /Refresh/i })).toBeDisabled();
  });

  it("greys the whole header when the job is hidden", () => {
    const { container } = render(
      <JobReportHeader
        job={mockJob}
        activeTab="details"
        onTabChange={() => {}}
        isHidden
      />
    );

    expect(container.firstChild).toHaveClass("opacity-60", "grayscale");
  });

  it("does not render a Refresh button without an onRefresh handler", () => {
    render(
      <JobReportHeader
        job={mockJob}
        activeTab="details"
        onTabChange={() => {}}
      />
    );

    expect(screen.queryByRole("button", { name: /Refresh/i })).not.toBeInTheDocument();
  });
});
