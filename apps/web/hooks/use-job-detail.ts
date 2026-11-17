import { useState, useEffect, useCallback } from "react";
import { useAuth } from "@clerk/nextjs";
import { Job, EvaluationSummary } from "@/lib/api/types";
import { getJobBySlug, getJobs, triggerJobCompatibility } from "@/lib/api/endpoints";

export function useJobDetail(slug: string, explicitToken?: string | null) {
  const { getToken, isLoaded } = useAuth();
  const [job, setJob] = useState<Job | null>(null);
  const [allJobs, setAllJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [evaluating, setEvaluating] = useState(false);
  // Bumping this tick re-runs the load effect. Used by the manual Refresh button.
  const [refreshTick, setRefreshTick] = useState(0);
  const refresh = useCallback(() => setRefreshTick((tick) => tick + 1), []);

  useEffect(() => {
    let mounted = true;

    if (explicitToken === undefined && !isLoaded) {
      return;
    }

    setLoading(true);

    const loadData = async () => {
      try {
        const token = explicitToken !== undefined ? explicitToken : await getToken();
        const [jobData, jobsList] = await Promise.all([
          getJobBySlug(slug, token),
          getJobs(token),
        ]);
        if (mounted) {
          setJob(jobData);
          setAllJobs(Array.isArray(jobsList) ? jobsList : []);
          setError(null);
        }
      } catch (err: any) {
        if (mounted) {
          setError(err);
        }
      } finally {
        if (mounted) {
          setLoading(false);
        }
      }
    };

    loadData();

    return () => {
      mounted = false;
    };
  }, [slug, explicitToken, isLoaded, getToken, refreshTick]);

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
  };
}
