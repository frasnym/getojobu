import { useState, useEffect, useMemo, useCallback } from "react";
import { useAuth } from "@clerk/nextjs";
import { Job, ApiPagination } from "@/lib/api/types";
import { getUserJobs, setUserJobHideStatus } from "@/lib/api/endpoints";
import { StatusFilter, filterJobs } from "./use-jobs";

export type HideFilter = "all" | "visible" | "hidden";

export function useUserJobs(explicitToken?: string | null) {
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
  const [hideFilter, setHideFilter] = useState<HideFilter>("all");

  const setPage = useCallback((newPage: number) => {
    setPageState(Math.max(1, newPage));
  }, []);

  const setLimit = useCallback((newLimit: number) => {
    setLimitState(newLimit);
    setPageState(1);
  }, []);

  const load = useCallback(
    async (mountedRef?: { current: boolean }) => {
      try {
        setLoading(true);
        const token = explicitToken !== undefined ? explicitToken : await getToken();
        const res = await getUserJobs(token, page, limit);
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
    () => filterJobs(jobs, searchQuery, statusFilter, remoteOnly, "all", hideFilter),
    [jobs, searchQuery, statusFilter, remoteOnly, hideFilter]
  );

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

  const handleHideFilterChange = (filter: HideFilter) => {
    setHideFilter(filter);
    setPageState(1);
  };

  const hideJob = useCallback(
    async (userJobId: string, isHidden: boolean) => {
      const token = explicitToken !== undefined ? explicitToken : await getToken();
      await setUserJobHideStatus(userJobId, isHidden, token);
      // Refetch the list so the UI reflects the new state.
      await load();
    },
    [explicitToken, getToken, load]
  );

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
    hideFilter,
    setHideFilter: handleHideFilterChange,
    hideJob,
    refetch: () => load(),
  };
}
