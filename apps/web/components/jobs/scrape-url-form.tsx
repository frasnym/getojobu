"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { scrapeJobFromURL } from "@/lib/api/endpoints";

export function ScrapeUrlForm() {
  const { getToken } = useAuth();
  const [sourceUrl, setSourceUrl] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [saving, setSaving] = useState(false);

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const value = sourceUrl.trim();
    if (!/^https?:\/\/.+/i.test(value)) {
      setError("Enter a valid http(s) URL");
      return;
    }

    setSaving(true);
    setError(null);
    setSuccess(false);
    try {
      const token = await getToken();
      await scrapeJobFromURL({ source_url: value }, token);
      setSuccess(true);
      setSourceUrl("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not scrape the URL");
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label className="block text-sm font-medium text-foreground" htmlFor="source_url">
          Job URL
        </label>
        <input
          id="source_url"
          type="url"
          className="w-full rounded-md border bg-background px-3 py-2 text-sm text-foreground shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          value={sourceUrl}
          onChange={(e) => setSourceUrl(e.target.value)}
          placeholder="https://example.com/jobs/1"
        />
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}
      {success && (
        <p className="text-sm text-foreground">
          Job queued for scraping. It will appear on{" "}
          <Link className="underline" href="/user-jobs">My Jobs</Link> shortly.
        </p>
      )}

      <button
        type="submit"
        disabled={saving}
        className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
      >
        {saving ? "Scraping..." : "Scrape"}
      </button>
    </form>
  );
}
