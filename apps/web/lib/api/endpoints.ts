import { apiClient, apiClientWithPagination, PaginatedResult } from "./client";
import {
  Job,
  UserJob,
  DownloadResumeResponse,
  EvaluationSummary,
  EvaluationDetail,
  AccountUser,
  UpdateAccountInput,
  CreateManualJobRequest,
  ScrapeURLRequest,
  CreateJobResponse,
  ApplicationChatMessage,
} from "./types";

export async function createManualJob(
  params: CreateManualJobRequest,
  token?: string | null
): Promise<CreateJobResponse> {
  return apiClient<CreateJobResponse>("/v1/jobs/manual", {
    method: "POST",
    token,
    body: JSON.stringify(params),
  });
}

export async function scrapeJobFromURL(
  params: ScrapeURLRequest,
  token?: string | null
): Promise<CreateJobResponse> {
  return apiClient<CreateJobResponse>("/v1/jobs/scrape-url", {
    method: "POST",
    token,
    body: JSON.stringify(params),
  });
}

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
    resume_status?: string;
    cover_letter_status?: string;
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
  if (filters?.resume_status && filters.resume_status !== "all") {
    params.set("resume_status", filters.resume_status);
  }
  if (filters?.cover_letter_status && filters.cover_letter_status !== "all") {
    params.set("cover_letter_status", filters.cover_letter_status);
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
): Promise<{ workflow_ids: string[]; failed?: { slug: string; error: string }[] }> {
  return apiClient<{ workflow_ids: string[]; failed?: { slug: string; error: string }[] }>(
    "/v1/jobs/bulk-queue-analyze",
    {
      method: "POST",
      token,
      body: JSON.stringify([{ slug, force: true }]),
    }
  );
}

export async function triggerJobCompatibility(
  slug: string,
  force: boolean = false,
  token?: string | null
): Promise<EvaluationSummary> {
  const created = await apiClient<Array<EvaluationSummary & { error?: string | null }>>(
    "/v1/jobs/evaluation",
    {
      method: "POST",
      token,
      body: JSON.stringify([{ slug }]),
    }
  );
  const ids = (Array.isArray(created) ? created : [])
    .filter((item) => item?.id && !item.error)
    .map((item) => ({ id: item.id }));
  if (ids.length === 0) {
    throw new Error("No evaluation was created for this job");
  }

  const result = await apiClient<Array<EvaluationSummary & { error?: string | null }>>(
    "/v1/matching-evaluations/bulk-generate-evaluation",
    {
      method: "POST",
      token,
      body: JSON.stringify(ids),
    }
  );
  // A workflow can reach `pending` before the backend's conditional status
  // flip lands. The summary then reports an error while the job is running
  // normally, so an in-flight status is still a valid result here.
  const inFlightStatuses = new Set(["pending", "queued", "queued_retry"]);
  const triggered = Array.isArray(result)
    ? result.find(
        (item) =>
          !!item &&
          (!item.error || inFlightStatuses.has(item.evaluation_status))
      )
    : undefined;
  if (!triggered) {
    throw new Error("Empty response from bulk-generate-evaluation");
  }
  return triggered;
}

/** Queue report generation for evaluation IDs via the bulk endpoint. Clerk callers must own each evaluation; an ownership failure is a per-item error. */
export async function bulkGenerateReport(
  items: string[],
  token?: string | null
): Promise<EvaluationSummary> {
  const result = await apiClient<Array<EvaluationSummary & { error?: string | null }>>(
    "/v1/matching-evaluations/bulk-generate-report",
    {
      method: "POST",
      token,
      body: JSON.stringify(items.map((id) => ({ id, force: true }))),
    }
  );
  const item = Array.isArray(result) ? result[0] : undefined;
  if (!item) {
    throw new Error("Empty response from bulk-generate-report");
  }
  if (item.error) {
    throw new Error(item.error);
  }
  return item;
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

/**
 * Ask for a one-sentence answer to an application form question.
 * Uses the same-origin /v1 rewrite; the token is passed explicitly by the caller.
 */
export async function answerApplicationQuestion(
  slug: string,
  question: string,
  history: ApplicationChatMessage[],
  token: string | null,
  onDelta: (text: string) => void
): Promise<void> {
  const response = await fetch(`/v1/jobs/${encodeURIComponent(slug)}/chat`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "text/event-stream",
      Authorization: `Bearer ${token ?? ""}`,
    },
    body: JSON.stringify({ question, history }),
  });

  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as {
      error?: { message?: string };
      message?: string;
    };
    throw new Error(
      body.error?.message || body.message || `API request failed with status ${response.status}`
    );
  }

  const reader = response.body?.getReader();
  if (!reader) {
    throw new Error("This browser does not support streaming responses");
  }

  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const frames = buffer.split("\n\n");
    buffer = frames.pop() ?? "";
    for (const frame of frames) {
      const line = frame.split("\n").find((part) => part.startsWith("data:"));
      if (!line) continue;
      const data = line.slice(5).trim();
      if (!data) continue;
      const payload = JSON.parse(data) as {
        delta?: string;
        done?: boolean;
        error?: string;
      };
      if (payload.error) throw new Error(payload.error);
      if (payload.delta) onDelta(payload.delta);
      if (payload.done) return;
    }
  }
}
