import { useState, useEffect } from "react";
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
  }, [slug, explicitToken, isLoaded, getToken]);

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
      setJob((prev) => (prev ? { ...prev, latest_evaluation: summary } : prev));
      return summary;
    } catch (err: any) {
      console.error("Evaluation trigger failed:", err);
      throw err;
    } finally {
      setEvaluating(false);
    }
  };

  return {
    job,
    loading,
    error,
    prevSlug,
    nextSlug,
    evaluating,
    runEvaluation,
  };
}
