import { useState, useEffect, useMemo } from "react";
import { useAuth } from "@clerk/nextjs";
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

export function useJobs(explicitToken?: string | null) {
  const { getToken, isLoaded } = useAuth();
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");

  useEffect(() => {
    let mounted = true;

    // Wait until Clerk auth state is loaded if no explicit token was passed
    if (explicitToken === undefined && !isLoaded) {
      return;
    }

    setLoading(true);

    const load = async () => {
      try {
        const token = explicitToken !== undefined ? explicitToken : await getToken();
        const data = await getJobs(token);
        if (mounted) {
          setJobs(data);
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

    load();

    return () => {
      mounted = false;
    };
  }, [explicitToken, isLoaded, getToken]);

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
