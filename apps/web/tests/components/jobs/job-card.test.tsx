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

  it("renders a blue Evaluation tag with a clock icon when evaluation_status is pending", () => {
    render(<JobCard userJob={evaluatedJob({ evaluation_status: "pending" })} />);
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags[1]).toHaveClass("bg-sky-500/10 text-sky-600");
    expect(tags[1].querySelector(".lucide-clock")).not.toBeNull();
  });

  it("renders a sky Evaluation tag with a clock icon when evaluation_status is queued", () => {
    render(<JobCard userJob={evaluatedJob({ evaluation_status: "queued" })} />);
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags[1]).toHaveClass("bg-sky-500/10 text-sky-600");
    expect(tags[1].querySelector(".lucide-clock")).not.toBeNull();
  });

  it("renders a red Evaluation tag when evaluation_status is failed", () => {
    render(<JobCard userJob={evaluatedJob({ evaluation_status: "failed" })} />);
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags[1]).toHaveClass("bg-red-500/10 text-red-400 dark:text-red-300");
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
      "bg-red-500/10 text-red-400 dark:text-red-300"
    );
  });

  it("shows eligibility and recommendation beside the remote badge, not in the status row", () => {
    render(
      <JobCard
        userJob={evaluatedJob({
          eligibility_status: "eligible",
          recommendation_label: "strong_match",
        })}
      />
    );

    const header = screen.getByText("Remote").parentElement;
    const eligibility = screen.getByTestId("eligibility-tag");
    const recommendation = screen.getByTestId("recommendation-tag");
    const statusRow = screen.getAllByTestId("evaluation-status-tag")[0].parentElement;

    expect(header).toContainElement(eligibility);
    expect(header).toContainElement(recommendation);
    expect(statusRow).not.toContainElement(eligibility);
    expect(statusRow).not.toContainElement(recommendation);
  });

  it("uses the same green text color for remote, eligibility and recommendation tags", () => {
    render(
      <JobCard
        userJob={evaluatedJob({
          eligibility_status: "eligible",
          recommendation_label: "strong_match",
        })}
      />
    );

    const expected = ["bg-emerald-500/10", "text-emerald-600", "dark:text-emerald-400"];
    expect(screen.getByText("Remote")).toHaveClass(...expected);
    expect(screen.getByTestId("eligibility-tag")).toHaveClass(...expected);
    expect(screen.getByTestId("recommendation-tag")).toHaveClass(...expected);
  });

  it("colors each recommendation label", () => {
    const labels: Array<[string, string]> = [
      ["strong_match", "bg-emerald-500/10 text-emerald-600"],
      ["worth_applying", "bg-blue-500/10 text-blue-600"],
      ["possible_match", "bg-amber-500/10 text-amber-600"],
      ["low_match", "bg-red-500/10 text-red-400 dark:text-red-300"],
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

  it("renders only the job status tag when there is no evaluation", () => {
    render(<JobCard userJob={mockUserJob} />);
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags).toHaveLength(1);
    expect(tags[0]).toHaveTextContent("Analyzed");
    expect(screen.queryByTestId("eligibility-tag")).not.toBeInTheDocument();
    expect(screen.queryByTestId("recommendation-tag")).not.toBeInTheDocument();
  });

  it("renders the job status tag for a job with any status and no evaluation", () => {
    const queuedJob: UserJob = {
      ...mockUserJob,
      job: { ...mockUserJob.job, status: "analyze_pending" },
    };
    render(<JobCard userJob={queuedJob} />);
    const tags = screen.getAllByTestId("evaluation-status-tag");
    expect(tags).toHaveLength(1);
    expect(tags[0]).toHaveTextContent("Analyze Pending");
    expect(tags[0]).toHaveClass("bg-sky-500/10 text-sky-600");
    expect(screen.queryByTestId("eligibility-tag")).not.toBeInTheDocument();
  });
});

