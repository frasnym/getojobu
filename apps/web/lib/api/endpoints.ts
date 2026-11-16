import { apiClient, apiClientWithPagination, PaginatedResult } from "./client";
import {
  Job,
  UserJob,
  DownloadResumeResponse,
  EvaluationSummary,
} from "./types";

export async function getJobs(
  token?: string | null,
  page: number = 1,
  limit: number = 10
): Promise<PaginatedResult<Job[]>> {
  return apiClientWithPagination<Job[]>(`/v1/jobs?page=${page}&limit=${limit}`, { token });
}

export async function getUserJobs(
  token?: string | null,
  page: number = 1,
  limit: number = 10,
  filters?: { status?: string; remote?: boolean; hide?: string }
): Promise<PaginatedResult<UserJob[]>> {
  const params = new URLSearchParams();
  params.set("page", String(page));
  params.set("limit", String(limit));
  if (filters?.status && filters.status !== "all") {
    params.set("status", filters.status);
  }
  if (filters?.remote) {
    params.set("remote", "true");
  }
  if (filters?.hide && filters.hide !== "all") {
    params.set("hide", filters.hide);
  }
  return apiClientWithPagination<UserJob[]>(`/v1/user-jobs?${params.toString()}`, {
    token,
  });
}

export async function saveUserJob(
  job: { id: number; slug: string },
  token?: string | null
): Promise<any> {
  return apiClient<{ job_id: number; created: boolean }>("/v1/user-jobs", {
    method: "POST",
    token,
    body: JSON.stringify({ job_id: job.id }),
  });
}

export async function setUserJobsHideStatus(
  userJobIds: string[],
  isHidden: boolean,
  token?: string | null
): Promise<{ updated: number }> {
  return apiClient<{ updated: number }>(`/v1/user-jobs/bulk`, {
    method: "PATCH",
    token,
    body: JSON.stringify({ user_job_ids: userJobIds, is_hidden: isHidden }),
  });
}

/** Convenience wrapper for a single user job. */
export async function setUserJobHideStatus(
  userJobId: string,
  isHidden: boolean,
  token?: string | null
): Promise<{ updated: number }> {
  return setUserJobsHideStatus([userJobId], isHidden, token);
}

export async function getUserJobEvaluation(
  userJobId: string,
  token?: string | null
): Promise<EvaluationSummary> {
  return apiClient<EvaluationSummary>(
    `/v1/user-jobs/${userJobId}/evaluation`,
    { token }
  );
}

export async function getJobBySlug(slug: string, token?: string | null): Promise<Job> {
  return apiClient<Job>(`/v1/jobs/${slug}`, { token });
}

export async function triggerJobCompatibility(
  slug: string,
  force: boolean = false,
  token?: string | null
): Promise<EvaluationSummary> {
  const result = await apiClient<EvaluationSummary[]>(
    "/v1/jobs/bulk-analyze-compatibility",
    {
      method: "POST",
      token,
      body: JSON.stringify([{ slug, force }]),
    }
  );
  if (Array.isArray(result) && result[0]) {
    return result[0];
  }
  throw new Error("Empty response from bulk-analyze-compatibility");
}

export async function getResumeDownloadUrl(
  evalId: string,
  token?: string | null
): Promise<DownloadResumeResponse> {
  return apiClient<DownloadResumeResponse>(
    `/v1/matching-evaluations/${evalId}/resume/download`,
    { token }
  );
}
