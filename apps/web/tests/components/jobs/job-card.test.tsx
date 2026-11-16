import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
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
  user_job_id: "11111111-1111-1111-1111-111111111111",
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

  it("renders a Hide button when job is not hidden", () => {
    render(<JobCard job={{ ...mockJob, is_hidden: false }} onHideToggle={vi.fn()} />);
    expect(screen.getByRole("button", { name: /Hide job/i })).toBeInTheDocument();
    expect(screen.getByText("Hide")).toBeInTheDocument();
  });

  it("renders an Unhide button when job is hidden", () => {
    render(<JobCard job={{ ...mockJob, is_hidden: true }} onHideToggle={vi.fn()} />);
    expect(screen.getByRole("button", { name: /Unhide job/i })).toBeInTheDocument();
    expect(screen.getByText("Unhide")).toBeInTheDocument();
  });

  it("calls onHideToggle with user_job_id and toggled state when clicked", () => {
    const onHideToggle = vi.fn();
    render(<JobCard job={{ ...mockJob, is_hidden: false }} onHideToggle={onHideToggle} />);
    const button = screen.getByRole("button", { name: /Hide job/i });
    fireEvent.click(button);
    expect(onHideToggle).toHaveBeenCalledWith("11111111-1111-1111-1111-111111111111", true);
  });
});
