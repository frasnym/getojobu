import { useState, useEffect, useCallback } from "react";
import { useAuth } from "@clerk/nextjs";
import { Job, EvaluationSummary } from "@/lib/api/types";
import { getJobBySlug, getJobs, triggerJobCompatibility } from "@/lib/api/endpoints";

export function useJobDetail(slug: string, explicitToken?: string | null) {
  const { getToken, isLoaded } = useAuth();
  const [job, setJob] = useState<Job | null>(null);
  const [allJobs, setAllJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  // `refreshing` is separate from `loading`: a manual refresh keeps the page
  // mounted and only spins the Refresh button instead of showing the skeleton.
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [refreshError, setRefreshError] = useState<Error | null>(null);
  const [evaluating, setEvaluating] = useState(false);

  const fetchJobDetail = useCallback(async () => {
    const token = explicitToken !== undefined ? explicitToken : await getToken();
    const [jobData, jobsList] = await Promise.all([
      getJobBySlug(slug, token),
      getJobs(token),
    ]);
    return { jobData, jobsList: Array.isArray(jobsList) ? jobsList : [] };
  }, [slug, explicitToken, getToken]);

  const applyJobDetail = useCallback((jobData: Job, jobsList: Job[]) => {
    setJob(jobData);
    setAllJobs(jobsList);
  }, []);

  // Initial load. Manual refresh does not re-run this effect; it calls
  // fetchJobDetail directly through `refresh` below (no polling is added).
  useEffect(() => {
    let mounted = true;

    if (explicitToken === undefined && !isLoaded) {
      return;
    }

    setLoading(true);

    fetchJobDetail()
      .then(({ jobData, jobsList }) => {
        if (mounted) {
          applyJobDetail(jobData, jobsList);
          setError(null);
        }
      })
      .catch((err) => {
        if (mounted) {
          setError(err instanceof Error ? err : new Error(String(err)));
        }
      })
      .finally(() => {
        if (mounted) {
          setLoading(false);
        }
      });

    return () => {
      mounted = false;
    };
  }, [fetchJobDetail, applyJobDetail, explicitToken, isLoaded]);

  // Manual, awaitable refetch. Failures are captured in `refreshError` so the
  // caller can show a visible message instead of silently doing nothing.
  const refresh = useCallback(async (): Promise<void> => {
    setRefreshing(true);
    setRefreshError(null);
    try {
      const { jobData, jobsList } = await fetchJobDetail();
      applyJobDetail(jobData, jobsList);
    } catch (err) {
      setRefreshError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      setRefreshing(false);
    }
  }, [fetchJobDetail, applyJobDetail]);

  // Compute prev/next slugs for sequential navigation
  const currentIndex = allJobs.findIndex((j) => j.slug === slug);
  const prevSlug = currentIndex > 0 ? allJobs[currentIndex - 1]?.slug : undefined;
  const nextSlug =
    currentIndex >= 0 && currentIndex < allJobs.length - 1
      ? allJobs[currentIndex + 1]?.slug
      : undefined;

  const runEvaluation = async (force: boolean = false): Promise<EvaluationSummary | null> => {
    if (!job) return null;
    try {
      setEvaluating(true);
      const token = explicitToken !== undefined ? explicitToken : await getToken();
      const summary = await triggerJobCompatibility(job.slug, force, token);
      return summary;
    } catch (err: any) {
      console.error("Evaluation trigger failed:", err);
      throw err;
    } finally {
      setEvaluating(false);
    }
  };

  const userJobId = job?.user_job?.id;

  return {
    job,
    userJobId,
    loading,
    error,
    prevSlug,
    nextSlug,
    evaluating,
    runEvaluation,
    refresh,
    refreshing,
    refreshError,
  };
}
