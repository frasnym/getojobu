"use client";

import React, { use, useState, useEffect, useCallback } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { useJobDetail } from "@/hooks/use-job-detail";
import { JobReportHeader, ReportTab } from "@/components/jobs/job-report-header";
import { JobDetailTab } from "@/components/jobs/job-detail-tab";
import { MatchReportTab } from "@/components/matching/match-report-tab";
import { ResumeTab } from "@/components/resume/resume-tab";
import { CoverLetterTab } from "@/components/cover-letter/cover-letter-tab";
import { JobDetailDrawer } from "@/components/jobs/job-detail-drawer";
import { ApplicationChatTab } from "@/components/jobs/application-chat-tab";
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

  const {
    job,
    userJobId,
    loading,
    error,
    prevSlug,
    nextSlug,
    refresh,
    refreshing: refreshingJob,
    refreshError: jobRefreshError,
  } = useJobDetail(slug);

  const tabParam = searchParams.get("tab") as ReportTab | null;
  const openDrawer = searchParams.get("openDrawer") === "1";
  const [activeTab, setActiveTab] = useState<ReportTab>("details");
  const { getToken } = useAuth();
  const [evalId, setEvalId] = useState<string | null>(null);
  const [fetchedEvaluation, setFetchedEvaluation] = useState<EvaluationDetail | null>(null);
  const resolvedUserJobId = userJobId ?? job?.user_job?.id;
  const [isHidden, setIsHidden] = useState(false);
  const [evalRefreshing, setEvalRefreshing] = useState(false);
  const [evalRefreshError, setEvalRefreshError] = useState<Error | null>(null);

  const fetchEvaluation = useCallback(async () => {
    if (!resolvedUserJobId) return null;
    const token = await getToken();
    return getUserJobEvaluation(resolvedUserJobId, token);
  }, [resolvedUserJobId, getToken]);

  const applyEvaluation = useCallback((evalRec: EvaluationDetail | null) => {
    if (!evalRec?.id) return;
    setFetchedEvaluation(evalRec);
    setEvalId((current) => current ?? String(evalRec.id));
  }, []);

  // Manual, awaitable refetch of the latest evaluation. Errors are captured in
  // state so the header can show a visible message.
  const refreshEvaluation = useCallback(async () => {
    if (!resolvedUserJobId) return;
    setEvalRefreshing(true);
    setEvalRefreshError(null);
    try {
      applyEvaluation(await fetchEvaluation());
    } catch (err) {
      setEvalRefreshError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      setEvalRefreshing(false);
    }
  }, [resolvedUserJobId, fetchEvaluation, applyEvaluation]);

  // Manual refresh refetches both the job detail and the latest evaluation.
  // router.refresh() is not used here because this page fetches client-side.
  const handleRefresh = useCallback(() => {
    void refresh();
    void refreshEvaluation();
  }, [refresh, refreshEvaluation]);

  const handleHideToggle = useCallback(
    async (userJobIdParam: string, nextHidden: boolean) => {
      const token = await getToken();
      await setUserJobsHideStatus([userJobIdParam], nextHidden, token);
      setIsHidden(nextHidden);
    },
    [getToken]
  );

  useEffect(() => {
    if (tabParam && ["details", "report", "resume", "cover-letter", "chat"].includes(tabParam)) {
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

  // Initial load of the latest evaluation for this user job so the resume and
  // cover-letter tabs can show generation status. A missing evaluation is
  // expected (the Analyze button creates one), so this path stays silent.
  useEffect(() => {
    if (!job || !resolvedUserJobId) return;
    let cancelled = false;
    (async () => {
      try {
        const evalRec = await fetchEvaluation();
        if (!cancelled) applyEvaluation(evalRec);
      } catch (err) {
        if (!cancelled) {
          console.debug("No existing evaluation for user job", resolvedUserJobId, err);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [job, resolvedUserJobId, fetchEvaluation, applyEvaluation]);

  const handleDrawerClose = () => {
    router.push("/user-jobs");
  };

  const refreshing = refreshingJob || evalRefreshing;
  const refreshErrorMessage = jobRefreshError?.message ?? evalRefreshError?.message;

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
        evaluation={fetchedEvaluation?.evaluation}
        onRefresh={handleRefresh}
        refreshing={refreshing}
      />

      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {refreshErrorMessage && (
          <div
            role="alert"
            className="mb-6 flex items-center gap-2 rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm font-medium text-destructive"
          >
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>Could not refresh: {refreshErrorMessage}</span>
          </div>
        )}

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
            reportStatus={fetchedEvaluation?.evaluation.report_status ?? undefined}
            cvImprovements={fetchedEvaluation?.evaluation.report_data?.cv_improvements ?? []}
            token={null}
            onRetried={refreshEvaluation}
          />
        )}

        {/* Tab 4: Cover Letter */}
        {activeTab === "cover-letter" && (
          <CoverLetterTab
            evalId={evalId || undefined}
            status={fetchedEvaluation?.evaluation.cover_letter_status ?? undefined}
            text={fetchedEvaluation?.evaluation.cover_letter_text ?? undefined}
            failureMessage={fetchedEvaluation?.evaluation.cover_letter_failure_message ?? undefined}
            fileName={fetchedEvaluation?.evaluation.cover_letter_file_name ?? undefined}
            reportStatus={fetchedEvaluation?.evaluation.report_status ?? undefined}
            onQueued={refreshEvaluation}
          />
        )}

        {/* Tab 5: Application Helper */}
        {activeTab === "chat" && <ApplicationChatTab job={job} />}
      </main>
    </div>
  );
}
