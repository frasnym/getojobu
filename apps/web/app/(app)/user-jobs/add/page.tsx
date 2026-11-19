"use client";

import React, { useState } from "react";
import Link from "next/link";
import { AddJobForm } from "@/components/jobs/add-job-form";
import { ScrapeUrlForm } from "@/components/jobs/scrape-url-form";

type Tab = "manual" | "scrape";

export default function AddJobPage() {
  const [tab, setTab] = useState<Tab>("manual");

  return (
    <div className="mx-auto max-w-3xl space-y-6 p-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Add a job</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Add a job you found yourself. Only you can see it.
          </p>
        </div>
        <Link className="text-sm underline" href="/user-jobs">
          Back to My Jobs
        </Link>
      </div>

      <div className="flex gap-2 border-b">
        <button
          type="button"
          onClick={() => setTab("manual")}
          className={`px-4 py-2 text-sm font-medium ${
            tab === "manual"
              ? "border-b-2 border-primary text-foreground"
              : "text-muted-foreground"
          }`}
        >
          Manual Entry
        </button>
        <button
          type="button"
          onClick={() => setTab("scrape")}
          className={`px-4 py-2 text-sm font-medium ${
            tab === "scrape"
              ? "border-b-2 border-primary text-foreground"
              : "text-muted-foreground"
          }`}
        >
          Scrape from URL
        </button>
      </div>

      <div className="rounded-lg border bg-card p-6 text-card-foreground">
        {tab === "manual" ? <AddJobForm /> : <ScrapeUrlForm />}
      </div>
    </div>
  );
}
