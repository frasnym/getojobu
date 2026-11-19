import { describe, it, expect, afterEach, vi } from "vitest";
import {
  getJobs,
  getJobBySlug,
  getResumeDownloadUrl,
  generateCoverLetter,
  triggerJobCompatibility,
  retryReport,
} from "@/lib/api/endpoints";

const originalFetch = global.fetch;

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

describe("API endpoints", () => {
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

  it("triggerJobCompatibility inserts then triggers and returns the triggered summary", async () => {
    const fetchMock = vi.fn(async () =>
      new Response(
        JSON.stringify({
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
        }),
        { status: 202, headers: { "Content-Type": "application/json" } }
      )
    ) as any;
    global.fetch = fetchMock;

    const summary = await triggerJobCompatibility("senior-go-engineer-stripe-remote", false);
    expect(summary.id).toBe("eval-senior-go-engineer-stripe-remote-new");
    expect(summary.evaluation_status).toBe("success");
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[0][0]).toContain("/v1/jobs/evaluation");
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual([
      { slug: "senior-go-engineer-stripe-remote" },
    ]);
    expect(fetchMock.mock.calls[1][0]).toContain(
      "/v1/matching-evaluations/bulk-generate-evaluation"
    );
    expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual([
      { id: "eval-senior-go-engineer-stripe-remote-new" },
    ]);
  });

  it("triggerJobCompatibility accepts an in-flight summary that carries an error", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            success: true,
            data: [
              {
                id: "eval-in-flight",
                user_job_id: "uj-001",
                evaluation_status: "new",
              },
            ],
            meta: { timestamp: "2026-11-19T12:00:00Z", requestId: "req-1" },
            error: null,
          }),
          { status: 202, headers: { "Content-Type": "application/json" } }
        )
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            success: true,
            data: [
              {
                id: "eval-in-flight",
                user_job_id: "uj-001",
                evaluation_status: "pending",
                error: "evaluation status changed before it could be queued",
              },
            ],
            meta: { timestamp: "2026-11-19T12:00:01Z", requestId: "req-2" },
            error: null,
          }),
          { status: 202, headers: { "Content-Type": "application/json" } }
        )
      ) as any;
    global.fetch = fetchMock;

    const summary = await triggerJobCompatibility("senior-go-engineer-stripe-remote", false);
    expect(summary.id).toBe("eval-in-flight");
    expect(summary.evaluation_status).toBe("pending");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("retryReport posts to the per-evaluation report retry endpoint", async () => {
    const fetchMock = vi.fn(async () =>
      new Response(
        JSON.stringify({
          success: true,
          data: [{ id: "eval-1", evaluation_status: "success", report_status: "queued_retry" }],
          meta: { timestamp: "2026-11-15T12:00:00Z", requestId: "req-123" },
          error: null,
        }),
        { status: 202, headers: { "Content-Type": "application/json" } }
      )
    ) as any;
    global.fetch = fetchMock;

    const summary = await retryReport("eval-1");
    expect(summary.report_status).toBe("queued_retry");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toContain(
      "/v1/matching-evaluations/eval-1/retry-report"
    );
    expect(fetchMock.mock.calls[0][1].method).toBe("POST");
  });

  it("unwraps canonical backend response envelope", async () => {
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

    const res = await getJobs();
    expect(Array.isArray(res.data)).toBe(true);
    expect(res.data).toHaveLength(1);
    expect(res.data[0].title).toBe("Staff Platform Engineer");
  });

  it("extracts error message and code from canonical error envelope", async () => {
    mockFetch(
      {
        success: false,
        data: null,
        meta: { timestamp: "2026-11-15T12:00:00Z", requestId: "req-err" },
        error: { code: "UNAUTHORIZED", message: "unauthenticated user token" },
      },
      401
    );

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
  });

  it("generateCoverLetter posts one item to the bulk endpoint", async () => {
    const summary = {
      id: "eval-001",
      cover_letter_status: "queued",
    };
    mockFetch({
      success: true,
      data: [summary],
      meta: { timestamp: "", requestId: "" },
      error: null,
    });

    const result = await generateCoverLetter("eval-001", "short", true);

    const [url, init] = (global.fetch as any).mock.calls[0];
    expect(url).toContain("/v1/matching-evaluations/bulk-generate-cover-letter");
    expect(JSON.parse(init.body)).toEqual([
      { id: "eval-001", variant: "short", force: true },
    ]);
    expect(result).toEqual(summary);
  });

  it("generateCoverLetter throws on a per-item error", async () => {
    mockFetch({
      success: true,
      data: [{ id: "eval-001", error: "evaluation must be successful" }],
      meta: { timestamp: "", requestId: "" },
      error: null,
    });

    await expect(generateCoverLetter("eval-001", "short", false)).rejects.toThrow(
      "evaluation must be successful"
    );
  });

  it("throws error when backend fails", async () => {
    global.fetch = vi.fn(async () => {
      throw new Error("Network connection refused");
    }) as any;

    try {
      await expect(getJobs()).rejects.toThrow("Network connection refused");
      await expect(getJobBySlug("some-slug")).rejects.toThrow("Network connection refused");
    } finally {
      global.fetch = originalFetch;
    }
  });
});
