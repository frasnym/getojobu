import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import React from "react";

// Keep getToken referentially stable across renders so the load effect does
// not re-run on every state update (real Clerk memoizes this callback).
const { stableGetToken } = vi.hoisted(() => ({
  stableGetToken: vi.fn().mockResolvedValue("test-token"),
}));

vi.mock("@clerk/nextjs", () => ({
  useAuth: () => ({ getToken: stableGetToken, isLoaded: true }),
}));

vi.mock("@/lib/api/endpoints", () => ({
  getJobBySlug: vi.fn(),
  getJobs: vi.fn(),
  triggerJobCompatibility: vi.fn(),
}));

import { useJobDetail } from "@/hooks/use-job-detail";
import * as api from "@/lib/api/endpoints";
import { Job } from "@/lib/api/types";

const sampleJob: Job = {
  id: 1,
  slug: "go-stripe",
  title: "Senior Go Engineer",
  company: "Stripe",
  location: "Remote",
  is_remote: true,
  status: "analyzed",
  created_at: "2026-11-13T10:00:00Z",
  user_job: { id: "uj-1" },
};

describe("useJobDetail manual refresh", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.getJobBySlug).mockResolvedValue(sampleJob);
    vi.mocked(api.getJobs).mockResolvedValue({
      data: [sampleJob],
      pagination: {
        page: 1,
        limit: 10,
        totalItems: 1,
        totalPages: 1,
        hasNext: false,
        hasPrev: false,
      },
    });
  });

  it("exposes an awaitable refresh and captures refetch failures", async () => {
    const { result } = renderHook(() => useJobDetail("go-stripe"));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.job?.slug).toBe("go-stripe");

    vi.mocked(api.getJobBySlug).mockRejectedValueOnce(new Error("network down"));
    await act(async () => {
      await result.current.refresh();
    });

    expect(result.current.refreshError?.message).toBe("network down");
    expect(result.current.refreshing).toBe(false);
    // A failed refetch keeps the last good job instead of clearing the page.
    expect(result.current.job?.slug).toBe("go-stripe");
  });

  it("clears a previous refresh error after a successful refetch", async () => {
    const { result } = renderHook(() => useJobDetail("go-stripe"));
    await waitFor(() => expect(result.current.loading).toBe(false));

    vi.mocked(api.getJobBySlug).mockRejectedValueOnce(new Error("network down"));
    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.refreshError).not.toBeNull();

    vi.mocked(api.getJobBySlug).mockResolvedValueOnce({ ...sampleJob, title: "Updated Title" });
    await act(async () => {
      await result.current.refresh();
    });

    expect(result.current.refreshError).toBeNull();
    expect(result.current.job?.title).toBe("Updated Title");
  });
});
