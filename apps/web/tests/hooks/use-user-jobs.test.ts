import { describe, it, expect, vi } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { useUserJobs, filterUserJobs } from "@/hooks/use-user-jobs";
import * as endpoints from "@/lib/api/endpoints";
import { UserJob } from "@/lib/api/types";

const { getToken } = vi.hoisted(() => ({
  getToken: vi.fn().mockResolvedValue("test-token"),
}));

vi.mock("@clerk/nextjs", () => ({
  useAuth: () => ({
    getToken,
    isLoaded: true,
  }),
}));

const mockUserJobs: UserJob[] = [
  {
    id: "11111111-1111-1111-1111-111111111111",
    is_hidden: false,
    created_at: "2026-11-13T10:00:00Z",
    updated_at: "2026-11-13T10:00:00Z",
    job: {
      id: 1,
      slug: "go-stripe",
      title: "Senior Go Engineer",
      company: "Stripe",
      location: "Remote",
      status: "analyzed",
      is_remote: true,
      created_at: "2026-11-13T10:00:00Z",
    },
  },
  {
    id: "22222222-2222-2222-2222-222222222222",
    is_hidden: true,
    created_at: "2026-11-14T10:00:00Z",
    updated_at: "2026-11-14T10:00:00Z",
    job: {
      id: 2,
      slug: "cplusplus-lead",
      title: "Lead C++ Systems Architect",
      company: "Cloudflare",
      location: "Austin, TX",
      status: "analyze_pending",
      is_remote: false,
      created_at: "2026-11-14T10:00:00Z",
    },
  },
];

