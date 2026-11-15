import { useState, useEffect } from "react";
import { Job, EvaluationSummary } from "@/lib/api/types";
import { getJobBySlug, getJobs, triggerJobCompatibility } from "@/lib/api/endpoints";

export function useJobDetail(slug: string, token?: string | null) {
  const [job, setJob] = useState<Job | null>(null);
  const [allJobs, setAllJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [evaluating, setEvaluating] = useState(false);

  useEffect(() => {
    let mounted = true;
    setLoading(true);

    Promise.all([getJobBySlug(slug, token), getJobs(token)])
      .then(([jobData, jobsList]) => {
        if (mounted) {
          setJob(jobData);
          setAllJobs(jobsList);
          setError(null);
        }
      })
      .catch((err) => {
        if (mounted) {
          setError(err);
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
  }, [slug, token]);

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
