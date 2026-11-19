# Manual job insertion

## Motivation and goals

Scraped listings cover job boards, but users also find roles through personal networks, recruiter emails, and company pages. This feature lets a user add those jobs by filling in the fields or by pasting a URL. A user-created job is private: only its owner can see it. Once created, it uses the same analysis, evaluation, resume, and cover-letter pipeline as a scraped job.

## Design decisions

The shared `jobs` table gains a nullable `owner_id` FK to `users(id)` (migration `000018_jobs_owner_id.sql`). `NULL` means a shared listing, unchanged from existing behavior; a non-null value means a private listing. This avoids a second job table while keeping one pipeline. The listing is also linked to the user in `user_jobs` through the normal jobs usecase, so ownership, saving, and evaluation continue to work without a parallel path.

Manual slugs are prefixed with `m-{first8 of user UUID}-`. Scraped slugs never use this prefix, so manual and scraped jobs cannot collide, and two users cannot collide with each other. A repeated insert by the same user deduplicates on the unique slug.

The URL scrape is a Temporal workflow on the Python worker. The Go API only creates the skeleton job and enqueues work; it never performs LLM calls or page fetches inside an HTTP handler.

## API endpoints

- `POST /v1/jobs/manual` — Clerk-authenticated. Body: `title`, `company`, `location` (required), plus optional `description`, `source_url`, `apply_url`, `posted_date`, `is_remote`, `visa_relocation`. Returns `201` with `CreateJobResponse`.
- `POST /v1/jobs/scrape-url` — Clerk-authenticated. Body: `source_url` (an `http(s)` URL). Returns `202` with a skeleton job at `scrape_new`; the worker moves it to `scrape_pending`.
- `PATCH /v1/jobs/{slug}` — extended with optional job fields and a new `slug` value for the scrape callback. The Go usecase applies these fields only when the current status is `scrape_pending`.

`JobResponse` now includes `source` so the frontend can badge manual and user-scraped jobs.

## Flow

Manual entry: handler resolves the Clerk principal, validates fields, and calls the jobs usecase `CreateManual`. That method builds the prefixed slug, sets `source = manual` and `owner_id`, then calls the jobs usecase `Create`, which inserts the row and the `user_jobs` link in one transaction.

URL scrape: the jobs usecase `ScrapeURL` creates a skeleton job at `scrape_new` with `source = user_scrape`, the user's `owner_id`, and the original URL, then calls `Workflows.StartUserJobScrape`. That wrapper enqueues a `LimiterWorkItem` of type `user_job_scrape` with a stable item ID `user-job-scrape-{slug}`. The concurrency limiter maps the type to `UserJobScrapeWorkflow` on the `job-scrape` queue, where the workflow and its activities are registered. Starting the workflow directly on the `job-limit` queue would not work: that queue only polls the limiter workflow.

The Python workflow first calls `mark_scrape_pending` (PATCH `scrape_new → scrape_pending`), then `fetch_and_extract_job` (HTTP fetch, HTML-to-text, LLM extraction), `update_scraped_job` (regenerate the prefixed slug and PATCH the skeleton to `new` with the extracted fields), then `queue_scraped_job_analysis` (POST `/v1/jobs/bulk-queue-analyze`). On failure it calls `mark_scrape_failed` to record `scrape_failed` and a failure message. Activity retries: 2 for fetch, 3 for API calls. Timeouts: 60s for fetch plus LLM, 30s for API calls.

If the workflow cannot be enqueued, the API marks the skeleton job `scrape_failed` so the UI offers a retry. A `scrape_failed` job can be retried from the My Jobs page, which submits the same URL and creates a fresh skeleton.

## Visibility model

Clerk callers get `WHERE owner_id IS NULL OR owner_id = :current_user_id` on `GetBySlug` and `List`, so guessing a private slug returns 404. API-key callers and internal Go reads pass an empty user ID and receive all jobs, including private ones. The Python worker depends on this to read status, PATCH updates, and queue analysis for private jobs.

## Status state machine

```
                 API creates skeleton        worker starts scrape
scrape_new ─────────────────────────────► scrape_pending ──scrape succeeded──► new ──► queued ──► analyze_pending ──► analyzed
     │                                         │                                                     │
     │ dispatch failure                        │ scrape failed                                      ├──► failed ──► queued_retry
     ▼                                         ▼                                                     └──► queued_retry
scrape_failed ◄─────────────────────────────────┘
      │
      └── retry ──► scrape_pending
```

All transitions are validated by `ValidateJobStatusTransition`. The new paths are `scrape_new → scrape_pending`, `scrape_new → scrape_failed`, `scrape_pending → new`, `scrape_pending → scrape_failed`, and `scrape_failed → scrape_pending`.
