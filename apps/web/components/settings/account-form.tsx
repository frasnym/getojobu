"use client";

import React, { useEffect, useRef, useState } from "react";
import { AlertTriangle, Lock } from "lucide-react";
import { useMe } from "@/hooks/use-me";
import { validateAccountForm } from "@/lib/account-validation";
import { UpdateAccountInput } from "@/lib/api/types";

const FIELDS: Array<{
  key: keyof UpdateAccountInput;
  label: string;
  placeholder: string;
}> = [
  { key: "full_name", label: "Full name", placeholder: "Your full name" },
  { key: "headline", label: "Headline", placeholder: "e.g. Backend Engineer" },
  { key: "phone", label: "Phone", placeholder: "+62 812 3456 7890" },
  { key: "portfolio_url", label: "Portfolio URL", placeholder: "https://example.com" },
  { key: "linkedin_url", label: "LinkedIn URL", placeholder: "https://linkedin.com/in/you" },
  { key: "github_url", label: "GitHub URL", placeholder: "https://github.com/you" },
];

const inputClasses =
  "w-full rounded-md border bg-background px-3 py-2 text-sm text-foreground shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2";

function messageOf(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback;
}

function formatDate(value: string | null): string {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleDateString();
}

export function AccountForm() {
  const { user, form, setField, loading, saving, error, status, save, deactivate } = useMe();
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [edited, setEdited] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [deactivateError, setDeactivateError] = useState<string | null>(null);
  const [deactivating, setDeactivating] = useState(false);
  const cancelRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (confirmOpen) {
      cancelRef.current?.focus();
    }
  }, [confirmOpen]);

  const handleFieldChange = (field: keyof UpdateAccountInput, value: string) => {
    setField(field, value === "" ? null : value);
    setEdited(true);
  };

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const errors = validateAccountForm(form);
    setFieldErrors(errors);
    if (Object.keys(errors).length > 0) return;
    const saved = await save();
    if (saved) setEdited(false);
  };

  const handleConfirmDeactivate = async () => {
    setDeactivateError(null);
    setDeactivating(true);
    try {
      await deactivate();
      setConfirmOpen(false);
    } catch (err) {
      setDeactivateError(messageOf(err, "Could not deactivate the account. Please try again."));
    } finally {
      setDeactivating(false);
    }
  };

  return (
    <div className="space-y-8">
      <header className="border-b border-border/40 pb-5">
        <h1 className="text-2xl font-black tracking-tight text-foreground">Account</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Manage your personal details. Your email and role are managed externally.
        </p>
      </header>

      <form noValidate onSubmit={handleSubmit} className="space-y-8">
        <section className="rounded-xl border border-border bg-card p-6 shadow-sm">
          <h2 className="text-lg font-semibold text-foreground">Account details</h2>
          <div className="mt-5 grid gap-4 sm:grid-cols-2">
            {FIELDS.map(({ key, label, placeholder }) => {
              const fieldError = fieldErrors[key];
              const inputId = `account-${key.replace(/_/g, "-")}`;
              return (
                <div key={key} className="space-y-1.5">
                  <label htmlFor={inputId} className="block text-sm font-medium text-foreground">
                    {label}
                  </label>
                  <input
                    id={inputId}
                    name={key}
                    type="text"
                    value={form[key] ?? ""}
                    placeholder={placeholder}
                    onChange={(event) => handleFieldChange(key, event.target.value)}
                    aria-invalid={fieldError ? true : undefined}
                    aria-describedby={fieldError ? `${inputId}-error` : undefined}
                    className={`${inputClasses} ${fieldError ? "border-red-500" : "border-border"}`}
                  />
                  {fieldError && (
                    <p id={`${inputId}-error`} className="text-xs text-red-500">
                      {fieldError}
                    </p>
                  )}
                </div>
              );
            })}
          </div>

          <div className="mt-6 space-y-1.5 border-t border-border/40 pt-6">
            <label htmlFor="account-email" className="block text-sm font-medium text-foreground">
              Email
            </label>
            <input
              id="account-email"
              type="text"
              readOnly
              value={user?.email ?? ""}
              aria-describedby="account-email-hint"
              className={`${inputClasses} cursor-default border-border bg-muted text-muted-foreground`}
            />
            <p id="account-email-hint" className="flex items-center gap-1.5 text-xs text-muted-foreground">
              <Lock className="h-3 w-3" />
              <span>Managed by Clerk</span>
            </p>
          </div>

          <p className="mt-5 text-xs text-muted-foreground">
            Role: <span className="font-medium text-foreground">{user?.role ?? "-"}</span>
            {" | "}Onboarding: <span className="font-medium text-foreground">{user?.onboarding_completed_at ? formatDate(user.onboarding_completed_at) : "not completed"}</span>
            {" | "}<span>Member since:</span>{" "}
            <span className="font-medium text-foreground">{formatDate(user?.created_at ?? null)}</span>
          </p>

          {error && (
            <div
              role="alert"
              className="mt-5 rounded-lg border border-red-500/30 bg-red-500/5 px-4 py-3 text-sm text-red-500 dark:text-red-400"
            >
              {messageOf(error, "Could not save your account details.")}
            </div>
          )}

          {status === "saved" && !edited && (
            <div
              role="status"
              className="mt-5 rounded-lg border border-emerald-500/30 bg-emerald-500/5 px-4 py-3 text-sm text-emerald-600 dark:text-emerald-400"
            >
              Account details saved.
            </div>
          )}

          <div className="mt-6 flex items-center gap-3">
            <button
              type="submit"
              disabled={saving || loading}
              className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:opacity-50"
            >
              {saving ? "Saving..." : "Save changes"}
            </button>
            {loading && <span className="text-xs text-muted-foreground">Loading account...</span>}
          </div>
        </section>

        <section className="rounded-xl border border-border bg-card p-6 shadow-sm">
          <h2 className="text-lg font-semibold text-foreground">Account state</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Deactivating your account signs you out and disables access.
          </p>
          <button
            type="button"
            onClick={() => {
              setDeactivateError(null);
              setConfirmOpen(true);
            }}
            className="mt-5 rounded-lg border border-red-500/40 px-4 py-2 text-sm font-semibold text-red-500 transition-colors hover:bg-red-500/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 dark:text-red-400"
          >
            Deactivate account
          </button>
        </section>
      </form>

      {confirmOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="deactivate-title"
            className="w-full max-w-md rounded-xl border border-border bg-card p-6 shadow-lg"
          >
            <div className="flex items-start gap-3">
              <AlertTriangle className="h-5 w-5 shrink-0 text-red-500" />
              <div>
                <h3 id="deactivate-title" className="text-lg font-semibold text-foreground">
                  Deactivate account?
                </h3>
                <p className="mt-1 text-sm text-muted-foreground">
                  This signs you out and deactivates your account. It cannot be undone from this page.
                </p>
              </div>
            </div>

            {deactivateError && (
              <p
                role="alert"
                className="mt-4 rounded-lg border border-red-500/30 bg-red-500/5 px-3 py-2 text-sm text-red-500"
              >
                {deactivateError}
              </p>
            )}

            <div className="mt-6 flex justify-end gap-3">
              <button
                ref={cancelRef}
                type="button"
                onClick={() => setConfirmOpen(false)}
                className="rounded-lg border border-border px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleConfirmDeactivate}
                disabled={deactivating}
                className="rounded-lg bg-red-500 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-red-500/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:opacity-50"
              >
                {deactivating ? "Deactivating..." : "Confirm"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
