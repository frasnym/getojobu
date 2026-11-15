import { useState, useEffect, useMemo } from "react";
import { Job } from "@/lib/api/types";
import { getJobs } from "@/lib/api/endpoints";

export type StatusFilter = "all" | "analyzed" | "pending" | "failed";

export function filterJobs(jobs: Job[], query: string, status: StatusFilter): Job[] {
  const normalizedQuery = query.trim().toLowerCase();

  return jobs.filter((job) => {
    // Status filter
    if (status !== "all" && job.status.toLowerCase() !== status.toLowerCase()) {
      return false;
    }

    // Query filter
    if (!normalizedQuery) {
      return true;
    }

    const titleMatch = job.title?.toLowerCase().includes(normalizedQuery);
    const companyMatch = job.company?.toLowerCase().includes(normalizedQuery);
    const locationMatch = job.location?.toLowerCase().includes(normalizedQuery);

    return titleMatch || companyMatch || locationMatch;
  });
}

export function useJobs(token?: string | null) {
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");

  useEffect(() => {
    let mounted = true;
    setLoading(true);

    getJobs(token)
      .then((data) => {
        if (mounted) {
          setJobs(data);
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
  }, [token]);

  const filteredJobs = useMemo(
    () => filterJobs(jobs, searchQuery, statusFilter),
    [jobs, searchQuery, statusFilter]
  );

  return {
    jobs,
    filteredJobs,
    loading,
    error,
    searchQuery,
    setSearchQuery,
    statusFilter,
    setStatusFilter,
  };
}
