import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import React from "react";
import { JobCard } from "@/components/jobs/job-card";
import { Job } from "@/lib/api/types";

const mockJob: Job = {
  id: 1,
  slug: "senior-go-engineer-stripe-remote",
  status: "analyzed",
  title: "Senior Go Engineer",
  company: "Stripe",
  location: "Remote, US",
  is_remote: true,
  posted_date: "2026-11-13T10:00:00Z",
  created_at: "2026-11-13T10:00:00Z",
  latest_evaluation: {
    id: "eval-001",
    evaluation_status: "success",
    overall_score: 88.5,
    eligibility_status: "eligible",
    recommendation_label: "strong_match",
  },
};

describe("JobCard component", () => {
  it("renders job title, company, location, and match score badge", () => {
    render(<JobCard job={mockJob} />);
    expect(screen.getByText("Senior Go Engineer")).toBeInTheDocument();
    expect(screen.getByText("Stripe")).toBeInTheDocument();
    expect(screen.getByText("Remote, US")).toBeInTheDocument();
    expect(screen.getByText("Remote")).toBeInTheDocument();
    expect(screen.getByText(/88(\.5)?%/)).toBeInTheDocument();
    expect(screen.getByText(/Strong Match/i)).toBeInTheDocument();
  });

  it("handles unevaluated jobs cleanly", () => {
    const unevaluatedJob: Job = { ...mockJob, latest_evaluation: null };
    render(<JobCard job={unevaluatedJob} />);
    expect(screen.getByText("Not evaluated")).toBeInTheDocument();
  });
});
