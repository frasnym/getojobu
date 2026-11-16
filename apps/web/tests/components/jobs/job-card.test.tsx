import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import React from "react";
import { JobCard } from "@/components/jobs/job-card";
import { UserJob, UserJobEvaluation } from "@/lib/api/types";

const mockUserJob: UserJob = {
  id: "11111111-1111-1111-1111-111111111111",
  is_hidden: false,
  created_at: "2026-11-13T10:00:00Z",
  updated_at: "2026-11-13T10:00:00Z",
  job: {
    id: 1,
    slug: "senior-go-engineer-stripe-remote",
    status: "analyzed",
    title: "Senior Go Engineer",
    company: "Stripe",
    location: "Remote, US",
    is_remote: true,
    posted_date: "2026-11-13T10:00:00Z",
    created_at: "2026-11-13T10:00:00Z",
  },
};

describe("JobCard component", () => {
  it("renders job title, company, location, and remote badge", () => {
    render(<JobCard userJob={mockUserJob} />);
    expect(screen.getByText("Senior Go Engineer")).toBeInTheDocument();
    expect(screen.getByText("Stripe")).toBeInTheDocument();
    expect(screen.getByText("Remote, US")).toBeInTheDocument();
    expect(screen.getByText("Remote")).toBeInTheDocument();
  });

  it("renders the details trigger", () => {
    render(<JobCard userJob={mockUserJob} />);
    expect(screen.getByText(/Details/i)).toBeInTheDocument();
  });

  it("renders a Hide button when job is not hidden", () => {
    render(<JobCard userJob={{ ...mockUserJob, is_hidden: false }} onHideToggle={vi.fn()} />);
    expect(screen.getByRole("button", { name: /Hide job/i })).toBeInTheDocument();
    expect(screen.getByText("Hide")).toBeInTheDocument();
  });

  it("renders an Unhide button when job is hidden", () => {
    render(<JobCard userJob={{ ...mockUserJob, is_hidden: true }} onHideToggle={vi.fn()} />);
    expect(screen.getByRole("button", { name: /Unhide job/i })).toBeInTheDocument();
    expect(screen.getByText("Unhide")).toBeInTheDocument();
  });

  it("calls onHideToggle with user_job_id and toggled state when clicked", () => {
    const onHideToggle = vi.fn();
    render(<JobCard userJob={{ ...mockUserJob, is_hidden: false }} onHideToggle={onHideToggle} />);
    const button = screen.getByRole("button", { name: /Hide job/i });
    fireEvent.click(button);
    expect(onHideToggle).toHaveBeenCalledWith("11111111-1111-1111-1111-111111111111", true);
  });
});

describe("JobCard evaluation tags", () => {
  const baseEvaluation: UserJobEvaluation = {
    id: "6aa11d70-d507-4913-b27d-a57473e54174",
    evaluation_status: "success",
    eligibility_status: "eligible",
    recommendation_label: "low_match",
    report_status: "success",
  };

  const mockEvaluatedUserJob: UserJob = {
    ...mockUserJob,
    evaluation: baseEvaluation,
  };

  const evaluatedJob = (overrides: Partial<UserJobEvaluation>): UserJob => ({
    ...mockUserJob,
    evaluation: { ...baseEvaluation, ...overrides },
  });

  it("renders a green Evaluation tag when evaluation_status is success", () => {
    render(<JobCard userJob={mockEvaluatedUserJob} />);
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags).toHaveLength(2);
    expect(tags[0]).toHaveClass("bg-emerald-500/10 text-emerald-600");
    expect(tags[0]).toHaveTextContent("Evaluation");
  });

  it("renders a yellow Evaluation tag when evaluation_status is pending", () => {
    render(<JobCard userJob={evaluatedJob({ evaluation_status: "pending" })} />);
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags[0]).toHaveClass("bg-amber-500/10 text-amber-600");
  });

  it("renders a red Evaluation tag when evaluation_status is failed", () => {
    render(<JobCard userJob={evaluatedJob({ evaluation_status: "failed" })} />);
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags[0]).toHaveClass("bg-destructive/10 text-destructive");
  });

  it("renders a report status tag", () => {
    render(<JobCard userJob={mockEvaluatedUserJob} />);
    expect(screen.getAllByTestId("evaluation-status-tag")).toHaveLength(2);
  });

  it("renders eligibility and recommendation pills like the detail page", () => {
    render(<JobCard userJob={mockEvaluatedUserJob} />);
    expect(screen.getByTestId("eligibility-tag")).toHaveTextContent("eligible");
    expect(screen.getByTestId("eligibility-tag")).toHaveClass(
      "bg-emerald-500/10 text-emerald-600"
    );
    expect(screen.getByTestId("recommendation-tag")).toHaveTextContent(
      "low match"
    );
    expect(screen.getByTestId("recommendation-tag")).toHaveClass(
      "bg-secondary text-secondary-foreground"
    );
  });

  it("renders no evaluation tags when there is no evaluation", () => {
    render(<JobCard userJob={mockUserJob} />);
    expect(screen.queryByTestId("evaluation-status-tag")).not.toBeInTheDocument();
    expect(screen.queryByTestId("eligibility-tag")).not.toBeInTheDocument();
    expect(screen.queryByTestId("recommendation-tag")).not.toBeInTheDocument();
  });
});
