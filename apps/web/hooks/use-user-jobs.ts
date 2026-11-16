import { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { useAuth } from "@clerk/nextjs";
import { Job, ApiPagination } from "@/lib/api/types";
import {
  getUserJobs,
  setUserJobHideStatus,
} from "@/lib/api/endpoints";
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

  // Capture the initial filter state once (on mount) so we can detect when the
  // user has changed any filter away from its default. Used to gate the
  // "Query all" / "Search all" button.
  const initialFiltersRef = useRef<{
    searchQuery: string;
    statusFilter: StatusFilter;
    remoteOnly: boolean;
    hideFilter: HideFilter;
  } | null>(null);

  useEffect(() => {
    if (initialFiltersRef.current !== null) return;
    initialFiltersRef.current = {
      searchQuery,
      statusFilter,
      remoteOnly,
      hideFilter,
    };
  }, [searchQuery, statusFilter, remoteOnly, hideFilter]);

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

  // Query the backend with the current filters (server-side), replacing the
  // locally-filtered list. Used by the "Query all" / "Search all" button.
  const queryAll = useCallback(async () => {
    const token = explicitToken !== undefined ? explicitToken : await getToken();
    setLoading(true);
    setError(null);
    try {
      const res = await getUserJobs(token, 1, limit, {
        status: statusFilter,
        remote: remoteOnly,
        hide: hideFilter,
      });
      setJobs(Array.isArray(res.data) ? res.data : []);
      setPagination(res.pagination);
      setPageState(1);
    } catch (err: any) {
      setError(err);
    } finally {
      setLoading(false);
    }
  }, [explicitToken, getToken, limit, statusFilter, remoteOnly, hideFilter]);

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

  const isFiltersChanged = useMemo(
    () =>
      initialFiltersRef.current === null ||
      searchQuery !== initialFiltersRef.current.searchQuery ||
      statusFilter !== initialFiltersRef.current.statusFilter ||
      remoteOnly !== initialFiltersRef.current.remoteOnly ||
      hideFilter !== initialFiltersRef.current.hideFilter,
    [searchQuery, statusFilter, remoteOnly, hideFilter]
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
      // Update local state directly; no refetch needed.
      setJobs((prev) =>
        prev.map((j) =>
          j.user_job_id === userJobId ? { ...j, is_hidden: isHidden } : j
        )
      );
    },
    [explicitToken, getToken]
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
    isFiltersChanged,
    queryAll,
  };
}
