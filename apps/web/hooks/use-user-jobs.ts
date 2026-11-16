import { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { useAuth } from "@clerk/nextjs";
import { UserJob, ApiPagination } from "@/lib/api/types";
import {
  getUserJobs,
  setUserJobHideStatus,
} from "@/lib/api/endpoints";
import { StatusFilter } from "./use-jobs";

export type HideFilter = "all" | "visible" | "hidden";

export function filterUserJobs(
  rows: UserJob[],
  query: string,
  status: StatusFilter,
  remoteOnly: boolean = false,
  hideFilter: HideFilter = "all"
): UserJob[] {
  if (!Array.isArray(rows)) {
    return [];
  }
  const normalizedQuery = query.trim().toLowerCase();

  return rows.filter((row) => {
    const job = row.job;
    if (!job) return false;

    // Remote filter
    if (remoteOnly && job.is_remote !== true) {
      return false;
    }

    // Hide filter
    if (hideFilter === "hidden" && row.is_hidden !== true) {
      return false;
    }
    if (hideFilter === "visible" && row.is_hidden === true) {
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

export function useUserJobs(explicitToken?: string | null) {
  const { getToken, isLoaded } = useAuth();
  const [userJobs, setUserJobs] = useState<UserJob[]>([]);
  const [pagination, setPagination] = useState<ApiPagination | null>(null);
  const [page, setPageState] = useState<number>(1);
  const [limit, setLimitState] = useState<number>(10);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [remoteOnly, setRemoteOnly] = useState(false);
  const [hideFilter, setHideFilter] = useState<HideFilter>("all");

  // Snapshot of the initial filter state (the defaults) captured on first
  // render. Used to detect when the user has changed any filter away from its
  // default, which gates the "Query all" / "Search all" button.
  const initialFiltersRef = useRef({
    searchQuery,
    statusFilter,
    remoteOnly,
    hideFilter,
  });

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
        const res = await getUserJobs(token, page, limit);
        if (!mountedRef || mountedRef.current) {
          setUserJobs(Array.isArray(res.data) ? res.data : []);
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
      setUserJobs(Array.isArray(res.data) ? res.data : []);
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

  const filteredUserJobs = useMemo(
    () => filterUserJobs(userJobs, searchQuery, statusFilter, remoteOnly, hideFilter),
    [userJobs, searchQuery, statusFilter, remoteOnly, hideFilter]
  );

  const isFiltersChanged = useMemo(
    () =>
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
      setUserJobs((prev) =>
        prev.map((uj) =>
          uj.id === userJobId ? { ...uj, is_hidden: isHidden } : uj
        )
      );
    },
    [explicitToken, getToken]
  );

  return {
    jobs: userJobs,
    userJobs,
    filteredJobs: filteredUserJobs,
    filteredUserJobs,
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
