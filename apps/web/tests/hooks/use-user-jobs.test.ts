import { describe, it, expect, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { useUserJobs } from "@/hooks/use-user-jobs";
import * as endpoints from "@/lib/api/endpoints";
import { Job } from "@/lib/api/types";

vi.mock("@clerk/nextjs", () => ({
  useAuth: () => ({
    getToken: vi.fn().mockResolvedValue("test-token"),
    isLoaded: true,
  }),
}));

const mockUserJobs: Job[] = [
  {
    id: 1,
    slug: "go-stripe",
    title: "Senior Go Engineer",
    company: "Stripe",
    location: "Remote",
    status: "analyzed",
    is_owned: true,
    created_at: "2026-11-13T10:00:00Z",
  },
];

describe("useUserJobs hook", () => {
  it("fetches user jobs and returns filtered list", async () => {
    vi.spyOn(endpoints, "getUserJobs").mockResolvedValue({
      data: mockUserJobs,
      pagination: {
        page: 1,
        limit: 10,
        totalItems: mockUserJobs.length,
        totalPages: 1,
        hasNext: false,
        hasPrev: false,
      },
    });

    const { result } = renderHook(() => useUserJobs());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.jobs).toHaveLength(1);
    expect(result.current.filteredJobs).toHaveLength(1);
    expect(result.current.jobs[0].title).toBe("Senior Go Engineer");
    expect(result.current.pagination?.totalItems).toBe(1);
  });

  it("handles empty or non-array returns gracefully", async () => {
    vi.spyOn(endpoints, "getUserJobs").mockResolvedValue({
      data: [] as Job[],
      pagination: {
        page: 1,
        limit: 10,
        totalItems: 0,
        totalPages: 1,
        hasNext: false,
        hasPrev: false,
      },
    });

    const { result } = renderHook(() => useUserJobs());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.jobs).toEqual([]);
    expect(result.current.filteredJobs).toEqual([]);
    expect(result.current.error).toBeNull();
  });
});
