import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import React from "react";

const { mockUserJob } = vi.hoisted(() => ({
  mockUserJob: {
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
    evaluation: {
      id: "eval-1",
      evaluation_status: "success",
      eligibility_status: "eligible",
      recommendation_label: "strong_match",
      report_status: "success",
      resume_status: "success",
    },
  },
}));

vi.mock("@/hooks/use-user-jobs", () => ({
  useUserJobs: () => ({
    filteredJobs: [mockUserJob],
    pagination: null,
    page: 1,
    setPage: vi.fn(),
    limit: 10,
    setLimit: vi.fn(),
    loading: false,
    error: null,
    searchQuery: "",
    setSearchQuery: vi.fn(),
    statusFilter: "all",
    setStatusFilter: vi.fn(),
    remoteOnly: false,
    setRemoteOnly: vi.fn(),
    hideFilter: "all",
    setHideFilter: vi.fn(),
    hideJob: vi.fn(),
    bulkHideJob: vi.fn(),
    retryJob: vi.fn(),
    refetch: vi.fn(),
    isFiltersChanged: false,
    queryAll: vi.fn(),
    evaluationStatusFilter: "all",
    setEvaluationStatusFilter: vi.fn(),
    reportStatusFilter: "all",
    setReportStatusFilter: vi.fn(),
    eligibilityStatusFilter: "all",
    setEligibilityStatusFilter: vi.fn(),
    recommendationLabelFilter: "all",
    setRecommendationLabelFilter: vi.fn(),
  }),
}));

import UserJobsPage from "@/app/(app)/user-jobs/page";

describe("UserJobsPage", () => {
  it("does not show a resume download button on the job list", () => {
    render(<UserJobsPage />);

    expect(screen.getByText("Senior Go Engineer")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Download tailored resume/i })).not.toBeInTheDocument();
  });
});
