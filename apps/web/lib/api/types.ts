export interface ReportGap {
  requirement: string;
  status?: "met" | "partially_met" | "not_found_in_cv" | string;
  suggestion?: string | null;
}

export interface EvaluationSummary {
  id: string;
  user_job_id: string;
  job?: Job;
  evaluation_status: "queued" | "queued_retry" | "pending" | "success" | "failed";
  overall_score?: number;
  eligibility_status?: string;
  recommendation_label?: string;
  report_status?: string;
  resume_status?: string;
  created_at?: string;
  updated_at?: string;
  // Report payload — returned alongside the evaluation so the detail view
  // can render the match report without a second fetch.
  evaluation_data?: {
    evaluations: CategoryEvaluation[];
    mandatory_requirements: MandatoryRequirementAssessment[];
  };
  report_data?: {
    summary?: string;
    strengths?: string[];
    gaps?: ReportGap[];
    cv_improvements?: string[];
  };
}

// The /v1/user-jobs/{user_job_id}/evaluation response nests the evaluation
// under its own "evaluation" key (mirroring "job"), so it is not merged with
// the user-jobs data at the root.
export interface EvaluationDetail {
  id: string;
  user_job_id: string;
  job?: Job;
  evaluation: MatchReport;
}

export interface CreateManualJobRequest {
  title: string;
  company: string;
  location: string;
  description?: string;
  source_url?: string;
  apply_url?: string;
  posted_date?: string;
  is_remote?: boolean;
  visa_relocation?: boolean;
}

export interface ScrapeURLRequest {
  source_url: string;
}

export interface CreateJobResponse {
  job: Job;
  created: boolean;
  added_to_user_list: boolean;
}

export interface Job {
  id: number;
  slug: string;
  status: string;
  title: string;
  company: string;
  location: string;
  description?: string;
  source?: string;
  failure_message?: string;
  apply_url?: string;
  source_url?: string;
  is_remote?: boolean;
  visa_relocation?: boolean;
  posted_date?: string;
  created_at: string;
  updated_at?: string;
  is_owned?: boolean;
  user_job?: { id: string };
}

export interface UserJobEvaluation {
  id: string;
  evaluation_status: string;
  eligibility_status?: string;
  recommendation_label?: string;
  report_status?: string;
  resume_status?: string;
  resume_failure_message?: string;
  cover_letter_status?: string;
  cover_letter_failure_message?: string;
}

export interface UserJob {
  id: string;
  is_hidden: boolean;
  created_at: string;
  updated_at: string;
  job: Job;
  evaluation?: UserJobEvaluation | null;
}

export interface CategoryEvidence {
  candidate_evidence: string;
  job_requirement: string;
  relationship: "direct" | "transferable";
}

export interface CategoryEvaluation {
  category: string;
  score: number;
  reason: string;
  strengths?: string[];
  gaps?: string[];
  evidence?: CategoryEvidence[];
}

export interface MandatoryRequirementAssessment {
  requirement: string;
  status: "met" | "not_met" | "uncertain";
  reason: string;
  evidence?: string[];
}

export interface MatchReport {
  id: string;
  user_job_id: string;
  job?: Job;
  evaluation_status: string;
  report_status?: string;
  overall_score?: number;
  eligibility_status?: string;
  recommendation_label?: string;
  resume_status?: string;
  resume_failure_message?: string;
  cover_letter_status?: string;
  cover_letter_text?: string;
  cover_letter_failure_message?: string;
  cover_letter_file_name?: string;
  report_data?: {
    summary?: string;
    strengths?: string[];
    gaps?: ReportGap[];
    cv_improvements?: string[];
  };
  evaluation_data?: {
    evaluations: CategoryEvaluation[];
    mandatory_requirements: MandatoryRequirementAssessment[];
  };
  created_at?: string;
  updated_at?: string;
}

export interface DownloadResumeResponse {
  download_url: string;
  expires_at: string;
  filename: string;
}

export interface ApiErrorDetail {
  field?: string;
  issue?: string;
}

export interface ApiErrorPayload {
  code: string;
  message: string;
  details?: ApiErrorDetail[];
}

export interface ApiPagination {
  page: number;
  limit: number;
  totalItems: number;
  totalPages: number;
  hasNext: boolean;
  hasPrev: boolean;
}

export interface ApiMeta {
  timestamp: string;
  requestId: string;
  pagination?: ApiPagination;
  build?: string;
}

export interface ApiResponse<T = any> {
  success: boolean;
  data: T;
  meta: ApiMeta;
  error?: ApiErrorPayload | null;
}

// Public account payload for GET/PATCH/POST /v1/me. Email, role, and timestamps
// are read-only; the profile fields below are the self-editable subset.
export interface AccountUser {
  id: string;
  email: string | null;
  full_name: string | null;
  headline: string | null;
  phone: string | null;
  portfolio_url: string | null;
  linkedin_url: string | null;
  github_url: string | null;
  role: "user" | "admin";
  onboarding_completed_at: string | null;
  created_at: string;
  updated_at: string;
}

// PATCH /v1/me requires all six keys; null clears the corresponding field.
export interface UpdateAccountInput {
  full_name: string | null;
  headline: string | null;
  phone: string | null;
  portfolio_url: string | null;
  linkedin_url: string | null;
  github_url: string | null;
}
