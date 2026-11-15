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

  it("getJobs returns a list of jobs with match score information and pagination", async () => {
    const res = await getJobs();
    expect(Array.isArray(res.data)).toBe(true);
    expect(res.data.length).toBeGreaterThan(0);
    expect(res.pagination).toBeDefined();
    expect(res.pagination.page).toBe(1);
    expect(res.pagination.limit).toBe(10);
    const first = res.data[0];
    expect(first).toHaveProperty("slug");
    expect(first).toHaveProperty("title");
    expect(first).toHaveProperty("company");
    expect(first).toHaveProperty("status");
  });

  it("getJobBySlug returns full job details", async () => {
    const job = await getJobBySlug("senior-go-engineer-stripe-remote");
    expect(job).toBeDefined();
    expect(job.title).toContain("Senior Go");
    expect(job.company).toBe("Stripe");
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

  it("unwraps canonical backend response envelope when NEXT_PUBLIC_ENABLE_MOCK=false", async () => {
    process.env.NEXT_PUBLIC_ENABLE_MOCK = "false";
    const originalFetch = global.fetch;
    const mockJobsData = [
      {
        id: 42,
        slug: "staff-platform-eng",
        title: "Staff Platform Engineer",
        company: "Acme Corp",
        location: "Remote",
        status: "analyzed",
        created_at: "2026-11-15T00:00:00Z",
      },
    ];

    global.fetch = async () =>
      new Response(
        JSON.stringify({
          success: true,
          data: mockJobsData,
          meta: { timestamp: "2026-11-15T12:00:00Z", requestId: "req-123" },
          error: null,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );

    try {
      const res = await getJobs();
      expect(Array.isArray(res.data)).toBe(true);
      expect(res.data).toHaveLength(1);
      expect(res.data[0].title).toBe("Staff Platform Engineer");
    } finally {
      global.fetch = originalFetch;
      process.env.NEXT_PUBLIC_ENABLE_MOCK = "true";
    }
  });

  it("extracts error message and code from canonical error envelope", async () => {
    process.env.NEXT_PUBLIC_ENABLE_MOCK = "false";
    const originalFetch = global.fetch;
    global.fetch = async () =>
      new Response(
        JSON.stringify({
          success: false,
          data: null,
          meta: { timestamp: "2026-11-15T12:00:00Z", requestId: "req-err" },
          error: {
            code: "UNAUTHORIZED",
            message: "unauthenticated user token",
          },
        }),
        { status: 401, headers: { "Content-Type": "application/json" } }
      );

    try {
      let caughtError: any;
      try {
        await getJobs();
      } catch (err) {
        caughtError = err;
      }
      expect(caughtError).toBeDefined();
      expect(caughtError.message).toBe("unauthenticated user token");
      expect(caughtError.code).toBe("UNAUTHORIZED");
      expect(caughtError.status).toBe(401);
    } finally {
      global.fetch = originalFetch;
      process.env.NEXT_PUBLIC_ENABLE_MOCK = "true";
    }
  });

  it("throws error when NEXT_PUBLIC_ENABLE_MOCK=false and backend fails", async () => {
    process.env.NEXT_PUBLIC_ENABLE_MOCK = "false";
    const originalFetch = global.fetch;
    global.fetch = async () => {
      throw new Error("Network connection refused");
    };

    try {
      await expect(getJobs()).rejects.toThrow("Network connection refused");
      await expect(getJobBySlug("some-slug")).rejects.toThrow("Network connection refused");
    } finally {
      global.fetch = originalFetch;
      process.env.NEXT_PUBLIC_ENABLE_MOCK = "true";
    }
  });
});
