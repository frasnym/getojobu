import { apiClient, apiClientWithPagination, isMockEnabled, PaginatedResult } from "./client";
import {
  Job,
  MatchReport,
  DownloadResumeResponse,
  EvaluationSummary,
} from "./types";
import {
  MOCK_JOBS,
  MOCK_EVALUATIONS,
  MOCK_REPORTS,
  MOCK_RESUME_DOWNLOADS,
  getMockUserJobs,
  mockSaveJob,
  paginateList,
} from "./mock-adapter";

export async function getJobs(
  token?: string | null,
  page: number = 1,
  limit: number = 10
): Promise<PaginatedResult<Job[]>> {
  if (isMockEnabled()) {
    return Promise.resolve(paginateList([...MOCK_JOBS], page, limit));
  }
  return apiClientWithPagination<Job[]>(`/v1/jobs?page=${page}&limit=${limit}`, { token });
}

export async function getUserJobs(
  token?: string | null,
  page: number = 1,
  limit: number = 10
): Promise<PaginatedResult<Job[]>> {
  if (isMockEnabled()) {
    return Promise.resolve(paginateList(getMockUserJobs(), page, limit));
  }
  return apiClientWithPagination<Job[]>(`/v1/user-jobs?page=${page}&limit=${limit}`, { token });
}

export async function saveUserJob(
  job: { id: number; slug: string },
  token?: string | null
): Promise<any> {
  if (isMockEnabled()) {
    return Promise.resolve(mockSaveJob(job.slug));
  }
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
  if (isMockEnabled()) {
    return Promise.resolve({ user_job_id: userJobId, is_hidden: isHidden });
  }
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
  if (isMockEnabled()) {
    const evalRec = MOCK_EVALUATIONS[userJobId];
    if (!evalRec) {
      throw new Error(`Evaluation not found for user job: ${userJobId}`);
    }
    return Promise.resolve({ ...evalRec });
  }
  return apiClient<EvaluationSummary>(
    `/v1/user-jobs/${userJobId}/evaluation`,
    { token }
  );
}

export async function getJobBySlug(slug: string, token?: string | null): Promise<Job> {
  if (isMockEnabled()) {
    const job = MOCK_JOBS.find((j) => j.slug === slug);
    if (!job) {
      throw new Error(`Job not found: ${slug}`);
    }
    return Promise.resolve({ ...job });
  }
  return apiClient<Job>(`/v1/jobs/${slug}`, { token });
}

export async function triggerJobCompatibility(
  slug: string,
  force: boolean = false,
  token?: string | null
): Promise<EvaluationSummary> {
  if (isMockEnabled()) {
    const job = MOCK_JOBS.find((j) => j.slug === slug);
    return Promise.resolve({
      id: `eval-${slug}-new`,
      evaluation_status: "success",
      overall_score: 85.0,
      eligibility_status: "eligible",
      recommendation_label: "strong_match",
      report_status: "ready",
      resume_status: "ready",
      created_at: new Date().toISOString(),
    });
  }
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

export async function getMatchReport(
  evalId: string,
  token?: string | null
): Promise<MatchReport> {
  if (isMockEnabled()) {
    const report = MOCK_REPORTS[evalId] || MOCK_REPORTS["eval-stripe-001"];
    if (!report) {
      throw new Error(`Match report not found: ${evalId}`);
    }
    return Promise.resolve({ ...report, id: evalId });
  }
  return apiClient<MatchReport>(
    `/v1/matching-evaluations/${evalId}/report`,
    { token }
  );
}

export async function getResumeDownloadUrl(
  evalId: string,
  token?: string | null
): Promise<DownloadResumeResponse> {
  if (isMockEnabled()) {
    const download = MOCK_RESUME_DOWNLOADS[evalId] || MOCK_RESUME_DOWNLOADS["eval-stripe-001"];
    return Promise.resolve({ ...download });
  }
  return apiClient<DownloadResumeResponse>(
    `/v1/matching-evaluations/${evalId}/resume/download`,
    { token }
  );
}
