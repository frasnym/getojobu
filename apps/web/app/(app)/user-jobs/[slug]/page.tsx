"use client";

import React, { use, useState, useEffect, useCallback } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { useJobDetail } from "@/hooks/use-job-detail";
import { JobReportHeader, ReportTab } from "@/components/jobs/job-report-header";
import { JobDetailTab } from "@/components/jobs/job-detail-tab";
import { MatchReportTab } from "@/components/matching/match-report-tab";
import { ResumeTab } from "@/components/resume/resume-tab";
import { JobDetailDrawer } from "@/components/jobs/job-detail-drawer";
import { getUserJobEvaluation } from "@/lib/api/endpoints";
import { setUserJobsHideStatus } from "@/lib/api/endpoints";
import { EvaluationDetail } from "@/lib/api/types";
import { AlertCircle } from "lucide-react";

interface UserJobDetailPageProps {
  params: Promise<{ slug: string }>;
}

export default function UserJobDetailPage({ params }: UserJobDetailPageProps) {
  const { slug } = use(params);
  const router = useRouter();
  const searchParams = useSearchParams();

  const { job, userJobId, loading, error, prevSlug, nextSlug } = useJobDetail(slug);

  const tabParam = searchParams.get("tab") as ReportTab | null;
  const openDrawer = searchParams.get("openDrawer") === "1";
  const [activeTab, setActiveTab] = useState<ReportTab>("details");
  const { getToken } = useAuth();
  const [evalId, setEvalId] = useState<string | null>(null);
  const [fetchedEvaluation, setFetchedEvaluation] = useState<EvaluationDetail | null>(null);
  const resolvedUserJobId = userJobId ?? job?.user_job?.id;
  const [isHidden, setIsHidden] = useState(false);
  const [evalRefreshTick, setEvalRefreshTick] = useState(0);
  const refreshEvaluation = useCallback(() => setEvalRefreshTick((t) => t + 1), []);

  const handleHideToggle = useCallback(
    async (userJobIdParam: string, nextHidden: boolean) => {
      const token = await getToken();
      await setUserJobsHideStatus([userJobIdParam], nextHidden, token);
      setIsHidden(nextHidden);
    },
    [getToken]
  );

  useEffect(() => {
    if (tabParam && ["details", "report", "resume"].includes(tabParam)) {
      setActiveTab(tabParam);
    }
  }, [tabParam]);

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
    if (!job || !resolvedUserJobId) return;
    let cancelled = false;
    const load = async () => {
      try {
        const token = await getToken();
        const evalRec = await getUserJobEvaluation(resolvedUserJobId, token);
        if (!cancelled && evalRec.id) {
          setFetchedEvaluation(evalRec);
          if (!evalId) setEvalId(String(evalRec.id));
        }
      } catch (err) {
        // Evaluation may not exist yet — that's fine, the Analyze button
        // creates one. Silently ignore so the page still renders.
        console.debug("No existing evaluation for user job", resolvedUserJobId, err);
      }
    };

    load();
    return () => {
      cancelled = true;
    };
  }, [job, resolvedUserJobId, getToken, evalRefreshTick]);

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
          userJobId={resolvedUserJobId}
          isOpen={true}
          onClose={handleDrawerClose}
          activeTab={activeTab}
          onTabChange={handleTabChange}
          evalId={evalId || undefined}
          showOwnedActions={false}
          isHidden={isHidden}
          onHideToggle={handleHideToggle}
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
        basePath="/user-jobs"
        isHidden={isHidden}
        userJobId={resolvedUserJobId}
        onHideToggle={handleHideToggle}
      />

      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {/* Tab 1: Details */}
        {activeTab === "details" && <JobDetailTab job={job} />}

        {/* Tab 2: Match Report */}
        {activeTab === "report" && (
          <MatchReportTab
            report={fetchedEvaluation?.evaluation || undefined}
            evalId={evalId || undefined}
          />
        )}

        {/* Tab 3: Tailored Resume */}
        {activeTab === "resume" && (
          <ResumeTab
            evalId={evalId || undefined}
            resumeStatus={fetchedEvaluation?.evaluation.resume_status ?? undefined}
            cvImprovements={fetchedEvaluation?.evaluation.report_data?.cv_improvements ?? []}
            token={null}
            onRetried={refreshEvaluation}
          />
        )}
      </main>
    </div>
  );
}
