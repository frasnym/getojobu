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
};

describe("JobCard component", () => {
  it("renders job title, company, location, and remote badge", () => {
    render(<JobCard job={mockJob} />);
    expect(screen.getByText("Senior Go Engineer")).toBeInTheDocument();
    expect(screen.getByText("Stripe")).toBeInTheDocument();
    expect(screen.getByText("Remote, US")).toBeInTheDocument();
    expect(screen.getByText("Remote")).toBeInTheDocument();
  });

  it("renders the details trigger", () => {
    render(<JobCard job={mockJob} />);
    expect(screen.getByText(/Details/i)).toBeInTheDocument();
  });
});
