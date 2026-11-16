import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import React from "react";
import { MatchReportTab } from "@/components/matching/match-report-tab";
import { MatchReport } from "@/lib/api/types";

const sampleReport: MatchReport = {
  id: "eval-001",
  user_job_id: "uj-001",
  evaluation_status: "success",
  overall_score: 88.5,
  eligibility_status: "eligible",
  recommendation_label: "strong_match",
  report_data: {
    summary: "The candidate demonstrates exceptional Go expertise and system reliability.",
    strengths: ["6+ years production Go experience", "High throughput microservices"],
    gaps: [
      {
        requirement: "Direct Kafka experience",
        status: "not_found_in_cv",
        suggestion: "Highlight transferable messaging experience in RabbitMQ or Redis",
      },
    ],
    cv_improvements: ["Quantify TPS in previous roles"],
  },
  evaluation_data: {
    evaluations: [
      {
        category: "Core Go Mastery",
        score: 95,
        reason: "Deep understanding of Go concurrency",
        strengths: ["Concurrency primitives"],
        gaps: [],
        evidence: [
          {
            candidate_evidence: "Built payment gateway handling 15,000 req/sec",
            job_requirement: "5+ years production Go experience",
            relationship: "direct",
          },
        ],
      },
    ],
    mandatory_requirements: [
      {
        requirement: "Authorized to work remotely",
        status: "met",
        reason: "Candidate profile confirms valid status",
      },
    ],
  },
};

describe("MatchReportTab component", () => {
  it("renders score gauge, eligibility, categories, and evidence", () => {
    render(
      <MatchReportTab
        evalId="eval-001"
        report={sampleReport}
        loading={false}
      />
    );

    expect(screen.getByText(/88(\.5)?%/)).toBeInTheDocument();
    expect(screen.getByText(/Eligible/i)).toBeInTheDocument();
    expect(screen.getByText("Core Go Mastery")).toBeInTheDocument();
    expect(screen.getByText(/Built payment gateway handling 15,000 req\/sec/i)).toBeInTheDocument();
    expect(screen.getByText("direct")).toBeInTheDocument();
    expect(screen.getByText(/6\+ years production Go experience/i)).toBeInTheDocument();
    expect(screen.getByText(/Direct Kafka experience/i)).toBeInTheDocument();
  });

  it("renders structured gap objects with requirement, status, and suggestion without crashing", () => {
    const reportWithStructuredGaps: MatchReport = {
      ...sampleReport,
      report_data: {
        ...sampleReport.report_data,
        gaps: [
          {
            requirement: "Kubernetes cluster administration",
            status: "not_found_in_cv",
            suggestion: "Highlight container orchestration or Docker experience if any",
          },
          {
            requirement: "AWS Cloud Architecture",
            status: "partially_met",
            suggestion: "Emphasize transferable GCP and on-prem cloud experience",
          },
        ],
      },
    };

    render(
      <MatchReportTab
        evalId="eval-001"
        report={reportWithStructuredGaps}
        loading={false}
      />
    );

    expect(screen.getByText("Kubernetes cluster administration")).toBeInTheDocument();
    expect(screen.getByText("Not found in CV")).toBeInTheDocument();
    expect(
      screen.getByText("Highlight container orchestration or Docker experience if any")
    ).toBeInTheDocument();

    expect(screen.getByText("AWS Cloud Architecture")).toBeInTheDocument();
    expect(screen.getByText("Partially met")).toBeInTheDocument();
    expect(
      screen.getByText("Emphasize transferable GCP and on-prem cloud experience")
    ).toBeInTheDocument();
  });

  it("renders empty state with analyze button when no evaluation exists", () => {
    const handleAnalyze = vi.fn();
    render(
      <MatchReportTab
        evalId={undefined}
        report={null}
        loading={false}
        onAnalyzeClick={handleAnalyze}
      />
    );

    expect(
      screen.getByText(/This job has not been evaluated against your career profile yet/i)
    ).toBeInTheDocument();
    const btn = screen.getByRole("button", { name: /Analyze Compatibility/i });
    fireEvent.click(btn);
    expect(handleAnalyze).toHaveBeenCalled();
  });
});
