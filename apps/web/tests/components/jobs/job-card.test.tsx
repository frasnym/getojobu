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
    resume_status: "success",
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
    expect(tags).toHaveLength(4);
    expect(tags[0]).toHaveTextContent("Analyzed");
    expect(tags[1]).toHaveClass("bg-emerald-500/10 text-emerald-600");
    expect(tags[1]).toHaveTextContent("Evaluation");
  });

  it("renders a yellow Evaluation tag when evaluation_status is pending", () => {
    render(<JobCard userJob={evaluatedJob({ evaluation_status: "pending" })} />);
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags[1]).toHaveClass("bg-amber-500/10 text-amber-600");
  });

  it("renders a red Evaluation tag when evaluation_status is failed", () => {
    render(<JobCard userJob={evaluatedJob({ evaluation_status: "failed" })} />);
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags[1]).toHaveClass("bg-destructive/10 text-destructive");
  });

  it("renders an Analyzed tag colored by job.status first", () => {
    render(<JobCard userJob={mockEvaluatedUserJob} />);
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags[0]).toHaveTextContent("Analyzed");
    expect(tags[0]).toHaveClass("bg-emerald-500/10 text-emerald-600");
  });

  it("renders a Report tag colored by report_status", () => {
    render(<JobCard userJob={mockEvaluatedUserJob} />);
    const reportTag = screen.getAllByTestId("evaluation-status-tag")[2];
    expect(reportTag).toHaveClass("bg-emerald-500/10 text-emerald-600");
    expect(reportTag).toHaveTextContent("Report");
  });

  it("renders a Resume tag colored by resume_status", () => {
    render(<JobCard userJob={mockEvaluatedUserJob} />);
    const resumeTag = screen.getAllByTestId("evaluation-status-tag")[3];
    expect(resumeTag).toHaveClass("bg-emerald-500/10 text-emerald-600");
    expect(resumeTag).toHaveTextContent("Resume");
  });

  it("omits a tag when its status is undefined", () => {
    render(
      <JobCard
        userJob={evaluatedJob({ report_status: undefined, resume_status: undefined })}
      />
    );
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags).toHaveLength(2);
    expect(tags[0]).toHaveTextContent("Analyzed");
    expect(tags[1]).toHaveTextContent("Evaluation");
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
      "bg-destructive/10 text-destructive"
    );
  });

  it("colors each recommendation label", () => {
    const labels: Array<[string, string]> = [
      ["strong_match", "bg-emerald-500/10 text-emerald-600"],
      ["worth_applying", "bg-blue-500/10 text-blue-600"],
      ["possible_match", "bg-amber-500/10 text-amber-600"],
      ["low_match", "bg-destructive/10 text-destructive"],
    ];
    for (const [label, expectedClass] of labels) {
      render(
        <JobCard
          userJob={evaluatedJob({ recommendation_label: label })}
        />
      );
      const tags = screen.getAllByTestId("recommendation-tag");
      const tag = tags[tags.length - 1];
      expect(tag).toHaveTextContent(label.replace("_", " "));
      expect(tag).toHaveClass(expectedClass);
    }
  });

  it("renders no evaluation tags when there is no evaluation", () => {
    render(<JobCard userJob={mockUserJob} />);
    expect(screen.queryByTestId("evaluation-status-tag")).not.toBeInTheDocument();
    expect(screen.queryByTestId("eligibility-tag")).not.toBeInTheDocument();
    expect(screen.queryByTestId("recommendation-tag")).not.toBeInTheDocument();
  });
});