describe("useUserJobs hook and filterUserJobs", () => {
  it("filters user jobs by row.job.title, remote, and row.is_hidden", () => {
    const titleMatch = filterUserJobs(mockUserJobs, "stripe", "all");
    expect(titleMatch).toHaveLength(1);
    expect(titleMatch[0].job.title).toBe("Senior Go Engineer");

    const hiddenOnly = filterUserJobs(mockUserJobs, "", "all", false, "hidden");
    expect(hiddenOnly).toHaveLength(1);
    expect(hiddenOnly[0].id).toBe("22222222-2222-2222-2222-222222222222");
    expect(hiddenOnly[0].is_hidden).toBe(true);

    const visibleOnly = filterUserJobs(mockUserJobs, "", "all", false, "visible");
    expect(visibleOnly).toHaveLength(1);
    expect(visibleOnly[0].id).toBe("11111111-1111-1111-1111-111111111111");
    expect(visibleOnly[0].is_hidden).toBe(false);

    const all = filterUserJobs(mockUserJobs, "", "all", false, "all");
    expect(all).toHaveLength(2);
  });

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

    expect(result.current.jobs).toHaveLength(2);
    expect(result.current.filteredJobs).toHaveLength(2);
    expect(result.current.jobs[0].job.title).toBe("Senior Go Engineer");
    expect(result.current.pagination?.totalItems).toBe(2);
  });

  it("handles empty or non-array returns gracefully", async () => {
    vi.spyOn(endpoints, "getUserJobs").mockResolvedValue({
      data: [] as UserJob[],
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

  it("hideJob calls the endpoint and updates local state without refetching", async () => {
    const hideSpy = vi.spyOn(endpoints, "setUserJobHideStatus").mockResolvedValue({
      updated: 1,
    });
    const listSpy = vi.spyOn(endpoints, "getUserJobs").mockResolvedValue({
      data: [mockUserJobs[0]],
      pagination: {
        page: 1,
        limit: 10,
        totalItems: 1,
        totalPages: 1,
        hasNext: false,
        hasPrev: false,
      },
    });

    const { result } = renderHook(() => useUserJobs());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    result.current.hideJob("11111111-1111-1111-1111-111111111111", true);

    await waitFor(() => {
      expect(hideSpy).toHaveBeenCalledWith(
        "11111111-1111-1111-1111-111111111111",
        true,
        "test-token"
      );
    });

    // No refetch after hide: call count should not increase after hideJob.
    const callsAtHide = listSpy.mock.calls.length;

    await waitFor(() => {
      expect(result.current.jobs.length).toBeGreaterThanOrEqual(1);
    });

    expect(listSpy.mock.calls.length).toBe(callsAtHide);
  });

  it("retryJob re-runs compatibility when only the evaluation failed", async () => {
    const evalFailedJob: UserJob = {
      ...mockUserJobs[0],
      evaluation: {
        id: "eval-1",
        evaluation_status: "failed",
        report_status: "success",
      },
    };
    vi.spyOn(endpoints, "getUserJobs").mockResolvedValue({
      data: [evalFailedJob],
      pagination: {
        page: 1,
        limit: 10,
        totalItems: 1,
        totalPages: 1,
        hasNext: false,
        hasPrev: false,
      },
    });
    const analysisSpy = vi.spyOn(endpoints, "retryJobAnalysis");
    const compatSpy = vi
      .spyOn(endpoints, "triggerJobCompatibility")
      .mockResolvedValue({} as never);

    const { result } = renderHook(() => useUserJobs());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.retryJob(evalFailedJob.id);
    });

    expect(compatSpy).toHaveBeenCalledWith("go-stripe", true, "test-token");
    expect(analysisSpy).not.toHaveBeenCalled();
    await waitFor(() => {
      expect(result.current.jobs[0].evaluation?.evaluation_status).toBe("queued_retry");
    });
  });

  it("retryJob queues analysis by slug and moves the job to queued_retry locally", async () => {
    const failedJob: UserJob = {
      ...mockUserJobs[0],
      job: { ...mockUserJobs[0].job, status: "failed" },
    };
    vi.spyOn(endpoints, "getUserJobs").mockResolvedValue({
      data: [failedJob],
      pagination: {
        page: 1,
        limit: 10,
        totalItems: 1,
        totalPages: 1,
        hasNext: false,
        hasPrev: false,
      },
    });
    const retrySpy = vi
      .spyOn(endpoints, "retryJobAnalysis")
      .mockResolvedValue({ workflow_ids: ["job-analyze-1-force-1"] });

    const { result } = renderHook(() => useUserJobs());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.retryJob(failedJob.id);
    });

    expect(retrySpy).toHaveBeenCalledWith("go-stripe", "test-token");
    await waitFor(() => {
      expect(result.current.jobs[0].job.status).toBe("queued_retry");
    });
  });

  it("retryJob retries only the report when the report failed", async () => {
    const reportFailedJob: UserJob = {
      ...mockUserJobs[0],
      evaluation: { id: "eval-1", evaluation_status: "success", report_status: "failed" },
    };
    vi.spyOn(endpoints, "getUserJobs").mockResolvedValue({
      data: [reportFailedJob],
      pagination: {
        page: 1,
        limit: 10,
        totalItems: 1,
        totalPages: 1,
        hasNext: false,
        hasPrev: false,
      },
    });
    const reportSpy = vi
      .spyOn(endpoints, "retryReport")
      .mockResolvedValue({} as never);

    const { result } = renderHook(() => useUserJobs());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.retryJob(reportFailedJob.id);
    });

    expect(reportSpy).toHaveBeenCalledWith("eval-1", "test-token");
    await waitFor(() => {
      expect(result.current.jobs[0].evaluation?.report_status).toBe("queued_retry");
    });
  });

  it("retryJob leaves report status alone when report retry is forbidden", async () => {
    const reportFailedJob: UserJob = {
      ...mockUserJobs[0],
      evaluation: { id: "eval-1", evaluation_status: "success", report_status: "failed" },
    };
    vi.spyOn(endpoints, "getUserJobs").mockResolvedValue({
      data: [reportFailedJob],
      pagination: {
        page: 1,
        limit: 10,
        totalItems: 1,
        totalPages: 1,
        hasNext: false,
        hasPrev: false,
      },
    });
    vi.spyOn(endpoints, "retryReport").mockRejectedValue(
      Object.assign(new Error("access denied"), { status: 403 })
    );

    const { result } = renderHook(() => useUserJobs());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await expect(
      act(async () => {
        await result.current.retryJob(reportFailedJob.id);
      })
    ).rejects.toThrow("You do not have permission to retry this report.");

    expect(result.current.jobs[0].evaluation?.report_status).toBe("failed");
  });

  it("retryJob queues resume generation when the resume failed", async () => {
    const resumeFailedJob: UserJob = {
      ...mockUserJobs[0],
      evaluation: {
        id: "eval-1",
        evaluation_status: "success",
        report_status: "success",
        resume_status: "failed",
      },
    };
    vi.spyOn(endpoints, "getUserJobs").mockResolvedValue({
      data: [resumeFailedJob],
      pagination: {
        page: 1,
        limit: 10,
        totalItems: 1,
        totalPages: 1,
        hasNext: false,
        hasPrev: false,
      },
    });
    const resumeSpy = vi
      .spyOn(endpoints, "retryResumeGeneration")
      .mockResolvedValue({} as never);

    const { result } = renderHook(() => useUserJobs());

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    await act(async () => {
      await result.current.retryJob(resumeFailedJob.id);
    });

    expect(resumeSpy).toHaveBeenCalledWith("eval-1", "test-token");
    await waitFor(() => {
      expect(result.current.jobs[0].evaluation?.resume_status).toBe("queued_retry");
    });
  });
});
