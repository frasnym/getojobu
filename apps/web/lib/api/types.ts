export interface EvaluationSummary {
  id: string;
  evaluation_status: "new" | "pending" | "success" | "failed";
  overall_score?: number;
  eligibility_status?: string;
  recommendation_label?: string;
  report_status?: string;
  resume_status?: string;
  created_at?: string;
  updated_at?: string;
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
  latest_evaluation?: EvaluationSummary | null;
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
    gaps?: string[];
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
