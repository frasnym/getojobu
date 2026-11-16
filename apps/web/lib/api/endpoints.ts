import { apiClient, apiClientWithPagination, PaginatedResult } from "./client";
import {
  Job,
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
  limit: number = 10
): Promise<PaginatedResult<Job[]>> {
  return apiClientWithPagination<Job[]>(`/v1/user-jobs?page=${page}&limit=${limit}`, { token });
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

export async function setUserJobHideStatus(
  userJobId: string,
  isHidden: boolean,
  token?: string | null
): Promise<{ user_job_id: string; is_hidden: boolean }> {
  return apiClient<{ user_job_id: string; is_hidden: boolean }>(
    `/v1/user-jobs/${userJobId}/hide`,
    {
      method: "PATCH",
      token,
      body: JSON.stringify({ is_hidden: isHidden }),
    }
  );
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
