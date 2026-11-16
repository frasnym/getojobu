"use client";

import React, { use, useState, useEffect } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { useJobDetail } from "@/hooks/use-job-detail";
import { JobReportHeader, ReportTab } from "@/components/jobs/job-report-header";
import { JobDetailTab } from "@/components/jobs/job-detail-tab";
import { MatchReportTab } from "@/components/matching/match-report-tab";
import { ResumeTab } from "@/components/resume/resume-tab";
import { JobDetailDrawer } from "@/components/jobs/job-detail-drawer";
import { getUserJobEvaluation } from "@/lib/api/endpoints";
import { EvaluationSummary } from "@/lib/api/types";
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
  const openDrawer = searchParams.get("openDrawer") === "1";
  const [activeTab, setActiveTab] = useState<ReportTab>("details");
  const { getToken } = useAuth();
  const [evalId, setEvalId] = useState<string | null>(null);
  const [fetchedEvaluation, setFetchedEvaluation] = useState<EvaluationSummary | null>(null);

  useEffect(() => {
    if (tabParam && ["details", "report", "resume"].includes(tabParam)) {
      setActiveTab(tabParam);
    }
  }, [tabParam]);

  const handleAnalyze = async (force: boolean) => {
    const summary = await runEvaluation(force);
    if (summary?.id) {
      setEvalId(String(summary.id));
    }
  };

  const handleTabChange = (tab: ReportTab) => {
    setActiveTab(tab);
    // Inside the drawer pop-up there is no page URL to sync; keep the tab in local state only.
    if (!openDrawer) {
      router.replace(`/user-jobs/${slug}?tab=${tab}`);
    }
  };

  // Fetch the latest evaluation for this user job so the resume tab can show
  // the generation status and the presigned download button when ready.
  useEffect(() => {
    if (!job || !job.user_job_id) return;
    const userJobId = job.user_job_id;
    let cancelled = false;
    const load = async () => {
      try {
        const token = await getToken();
        const evalRec = await getUserJobEvaluation(userJobId, token);
        if (!cancelled && evalRec.id) {
          setFetchedEvaluation(evalRec);
          if (!evalId) setEvalId(String(evalRec.id));
        }
      } catch (err) {
        // Evaluation may not exist yet — that's fine, the Analyze button
        // creates one. Silently ignore so the page still renders.
        console.debug("No existing evaluation for user job", job.user_job_id, err);
      }
    };

    load();
    return () => {
      cancelled = true;
    };
  }, [job, job?.user_job_id, getToken]);

  const handleDrawerClose = () => {
    router.push("/user-jobs");
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

  // Open the workspace drawer directly on this page (used by "Open Evaluation Workspace →").
  if (openDrawer) {
    return (
      <div className="min-h-screen bg-background">
        <JobDetailDrawer
          job={job}
          isOpen={true}
          onClose={handleDrawerClose}
          activeTab={activeTab}
          onTabChange={handleTabChange}
          evalId={evalId || undefined}
          evaluating={evaluating}
          onAnalyzeClick={() => handleAnalyze(false)}
        />
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
            report={fetchedEvaluation || undefined}
            evalId={evalId || undefined}
            onAnalyzeClick={() => handleAnalyze(false)}
            evaluating={evaluating}
          />
        )}

        {/* Tab 3: Tailored Resume */}
        {activeTab === "resume" && (
          <ResumeTab
            evalId={evalId || undefined}
            resumeStatus={fetchedEvaluation?.resume_status ?? undefined}
            cvImprovements={fetchedEvaluation?.report_data?.cv_improvements ?? []}
            token={null}
            onAnalyzeClick={() => handleAnalyze(false)}
            evaluating={evaluating}
          />
        )}
      </main>
    </div>
  );
}
