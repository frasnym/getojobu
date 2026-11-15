import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import React from "react";
import { JobDetailTab } from "@/components/jobs/job-detail-tab";
import { Job } from "@/lib/api/types";

const sampleJob: Job = {
  id: 1,
  slug: "senior-go-engineer-stripe-remote",
  status: "analyzed",
  title: "Senior Go Engineer",
  company: "Stripe",
  location: "Remote, US/EU",
  is_remote: true,
  apply_url: "https://stripe.com/jobs/senior-go-infra",
  posted_date: "2026-11-13T10:00:00Z",
  created_at: "2026-11-13T10:00:00Z",
  description: `### About Stripe\nStripe is a financial infrastructure platform.\n\n### Responsibilities\n- Build high-throughput Go services.`,
};

describe("JobDetailTab component", () => {
  it("renders role description, attributes, and apply button with safe attributes", () => {
    render(<JobDetailTab job={sampleJob} />);
    expect(screen.getByText("Role Description")).toBeInTheDocument();
    expect(screen.getByText(/About Stripe/)).toBeInTheDocument();
    expect(screen.getByText(/Build high-throughput Go services/)).toBeInTheDocument();

    const applyButton = screen.getByRole("link", { name: /Apply on Company Site/i });
    expect(applyButton).toHaveAttribute("href", "https://stripe.com/jobs/senior-go-infra");
    expect(applyButton).toHaveAttribute("target", "_blank");
    expect(applyButton).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("handles empty description with clean fallback message", () => {
    render(<JobDetailTab job={{ ...sampleJob, description: undefined, apply_url: undefined }} />);
    expect(
      screen.getByText(/No detailed description provided for this listing/i)
    ).toBeInTheDocument();
  });
});
