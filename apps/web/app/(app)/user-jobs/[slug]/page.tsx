"use client";

import React, { use, useState, useEffect } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { useJobDetail } from "@/hooks/use-job-detail";
import { JobReportHeader, ReportTab } from "@/components/jobs/job-report-header";
import { JobDetailTab } from "@/components/jobs/job-detail-tab";
import { MatchReportTab } from "@/components/matching/match-report-tab";
import { ResumeTab } from "@/components/resume/resume-tab";
import { getMatchReport } from "@/lib/api/endpoints";
import { MatchReport } from "@/lib/api/types";
import { AlertCircle } from "lucide-react";

interface UserJobDetailPageProps {
  params: Promise<{ slug: string }>;
}

export default function UserJobDetailPage({ params }: UserJobDetailPageProps) {
  const { slug } = use(params);
  const router = useRouter();
  const searchParams = useSearchParams();

  const { job, loading, error, prevSlug, nextSlug, evaluating, runEvaluation } =
    useJobDetail(slug);

  const tabParam = searchParams.get("tab") as ReportTab | null;
  const [activeTab, setActiveTab] = useState<ReportTab>("report");
  const { getToken } = useAuth();
  const [report, setReport] = useState<MatchReport | null>(null);

  useEffect(() => {
    if (job?.latest_evaluation?.id) {
      getToken().then((token) => {
        getMatchReport(job.latest_evaluation!.id, token)
          .then(setReport)
          .catch(() => {});
      });
    }
  }, [job?.latest_evaluation?.id, getToken]);

  useEffect(() => {
    if (tabParam && ["details", "report", "resume"].includes(tabParam)) {
      setActiveTab(tabParam);
    } else if (job) {
      // Default to "report" if evaluated, otherwise "details"
      setActiveTab(job.latest_evaluation ? "report" : "details");
    }
  }, [tabParam, job]);

  const handleTabChange = (tab: ReportTab) => {
    setActiveTab(tab);
    router.replace(`/user-jobs/${slug}?tab=${tab}`);
  };

  if (loading) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8 space-y-6 animate-pulse">
        <div className="h-20 rounded-xl bg-card/60" />
        <div className="h-64 rounded-xl bg-card/60" />
      </div>
    );
  }

  if (error || !job) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-destructive/10 text-destructive mb-4">
          <AlertCircle className="h-6 w-6" />
        </div>
        <h2 className="text-xl font-bold text-foreground">Job Not Found</h2>
        <p className="mt-2 text-sm text-muted-foreground">
          {error?.message || "The requested job opportunity could not be found."}
        </p>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-background">
      <JobReportHeader
        job={job}
        activeTab={activeTab}
        onTabChange={handleTabChange}
        prevSlug={prevSlug}
        nextSlug={nextSlug}
        onAnalyzeClick={() => runEvaluation(true)}
        evaluating={evaluating}
        basePath="/user-jobs"
      />

      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {/* Tab 1: Details */}
        {activeTab === "details" && <JobDetailTab job={job} />}

        {/* Tab 2: Match Report */}
        {activeTab === "report" && (
          <MatchReportTab
            evalId={job.latest_evaluation?.id}
            onAnalyzeClick={() => runEvaluation(false)}
            evaluating={evaluating}
          />
        )}

        {/* Tab 3: Tailored Resume */}
        {activeTab === "resume" && (
          <ResumeTab
            evalId={job.latest_evaluation?.id}
            resumeStatus={job.latest_evaluation?.resume_status}
            cvImprovements={report?.report_data?.cv_improvements}
            onAnalyzeClick={() => runEvaluation(false)}
            evaluating={evaluating}
          />
        )}
      </main>
    </div>
  );
}
