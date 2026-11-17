import { describe, it, expect } from "vitest";
import { filterJobs, statusBadgeClasses } from "@/hooks/use-jobs";
import { Job } from "@/lib/api/types";

const sampleJobs: Job[] = [
  {
    id: 1,
    slug: "go-stripe",
    title: "Senior Go Engineer",
    company: "Stripe",
    location: "Remote",
    is_remote: true,
    status: "analyzed",
    created_at: "2026-11-13T10:00:00Z",
  },
  {
    id: 2,
    slug: "cplusplus-lead",
    title: "Lead C++ Systems Architect",
    company: "Cloudflare",
    location: "Austin, TX",
    is_remote: false,
    status: "analyze_pending",
    created_at: "2026-11-14T10:00:00Z",
  },
  {
    id: 3,
    slug: "frontend-vercel",
    title: "Staff Frontend Engineer",
    company: "Vercel",
    location: "Remote",
    is_remote: true,
    status: "failed",
    created_at: "2026-11-12T10:00:00Z",
  },
  {
    id: 4,
    slug: "queued-job",
    title: "Platform Engineer",
    company: "GitHub",
    location: "Remote",
    is_remote: true,
    status: "queued",
    created_at: "2026-11-15T10:00:00Z",
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

    const pending = filterJobs(sampleJobs, "", "analyze_pending");
    expect(pending).toHaveLength(1);
    expect(pending[0].status).toBe("analyze_pending");

    const queued = filterJobs(sampleJobs, "", "queued");
    expect(queued).toHaveLength(1);
    expect(queued[0].status).toBe("queued");
  });

  it("applies sky badge class for queued statuses", () => {
    expect(statusBadgeClasses("queued")).toContain("text-sky-600");
    expect(statusBadgeClasses("queued_retry")).toContain("text-sky-600");
  });

  it("filters by remoteOnly flag correctly", () => {
    const remoteJobs = filterJobs(sampleJobs, "", "all", true);
    expect(remoteJobs).toHaveLength(3);
    expect(remoteJobs.every((j) => j.is_remote === true)).toBe(true);

    const nonRemoteSearch = filterJobs(sampleJobs, "cloudflare", "all", true);
    expect(nonRemoteSearch).toHaveLength(0);
  });

  it("filters by owned status correctly", () => {
    const withOwned = sampleJobs.map((j, i) => ({
      ...j,
      is_owned: i === 1,
    }));

    const owned = filterJobs(withOwned, "", "all", false, "owned");
    expect(owned).toHaveLength(1);
    expect(owned[0].is_owned).toBe(true);

    const notOwned = filterJobs(withOwned, "", "all", false, "not_owned");
    expect(notOwned).toHaveLength(3);
    expect(notOwned.every((j) => j.is_owned !== true)).toBe(true);

    const all = filterJobs(withOwned, "", "all", false, "all");
    expect(all).toHaveLength(4);
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
