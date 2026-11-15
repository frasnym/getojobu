import { useState, useEffect, useMemo } from "react";
import { useAuth } from "@clerk/nextjs";
import { Job } from "@/lib/api/types";
import { getJobs } from "@/lib/api/endpoints";

export type StatusFilter = "all" | "analyzed" | "pending" | "failed";

export function filterJobs(
  jobs: Job[],
  query: string,
  status: StatusFilter,
  remoteOnly: boolean = false
): Job[] {
  if (!Array.isArray(jobs)) {
    return [];
  }
  const normalizedQuery = query.trim().toLowerCase();

  return jobs.filter((job) => {
    // Remote filter
    if (remoteOnly && job.is_remote !== true) {
      return false;
    }

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
  const [remoteOnly, setRemoteOnly] = useState(false);

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
          setJobs(Array.isArray(data) ? data : []);
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
    () => filterJobs(jobs, searchQuery, statusFilter, remoteOnly),
    [jobs, searchQuery, statusFilter, remoteOnly]
  );

  const setJobOwned = (slug: string, isOwned: boolean = true) => {
    setJobs((prev) =>
      prev.map((j) => (j.slug === slug ? { ...j, is_owned: isOwned } : j))
    );
  };

  const refetch = async () => {
    try {
      setLoading(true);
      const token = explicitToken !== undefined ? explicitToken : await getToken();
      const data = await getJobs(token);
      setJobs(Array.isArray(data) ? data : []);
      setError(null);
    } catch (err: any) {
      setError(err);
    } finally {
      setLoading(false);
    }
  };

  return {
    jobs,
    filteredJobs,
    loading,
    error,
    searchQuery,
    setSearchQuery,
    statusFilter,
    setStatusFilter,
    remoteOnly,
    setRemoteOnly,
    setJobOwned,
    refetch,
  };
}
