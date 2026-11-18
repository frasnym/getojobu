import { apiClient, apiClientWithPagination, PaginatedResult } from "./client";
import {
  Job,
  UserJob,
  DownloadResumeResponse,
  EvaluationSummary,
  EvaluationDetail,
  AccountUser,
  UpdateAccountInput,
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
  filters?: {
    status?: string;
    remote?: boolean;
    hide?: string;
    evaluation_status?: string;
    report_status?: string;
    eligibility_status?: string;
    recommendation_label?: string;
  }
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
  if (filters?.evaluation_status && filters.evaluation_status !== "all") {
    params.set("evaluation_status", filters.evaluation_status);
  }
  if (filters?.report_status && filters.report_status !== "all") {
    params.set("report_status", filters.report_status);
  }
  if (filters?.eligibility_status && filters.eligibility_status !== "all") {
    params.set("eligibility_status", filters.eligibility_status);
  }
  if (filters?.recommendation_label && filters.recommendation_label !== "all") {
    params.set("recommendation_label", filters.recommendation_label);
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
): Promise<EvaluationDetail> {
  return apiClient<EvaluationDetail>(
    `/v1/user-jobs/${userJobId}/evaluation`,
    { token }
  );
}

export async function getJobBySlug(slug: string, token?: string | null): Promise<Job> {
  return apiClient<Job>(`/v1/jobs/${slug}`, { token });
}

/** Re-queue analysis for a failed job. The backend sets its status to queued_retry. */
export async function retryJobAnalysis(
  slug: string,
  token?: string | null
): Promise<{ workflow_ids: string[] }> {
  return apiClient<{ workflow_ids: string[] }>("/v1/jobs/bulk-queue-analyze", {
    method: "POST",
    token,
    body: JSON.stringify([{ slug, force: true }]),
  });
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

/** Queue cover-letter generation for an evaluation via the bulk endpoint. */
export async function generateCoverLetter(
  evalId: string,
  variant: "short" | "full",
  force: boolean,
  token?: string | null
): Promise<EvaluationSummary> {
  const result = await apiClient<Array<EvaluationSummary & { error?: string | null }>>(
    "/v1/matching-evaluations/bulk-generate-cover-letter",
    {
      method: "POST",
      token,
      body: JSON.stringify([{ id: evalId, variant, force }]),
    }
  );
  const item = Array.isArray(result) ? result[0] : undefined;
  if (!item) {
    throw new Error("Empty response from bulk-generate-cover-letter");
  }
  if (item.error) {
    throw new Error(item.error);
  }
  return item;
}

/** Queue resume generation for an evaluation (retry path). Clerk users must own the evaluation. */
export async function retryResumeGeneration(
  evalId: string,
  token?: string | null,
  force: boolean = true
): Promise<EvaluationSummary> {
  const result = await apiClient<Array<EvaluationSummary & { error?: string | null }>>(
    "/v1/matching-evaluations/bulk-generate-resume",
    {
      method: "POST",
      token,
      body: JSON.stringify([{ id: evalId, force }]),
    }
  );
  const item = Array.isArray(result) ? result[0] : undefined;
  if (!item) {
    throw new Error("Empty response from bulk-generate-resume");
  }
  if (item.error) {
    throw new Error(item.error);
  }
  return item;
}

export async function getMe(token?: string | null): Promise<AccountUser> {
  return apiClient<AccountUser>("/v1/me", { token });
}

export async function updateMe(
  input: UpdateAccountInput,
  token?: string | null
): Promise<AccountUser> {
  return apiClient<AccountUser>("/v1/me", {
    method: "PATCH",
    token,
    body: JSON.stringify(input),
  });
}

export async function deactivateMe(token?: string | null): Promise<AccountUser> {
  return apiClient<AccountUser>("/v1/me/deactivate", {
    method: "POST",
    token,
  });
}
