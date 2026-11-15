import { describe, it, expect, beforeEach } from "vitest";
import {
  getJobs,
  getJobBySlug,
  getMatchReport,
  getResumeDownloadUrl,
  triggerJobCompatibility,
} from "@/lib/api/endpoints";

describe("API endpoints and mock adapter", () => {
  beforeEach(() => {
    // Force mock mode for unit tests
    process.env.NEXT_PUBLIC_ENABLE_MOCK = "true";
  });

  it("getJobs returns a list of jobs with match score information", async () => {
    const jobs = await getJobs();
    expect(Array.isArray(jobs)).toBe(true);
    expect(jobs.length).toBeGreaterThan(0);
    const first = jobs[0];
    expect(first).toHaveProperty("slug");
    expect(first).toHaveProperty("title");
    expect(first).toHaveProperty("company");
    expect(first).toHaveProperty("status");
  });

  it("getJobBySlug returns full job details with latest evaluation", async () => {
    const job = await getJobBySlug("senior-go-engineer-stripe-remote");
    expect(job).toBeDefined();
    expect(job.title).toContain("Senior Go");
    expect(job.company).toBe("Stripe");
    expect(job.latest_evaluation).toBeDefined();
    expect(job.latest_evaluation?.overall_score).toBe(88.5);
    expect(job.latest_evaluation?.eligibility_status).toBe("eligible");
  });

  it("getMatchReport returns complete score breakdown, categories, and evidence", async () => {
    const report = await getMatchReport("eval-stripe-001");
    expect(report.id).toBe("eval-stripe-001");
    expect(report.overall_score).toBe(88.5);
    expect(report.eligibility_status).toBe("eligible");
    expect(report.evaluation_data?.evaluations.length).toBeGreaterThan(0);
    expect(report.report_data?.strengths?.length).toBeGreaterThan(0);
    expect(report.report_data?.cv_improvements?.length).toBeGreaterThan(0);
  });

  it("getResumeDownloadUrl returns presigned download URL and metadata", async () => {
    const download = await getResumeDownloadUrl("eval-stripe-001");
    expect(download.download_url).toMatch(/^https?:\/\//);
    expect(download.filename).toContain(".pdf");
  });

  it("triggerJobCompatibility initiates/returns evaluation summary", async () => {
    const summary = await triggerJobCompatibility("senior-go-engineer-stripe-remote", false);
    expect(summary.id).toBeDefined();
    expect(summary.evaluation_status).toBe("success");
  });
});
