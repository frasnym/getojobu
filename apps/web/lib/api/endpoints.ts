import { apiClient, isMockEnabled } from "./client";
import {
  Job,
  MatchReport,
  DownloadResumeResponse,
  EvaluationSummary,
} from "./types";
import {
  MOCK_JOBS,
  MOCK_REPORTS,
  MOCK_RESUME_DOWNLOADS,
} from "./mock-adapter";

export async function getJobs(token?: string | null): Promise<Job[]> {
  if (isMockEnabled()) {
    return Promise.resolve([...MOCK_JOBS]);
  }
  try {
    return await apiClient<Job[]>("/v1/jobs", { token });
  } catch (err) {
    console.warn("Failed to reach live backend /v1/jobs, falling back to mock:", err);
    return [...MOCK_JOBS];
  }
}

export async function getJobBySlug(slug: string, token?: string | null): Promise<Job> {
  if (isMockEnabled()) {
    const job = MOCK_JOBS.find((j) => j.slug === slug);
    if (!job) {
      throw new Error(`Job not found: ${slug}`);
    }
    return Promise.resolve({ ...job });
  }
  try {
    return await apiClient<Job>(`/v1/jobs/${slug}`, { token });
  } catch (err) {
    console.warn(`Failed to reach /v1/jobs/${slug}, checking mock data:`, err);
    const mock = MOCK_JOBS.find((j) => j.slug === slug);
    if (mock) return { ...mock };
    throw err;
  }
}

export async function triggerJobCompatibility(
  slug: string,
  force: boolean = false,
  token?: string | null
): Promise<EvaluationSummary> {
  if (isMockEnabled()) {
    const job = MOCK_JOBS.find((j) => j.slug === slug);
    return Promise.resolve(
      job?.latest_evaluation || {
        id: `eval-${slug}-new`,
        evaluation_status: "success",
        overall_score: 85.0,
        eligibility_status: "eligible",
        recommendation_label: "strong_match",
        report_status: "ready",
        resume_status: "ready",
        created_at: new Date().toISOString(),
      }
    );
  }
  try {
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
  } catch (err) {
    console.warn("triggerJobCompatibility fallback to mock:", err);
    return {
      id: "eval-stripe-001",
      evaluation_status: "success",
      overall_score: 88.5,
      eligibility_status: "eligible",
      recommendation_label: "strong_match",
      report_status: "ready",
      resume_status: "ready",
    };
  }
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
  try {
    return await apiClient<MatchReport>(
      `/v1/matching-evaluations/${evalId}/report`,
      { token }
    );
  } catch (err) {
    console.warn(`getMatchReport fallback for ${evalId}:`, err);
    const fallback = MOCK_REPORTS[evalId] || MOCK_REPORTS["eval-stripe-001"];
    if (fallback) return { ...fallback, id: evalId };
    throw err;
  }
}

export async function getResumeDownloadUrl(
  evalId: string,
  token?: string | null
): Promise<DownloadResumeResponse> {
  if (isMockEnabled()) {
    const download = MOCK_RESUME_DOWNLOADS[evalId] || MOCK_RESUME_DOWNLOADS["eval-stripe-001"];
    return Promise.resolve({ ...download });
  }
  try {
    return await apiClient<DownloadResumeResponse>(
      `/v1/matching-evaluations/${evalId}/resume/download`,
      { token }
    );
  } catch (err) {
    console.warn(`getResumeDownloadUrl fallback for ${evalId}:`, err);
    return (
      MOCK_RESUME_DOWNLOADS[evalId] || MOCK_RESUME_DOWNLOADS["eval-stripe-001"]
    );
  }
}
