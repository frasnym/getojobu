export interface ReportGap {
  requirement: string;
  status?: "met" | "partially_met" | "not_found_in_cv" | string;
  suggestion?: string | null;
}

export interface EvaluationSummary {
  id: string;
  user_job_id: string;
  evaluation_status: "new" | "pending" | "success" | "failed";
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

export interface Job {
  id: number;
  slug: string;
  status: string;
  title: string;
  company: string;
  location: string;
  description?: string;
  apply_url?: string;
  source_url?: string;
  is_remote?: boolean;
  posted_date?: string;
  created_at: string;
  updated_at?: string;
  is_owned?: boolean;
  user_job_id?: string;
  is_hidden?: boolean;
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
  evaluation_status: string;
  report_status?: string;
  overall_score?: number;
  eligibility_status?: string;
  recommendation_label?: string;
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
