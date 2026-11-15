import { useState, useEffect, useMemo, useCallback } from "react";
import { useAuth } from "@clerk/nextjs";
import { Job, ApiPagination } from "@/lib/api/types";
import { getJobs } from "@/lib/api/endpoints";

export type StatusFilter = "all" | "analyzed" | "analyze_pending" | "failed";

// Tailwind classes for the job status badge, keyed by backend job status.
export function statusBadgeClasses(status: string): string {
  const base =
    "rounded-full px-2 py-0.5 text-[11px] font-medium capitalize border";
  switch (status.toLowerCase()) {
    case "analyzed":
      // green
      return `${base} border-emerald-500/20 bg-emerald-500/15 text-emerald-600 dark:text-emerald-400`;
    case "failed":
      // red
      return `${base} border-destructive/20 bg-destructive/15 text-destructive`;
    case "analyze_pending":
    case "pending":
      // yellow
      return `${base} border-amber-500/20 bg-amber-500/15 text-amber-600 dark:text-amber-400`;
    default:
      return `${base} border-border/60 bg-secondary text-secondary-foreground`;
  }
}

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
  const [pagination, setPagination] = useState<ApiPagination | null>(null);
  const [page, setPageState] = useState<number>(1);
  const [limit, setLimitState] = useState<number>(10);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [remoteOnly, setRemoteOnly] = useState(false);

  const setPage = useCallback((newPage: number) => {
    setPageState(Math.max(1, newPage));
  }, []);

  const setLimit = useCallback((newLimit: number) => {
    setLimitState(newLimit);
    setPageState(1); // Reset to page 1 on limit change
  }, []);

  const load = useCallback(
    async (mountedRef?: { current: boolean }) => {
      try {
        setLoading(true);
        const token = explicitToken !== undefined ? explicitToken : await getToken();
        const res = await getJobs(token, page, limit);
        if (!mountedRef || mountedRef.current) {
          setJobs(Array.isArray(res.data) ? res.data : []);
          setPagination(res.pagination);
          setError(null);
        }
      } catch (err: any) {
        if (!mountedRef || mountedRef.current) {
          setError(err);
        }
      } finally {
        if (!mountedRef || mountedRef.current) {
          setLoading(false);
        }
      }
    },
    [explicitToken, getToken, page, limit]
  );

  useEffect(() => {
    const mountedRef = { current: true };

    if (explicitToken === undefined && !isLoaded) {
      return;
    }

    load(mountedRef);

    return () => {
      mountedRef.current = false;
    };
  }, [explicitToken, isLoaded, load]);

  const filteredJobs = useMemo(
    () => filterJobs(jobs, searchQuery, statusFilter, remoteOnly),
    [jobs, searchQuery, statusFilter, remoteOnly]
  );

  const setJobOwned = (slug: string, isOwned: boolean = true) => {
    setJobs((prev) =>
      prev.map((j) => (j.slug === slug ? { ...j, is_owned: isOwned } : j))
    );
  };

  const handleSearchChange = (query: string) => {
    setSearchQuery(query);
    setPageState(1);
  };

  const handleStatusChange = (status: StatusFilter) => {
    setStatusFilter(status);
    setPageState(1);
  };

  const handleRemoteChange = (remote: boolean) => {
    setRemoteOnly(remote);
    setPageState(1);
  };

  return {
    jobs,
    filteredJobs,
    pagination,
    page,
    setPage,
    limit,
    setLimit,
    loading,
    error,
    searchQuery,
    setSearchQuery: handleSearchChange,
    statusFilter,
    setStatusFilter: handleStatusChange,
    remoteOnly,
    setRemoteOnly: handleRemoteChange,
    setJobOwned,
    refetch: () => load(),
  };
}
