import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import React from "react";
import { JobReportHeader } from "@/components/jobs/job-report-header";
import { Job } from "@/lib/api/types";

const mockJob: Job = {
  id: 1,
  slug: "senior-go-engineer-stripe-remote",
  status: "analyzed",
  title: "Senior Go Engineer",
  company: "Stripe",
  location: "Remote",
  is_remote: true,
  created_at: "2026-11-13T10:00:00Z",
  latest_evaluation: {
    id: "eval-001",
    evaluation_status: "success",
    overall_score: 88.5,
    eligibility_status: "eligible",
    recommendation_label: "strong_match",
    resume_status: "ready",
  },
};

describe("JobReportHeader component", () => {
  it("renders job title, company, back link, and match score", () => {
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
    expect(screen.getByText("Back to Jobs")).toBeInTheDocument();
    expect(screen.getByText(/88(\.5)?%/)).toBeInTheDocument();
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
  });
});
