import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import {
  getJobs,
  getJobBySlug,
  getResumeDownloadUrl,
  triggerJobCompatibility,
} from "@/lib/api/endpoints";

const originalFetch = global.fetch;

beforeEach(() => {});

afterEach(() => {
  global.fetch = originalFetch;
});

function mockFetch(response: unknown, status = 200) {
  global.fetch = vi.fn(async () =>
    new Response(JSON.stringify(response), {
      status,
      headers: { "Content-Type": "application/json" },
    })
  ) as any;
}

describe("API endpoints and mock adapter", () => {
  it("getJobs returns a list of jobs with match score information and pagination", async () => {
    const mockJobsData = [
      {
        id: 1,
        slug: "senior-go-engineer-stripe-remote",
        title: "Senior Go Engineer - Infrastructure",
        company: "Stripe",
        location: "Remote, US/EU",
        status: "analyzed",
        created_at: "2026-11-13T10:00:00Z",
      },
    ];
    mockFetch({
      success: true,
      data: mockJobsData,
      meta: { timestamp: "2026-11-15T12:00:00Z", requestId: "req-123", pagination: { page: 1, limit: 10, totalItems: 1, totalPages: 1, hasNext: false, hasPrev: false } },
      error: null,
    });

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
    mockFetch({
      success: true,
      data: {
        id: 1,
        slug: "senior-go-engineer-stripe-remote",
        title: "Senior Go Engineer - Infrastructure",
        company: "Stripe",
        location: "Remote, US/EU",
        status: "analyzed",
        created_at: "2026-11-13T10:00:00Z",
      },
      meta: { timestamp: "2026-11-15T12:00:00Z", requestId: "req-123" },
      error: null,
    });

    const job = await getJobBySlug("senior-go-engineer-stripe-remote");
    expect(job).toBeDefined();
    expect(job.title).toContain("Senior Go");
    expect(job.company).toBe("Stripe");
  });

  it("getResumeDownloadUrl returns presigned download URL and metadata", async () => {
    mockFetch({
      success: true,
      data: {
        download_url: "https://storage.getojobu.com/resumes/signed-download.pdf?signature=valid",
        expires_at: new Date(Date.now() + 3600 * 1000).toISOString(),
        filename: "Alex_Rivera_Stripe_Tailored_Resume.pdf",
      },
      meta: { timestamp: "2026-11-15T12:00:00Z", requestId: "req-123" },
      error: null,
    });

    const download = await getResumeDownloadUrl("eval-stripe-001");
    expect(download.download_url).toMatch(/^https?:\/\//);
    expect(download.filename).toContain(".pdf");
  });

  it("triggerJobCompatibility initiates/returns evaluation summary", async () => {
    mockFetch({
      success: true,
      data: [
        {
          id: "eval-senior-go-engineer-stripe-remote-new",
          user_job_id: "uj-001",
          evaluation_status: "success",
          overall_score: 85.0,
          eligibility_status: "eligible",
          recommendation_label: "strong_match",
          report_status: "ready",
          resume_status: "ready",
          created_at: new Date().toISOString(),
        },
      ],
      meta: { timestamp: "2026-11-15T12:00:00Z", requestId: "req-123" },
      error: null,
    });

    const summary = await triggerJobCompatibility("senior-go-engineer-stripe-remote", false);
    expect(summary.id).toBeDefined();
    expect(summary.evaluation_status).toBe("success");
  });

  it("unwraps canonical backend response envelope when NEXT_PUBLIC_ENABLE_MOCK=false", async () => {
    process.env.NEXT_PUBLIC_ENABLE_MOCK = "false";
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

    mockFetch({
      success: true,
      data: mockJobsData,
      meta: { timestamp: "2026-11-15T12:00:00Z", requestId: "req-123" },
      error: null,
    });

    try {
      const res = await getJobs();
      expect(Array.isArray(res.data)).toBe(true);
      expect(res.data).toHaveLength(1);
      expect(res.data[0].title).toBe("Staff Platform Engineer");
    } finally {
      process.env.NEXT_PUBLIC_ENABLE_MOCK = "true";
    }
  });

  it("extracts error message and code from canonical error envelope", async () => {
    process.env.NEXT_PUBLIC_ENABLE_MOCK = "false";
      {
        success: false,
        data: null,
        meta: { timestamp: "2026-11-15T12:00:00Z", requestId: "req-err" },
        error: { code: "UNAUTHORIZED", message: "unauthenticated user token" },
      },
      401
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
      process.env.NEXT_PUBLIC_ENABLE_MOCK = "true";
    }
  });

  it("throws error when backend fails", async () => {
    process.env.NEXT_PUBLIC_ENABLE_MOCK = "false";
    global.fetch = vi.fn(async () => {
      throw new Error("Network connection refused");
    }) as any;

    try {
      await expect(getJobs()).rejects.toThrow("Network connection refused");
      await expect(getJobBySlug("some-slug")).rejects.toThrow("Network connection refused");
    } finally {
      global.fetch = originalFetch;
      process.env.NEXT_PUBLIC_ENABLE_MOCK = "true";
    }
  });
});
