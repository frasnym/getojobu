import { describe, it, expect } from "vitest";
import { filterJobs } from "@/hooks/use-jobs";
import { Job } from "@/lib/api/types";

const sampleJobs: Job[] = [
  {
    id: 1,
    slug: "go-stripe",
    title: "Senior Go Engineer",
    company: "Stripe",
    location: "Remote",
    status: "analyzed",
    created_at: "2026-11-13T10:00:00Z",
    latest_evaluation: {
      id: "e1",
      evaluation_status: "success",
      overall_score: 90,
      eligibility_status: "eligible",
    },
  },
  {
    id: 2,
    slug: "cplusplus-lead",
    title: "Lead C++ Systems Architect",
    company: "Cloudflare",
    location: "Austin, TX",
    status: "pending",
    created_at: "2026-11-14T10:00:00Z",
    latest_evaluation: null,
  },
  {
    id: 3,
    slug: "frontend-vercel",
    title: "Staff Frontend Engineer",
    company: "Vercel",
    location: "Remote",
    status: "failed",
    created_at: "2026-11-12T10:00:00Z",
    latest_evaluation: null,
  },
];

describe("filterJobs helper", () => {
  it("filters by query matching title or company", () => {
    const results = filterJobs(sampleJobs, "stripe", "all");
    expect(results).toHaveLength(1);
    expect(results[0].company).toBe("Stripe");
  });

  it("handles regex special characters safely without crashing", () => {
    const results = filterJobs(sampleJobs, "c++", "all");
    expect(results).toHaveLength(1);
    expect(results[0].title).toBe("Lead C++ Systems Architect");
  });

  it("filters by status pill correctly", () => {
    const analyzed = filterJobs(sampleJobs, "", "analyzed");
    expect(analyzed).toHaveLength(1);
    expect(analyzed[0].status).toBe("analyzed");

    const pending = filterJobs(sampleJobs, "", "pending");
    expect(pending).toHaveLength(1);
    expect(pending[0].status).toBe("pending");
  });

  it("handles non-array inputs defensively without throwing TypeError", () => {
    // If backend envelope or null/undefined is passed, should return empty array safely
    expect(() => filterJobs(null as any, "", "all")).not.toThrow();
    expect(filterJobs(null as any, "", "all")).toEqual([]);

    expect(() => filterJobs(undefined as any, "", "all")).not.toThrow();
    expect(filterJobs(undefined as any, "", "all")).toEqual([]);

    expect(() => filterJobs({ success: true, data: [] } as any, "", "all")).not.toThrow();
    expect(filterJobs({ success: true, data: [] } as any, "", "all")).toEqual([]);
  });
});