describe("JobCard detail access", () => {
  it("opens the detail drawer on click by default", () => {
    const onSelect = vi.fn();
    render(<JobCard userJob={mockUserJob} onSelect={onSelect} />);
    fireEvent.click(screen.getByText("Senior Go Engineer"));
    expect(onSelect).toHaveBeenCalledWith(mockUserJob);
  });

  it("does not open detail when canOpenDetail is false", () => {
    const onSelect = vi.fn();
    const pendingJob: UserJob = {
      ...mockUserJob,
      job: { ...mockUserJob.job, status: "analyze_pending" },
    };
    render(<JobCard userJob={pendingJob} onSelect={onSelect} canOpenDetail={false} />);
    fireEvent.click(screen.getByText("Senior Go Engineer"));
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("hides the Details affordance when canOpenDetail is false", () => {
    const pendingJob: UserJob = {
      ...mockUserJob,
      job: { ...mockUserJob.job, status: "queued" },
    };
    render(<JobCard userJob={pendingJob} canOpenDetail={false} />);
    expect(screen.queryByText(/Details/i)).not.toBeInTheDocument();
  });

  it("renders the title as a link when used standalone with detail allowed", () => {
    render(<JobCard userJob={mockUserJob} />);
    expect(screen.getByRole("link", { name: /Senior Go Engineer/i })).toHaveAttribute(
      "href",
      "/user-jobs/senior-go-engineer-stripe-remote"
    );
  });
});

describe("JobCard retry", () => {
  const failedJob: UserJob = {
    ...mockUserJob,
    job: { ...mockUserJob.job, status: "failed" },
  };

  it("labels a failed job as Retry Analyze Job", () => {
    render(<JobCard userJob={failedJob} onRetry={vi.fn()} />);
    expect(screen.getByRole("button", { name: /Retry Analyze Job/i })).toBeInTheDocument();
  });

  it("labels a failed evaluation as Retry Evaluate Job", () => {
    const evalFailedJob: UserJob = {
      ...mockUserJob,
      evaluation: { id: "eval-1", evaluation_status: "failed", report_status: "success" },
    };
    render(<JobCard userJob={evalFailedJob} onRetry={vi.fn()} />);
    expect(screen.getByRole("button", { name: /Retry Evaluate Job/i })).toBeInTheDocument();
  });

  it("labels a failed report as Retry Generate Report", () => {
    const reportFailedJob: UserJob = {
      ...mockUserJob,
      evaluation: { id: "eval-1", evaluation_status: "success", report_status: "failed" },
    };
    render(<JobCard userJob={reportFailedJob} onRetry={vi.fn()} />);
    expect(screen.getByRole("button", { name: /Retry Generate Report/i })).toBeInTheDocument();
  });

  it("labels a failed resume as Retry Generate Resume", () => {
    const resumeFailedJob: UserJob = {
      ...mockUserJob,
      evaluation: {
        id: "eval-1",
        evaluation_status: "success",
        report_status: "success",
        resume_status: "failed",
      },
    };
    render(<JobCard userJob={resumeFailedJob} onRetry={vi.fn()} />);
    expect(screen.getByRole("button", { name: /Retry Generate Resume/i })).toBeInTheDocument();
  });

  it("shows the resume failure message as a tooltip on the failed Resume tag", () => {
    const resumeFailedJob: UserJob = {
      ...mockUserJob,
      evaluation: {
        id: "eval-1",
        evaluation_status: "success",
        report_status: "success",
        resume_status: "failed",
        resume_failure_message: "skills must contain between 1 and 12 items",
      },
    };
    render(<JobCard userJob={resumeFailedJob} onRetry={vi.fn()} />);
    const resumeTag = screen.getAllByTestId("evaluation-status-tag")[3];
    expect(resumeTag).toHaveAttribute("title", "skills must contain between 1 and 12 items");
  });

  it("calls onRetry with the user job id without opening the detail view", () => {
    const onRetry = vi.fn();
    const onSelect = vi.fn();
    render(<JobCard userJob={failedJob} onRetry={onRetry} onSelect={onSelect} />);
    fireEvent.click(screen.getByRole("button", { name: /Retry Analyze Job/i }));
    expect(onRetry).toHaveBeenCalledWith("11111111-1111-1111-1111-111111111111");
    expect(onSelect).not.toHaveBeenCalled();
  });

  it("places the Retry button beside the bottom status tags", () => {
    render(<JobCard userJob={failedJob} onRetry={vi.fn()} />);
    const statusRow = screen.getAllByTestId("evaluation-status-tag")[0].parentElement;
    expect(statusRow).toContainElement(screen.getByRole("button", { name: /Retry Analyze Job/i }));
  });

  it("styles the Retry button like the Explore More Jobs button with a retry icon", () => {
    const { container } = render(<JobCard userJob={failedJob} onRetry={vi.fn()} />);
    const retryButton = screen.getByRole("button", { name: /Retry Analyze Job/i });
    expect(retryButton.className).toContain("border-border");
    expect(retryButton.className).toContain("bg-card");
    expect(retryButton.className).toContain("text-foreground");
    expect(retryButton.className).toContain("hover:bg-secondary");
    expect(retryButton.className).not.toContain("bg-primary");
    expect(container.querySelector(".lucide-rotate-ccw")).toBeInTheDocument();
  });

  it("does not show a Retry button for a non-failed job", () => {
    render(<JobCard userJob={mockUserJob} onRetry={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /^Retry /i })).not.toBeInTheDocument();
  });

  it("does not show a Retry button when no handler is provided", () => {
    render(<JobCard userJob={failedJob} />);
    expect(screen.queryByRole("button", { name: /^Retry /i })).not.toBeInTheDocument();
  });
});
