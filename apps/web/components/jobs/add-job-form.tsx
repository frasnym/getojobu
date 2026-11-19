"use client";

import React, { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { createManualJob } from "@/lib/api/endpoints";

const inputClasses =
  "w-full rounded-md border bg-background px-3 py-2 text-sm text-foreground shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";
const labelClasses = "block text-sm font-medium text-foreground";

export function AddJobForm() {
  const router = useRouter();
  const { getToken } = useAuth();
  const [form, setForm] = useState({
    title: "",
    company: "",
    location: "",
    description: "",
    source_url: "",
    apply_url: "",
    posted_date: "",
    is_remote: false,
    visa_relocation: false,
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const setField = (key: keyof typeof form, value: string | boolean) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const nextErrors: Record<string, string> = {};
    if (!form.title.trim()) nextErrors.title = "Title is required";
    if (!form.company.trim()) nextErrors.company = "Company is required";
    if (!form.location.trim()) nextErrors.location = "Location is required";
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length > 0) return;

    setSaving(true);
    setError(null);
    try {
      const token = await getToken();
      const result = await createManualJob(
        {
          title: form.title.trim(),
          company: form.company.trim(),
          location: form.location.trim(),
          description: form.description.trim() || undefined,
          source_url: form.source_url.trim() || undefined,
          apply_url: form.apply_url.trim() || undefined,
          posted_date: form.posted_date || undefined,
          is_remote: form.is_remote,
          visa_relocation: form.visa_relocation,
        },
        token
      );
      router.push(`/user-jobs/${result.job.slug}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not add the job");
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="grid gap-4 md:grid-cols-2">
        <div>
          <label className={labelClasses} htmlFor="title">Title</label>
          <input
            id="title"
            className={inputClasses}
            value={form.title}
            onChange={(e) => setField("title", e.target.value)}
            placeholder="Senior Backend Engineer"
          />
          {errors.title && <p className="mt-1 text-xs text-destructive">{errors.title}</p>}
        </div>
        <div>
          <label className={labelClasses} htmlFor="company">Company</label>
          <input
            id="company"
            className={inputClasses}
            value={form.company}
            onChange={(e) => setField("company", e.target.value)}
            placeholder="Acme"
          />
          {errors.company && <p className="mt-1 text-xs text-destructive">{errors.company}</p>}
        </div>
        <div>
          <label className={labelClasses} htmlFor="location">Location</label>
          <input
            id="location"
            className={inputClasses}
            value={form.location}
            onChange={(e) => setField("location", e.target.value)}
            placeholder="Berlin"
          />
          {errors.location && <p className="mt-1 text-xs text-destructive">{errors.location}</p>}
        </div>
        <div>
          <label className={labelClasses} htmlFor="posted_date">Posted date</label>
          <input
            id="posted_date"
            type="date"
            className={inputClasses}
            value={form.posted_date}
            onChange={(e) => setField("posted_date", e.target.value)}
          />
        </div>
      </div>

      <div>
        <label className={labelClasses} htmlFor="description">Description</label>
        <textarea
          id="description"
          className={inputClasses}
          rows={6}
          value={form.description}
          onChange={(e) => setField("description", e.target.value)}
          placeholder="Paste the job description"
        />
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div>
          <label className={labelClasses} htmlFor="apply_url">Apply URL</label>
          <input
            id="apply_url"
            type="url"
            className={inputClasses}
            value={form.apply_url}
            onChange={(e) => setField("apply_url", e.target.value)}
            placeholder="https://example.com/apply"
          />
        </div>
        <div>
          <label className={labelClasses} htmlFor="source_url">Source URL</label>
          <input
            id="source_url"
            type="url"
            className={inputClasses}
            value={form.source_url}
            onChange={(e) => setField("source_url", e.target.value)}
            placeholder="https://example.com/jobs/1"
          />
        </div>
      </div>

      <div className="flex gap-6">
        <label className="flex items-center gap-2 text-sm text-foreground">
          <input
            type="checkbox"
            checked={form.is_remote}
            onChange={(e) => setField("is_remote", e.target.checked)}
          />
          Remote
        </label>
        <label className="flex items-center gap-2 text-sm text-foreground">
          <input
            type="checkbox"
            checked={form.visa_relocation}
            onChange={(e) => setField("visa_relocation", e.target.checked)}
          />
          Visa relocation
        </label>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <button
        type="submit"
        disabled={saving}
        className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
      >
        {saving ? "Adding..." : "Add job"}
      </button>
    </form>
  );
}
