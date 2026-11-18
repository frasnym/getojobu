import { useState, useEffect, useCallback, useRef } from "react";
import { useAuth } from "@clerk/nextjs";
import { AccountUser, UpdateAccountInput } from "@/lib/api/types";
import { getMe, updateMe, deactivateMe } from "@/lib/api/endpoints";

export type AccountSaveStatus = "idle" | "saved";

const EMPTY_FORM: UpdateAccountInput = {
  full_name: null,
  headline: null,
  phone: null,
  portfolio_url: null,
  linkedin_url: null,
  github_url: null,
};

function toForm(user: AccountUser): UpdateAccountInput {
  return {
    full_name: user.full_name ?? null,
    headline: user.headline ?? null,
    phone: user.phone ?? null,
    portfolio_url: user.portfolio_url ?? null,
    linkedin_url: user.linkedin_url ?? null,
    github_url: user.github_url ?? null,
  };
}

export function useMe(explicitToken?: string | null) {
  const { getToken, isLoaded, signOut } = useAuth();
  const [user, setUser] = useState<AccountUser | null>(null);
  const [form, setForm] = useState<UpdateAccountInput>(EMPTY_FORM);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [status, setStatus] = useState<AccountSaveStatus>("idle");
  const mountedRef = useRef(true);

  const resolveToken = useCallback(async () => {
    return explicitToken !== undefined ? explicitToken : await getToken();
  }, [explicitToken, getToken]);

  const load = useCallback(async () => {
    try {
      setLoading(true);
      const token = await resolveToken();
      const res = await getMe(token);
      if (mountedRef.current) {
        setUser(res);
        setForm(toForm(res));
        setError(null);
      }
    } catch (err) {
      if (mountedRef.current) {
        setError(err as Error);
      }
    } finally {
      if (mountedRef.current) {
        setLoading(false);
      }
    }
  }, [resolveToken]);

  useEffect(() => {
    mountedRef.current = true;
    if (explicitToken === undefined && !isLoaded) {
      return;
    }
    load();
    return () => {
      mountedRef.current = false;
    };
  }, [explicitToken, isLoaded, load]);

  const setField = useCallback((field: keyof UpdateAccountInput, value: string | null) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  }, []);

  const save = useCallback(async (): Promise<boolean> => {
    setSaving(true);
    setStatus("idle");
    try {
      const token = await resolveToken();
      const res = await updateMe(form, token);
      if (mountedRef.current) {
        setUser(res);
        setForm(toForm(res));
        setError(null);
        setStatus("saved");
      }
      return true;
    } catch (err) {
      if (mountedRef.current) {
        setError(err as Error);
      }
      return false;
    } finally {
      if (mountedRef.current) {
        setSaving(false);
      }
    }
  }, [resolveToken, form]);

  const deactivate = useCallback(async () => {
    const token = await resolveToken();
    await deactivateMe(token);
    await signOut();
  }, [resolveToken, signOut]);

  return {
    user,
    form,
    setField,
    loading,
    saving,
    error,
    status,
    save,
    deactivate,
    refetch: load,
  };
}
