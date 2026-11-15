import { useState, useEffect, useMemo } from "react";
import { useAuth } from "@clerk/nextjs";
import { Job } from "@/lib/api/types";
import { getUserJobs } from "@/lib/api/endpoints";
import { StatusFilter, filterJobs } from "./use-jobs";

export function useUserJobs(explicitToken?: string | null) {
  const { getToken, isLoaded } = useAuth();
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [remoteOnly, setRemoteOnly] = useState(false);

  const load = async (mountedRef?: { current: boolean }) => {
    try {
      setLoading(true);
      const token = explicitToken !== undefined ? explicitToken : await getToken();
      const data = await getUserJobs(token);
      if (!mountedRef || mountedRef.current) {
        setJobs(Array.isArray(data) ? data : []);
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
  };

  useEffect(() => {
    const mountedRef = { current: true };

    if (explicitToken === undefined && !isLoaded) {
      return;
    }

    load(mountedRef);

    return () => {
      mountedRef.current = false;
    };
  }, [explicitToken, isLoaded, getToken]);

  const filteredJobs = useMemo(
    () => filterJobs(jobs, searchQuery, statusFilter, remoteOnly),
    [jobs, searchQuery, statusFilter, remoteOnly]
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
    remoteOnly,
    setRemoteOnly,
    refetch: () => load(),
  };
}
