# Temporal Workflow Design

## 1. Responsibilities

Temporal orchestrates long-running and retryable job-search operations. The Go API starts workflows and sends signals. Workers poll Temporal task queues and execute activities. Workers do not call each other directly; Temporal is the coordination layer.

## 2. Initial workflow families

| Workflow | Purpose | Likely worker |
|---|---|---|
| `JobDiscoveryWorkflow` | Discover and normalize listings from configured sources | Python |
| `JobEvaluationWorkflow` | Evaluate jobs against one user's profile/skills | Python |
| `JobReviewWorkflow` | Coordinate review state and wait for user decisions where needed | Python/API signal |
| `DocumentGenerationWorkflow` | Generate CV/cover-letter variants | Python |
| `GenerateCoverLetterWorkflow` | Generate a cover letter for one evaluated job; one LLM activity writes the result back to the evaluation | Python |
| `JobApplicationWorkflow` | Execute an approved application attempt and record outcome | Browser + Python |
| `ProfileSyncWorkflow` | Reconcile profile/skills into normalized records | Python |
| `JobRepairWorkflow` | Reconcile recoverable inconsistencies | Python/admin |

Names are proposed logical boundaries. Keep workflows cohesive and avoid creating a workflow for every trivial database operation.

### Cover-letter generation (implemented)

`GenerateCoverLetterWorkflow` is the implemented document-generation workflow for cover letters. The Go API starts it with a deterministic workflow ID `cover-letter-gen-{evaluation_id}` and job type `cover_letter`, so double-clicks and retries are idempotent. The concurrency limiter maps `cover_letter` to the existing `job-evaluation` task queue with a cap of 2, so no new queue is introduced. The workflow marks the evaluation `pending`, runs a single LLM activity `generate_cover_letter_activity` with a 5-minute start-to-close timeout and a retry policy of 3 attempts, then PATCHes the letter text and metadata (or a safe fixed failure message) back to the Go API. This is document generation only: there is no R2 storage and no approval step. See `docs/superpowers/specs/2026-11-17-cover-letter-design.md` for the full decision record.

## 3. Task queues

Suggested queues:

- `job-discovery`
- `job-evaluation`
- `job-documents`
- `job-browser`
- `job-maintenance`

Python and browser workers register only the queues and activities they own. Configure separate worker concurrency/resource limits, especially for browser sessions.

## 4. Determinism and activity rules

- Workflow code must be deterministic. Put network, database, filesystem, LLM, and browser I/O in activities.
- Activities must be idempotent or use explicit idempotency keys.
- Set start-to-close and schedule-to-close timeouts intentionally.
- Use retries only for errors that can plausibly recover.
- Use heartbeats for long-running browser or scraping activities.
- Make cancellation cooperative and clean up browser contexts and temporary files.
- Version workflows when changing behavior that may affect in-flight executions.
- Do not store secrets or unnecessary personal data in workflow input/history.

Long-lived dispatcher workflows reset their event history with Continue-As-New before the server's hard history limit (50 MB in this deployment). A reset is only safe when no child workflow is in flight and no signal handler is running, so pending items are carried into the next generation without orphaning children. `ConcurrencyLimiter` triggers the reset on a local event-count/size threshold instead of waiting only for the server's "suggested" flag, because a continuously fed singleton rarely reaches an idle point. This replaced the previous behavior, where a busy limiter could grow until the history service terminated the run.

## 5. Human approval

The system must require an explicit approval before external application submission. Approval should be recorded as a durable application transition/activity, then signal or start the relevant workflow.

Do not infer approval from opening a page, generating a document, clicking a review card, or a timeout. The approval action must be attributable to the authenticated user.

## 6. Application outcomes and recovery

Use explicit outcomes:
- `applied`: external submission is confirmed.
- `failed`: submission is confirmed not to have succeeded and can be retried under policy.
- `submitted_unknown`: the browser/ATS result is ambiguous. Quarantine and require manual reconciliation.

Never blindly retry an ambiguous submit action. The workflow should first attempt a safe status check where possible, otherwise stop for user/operator review.

## 7. Browser worker isolation

The browser worker is a separate process/container with:
- independent concurrency limits,
- CPU/memory limits,
- browser version pinned in its image,
- isolated temporary directories and browser profiles,
- restricted network access where feasible,
- secrets/session material scoped to the minimum required operation,
- screenshots/downloads stored with controlled retention.

Do not expose browser debugging ports publicly.

## 8. Observability

Log workflow ID, run ID, activity type, job/application ID, and tenant ID where safe. Avoid logging CV contents, credentials, session cookies, or full prompt payloads. Emit metrics for workflow starts, completions, retries, failures, activity latency, queue backlog, and browser resource use.

### Tracing (§8.1)

Distributed tracing connects one user action end-to-end: API request → Temporal workflow → Python worker activities. The OTel trace ID is unified with the API's `request_id`, so the response `request_id` equals the trace ID shown in Jaeger. Tracing is opt-in via `OTEL_EXPORTER_OTLP_ENDPOINT`; unset means no-op (local dev unchanged).

Trace context flows across the API/worker boundary through Temporal SDK interceptors carrying W3C `traceparent` headers. Span names use route patterns (never raw paths with IDs), and spans must not contain CV contents, prompt payloads, credentials, or session cookies.

## 9. Failed-workflow retry sweep

Temporal retries activities according to each workflow's retry policy, but a workflow that exhausts its retries ends in `failed` and its application row stays `failed` until something re-queues it. Two machine-key endpoints re-queue those states, both selecting oldest `updated_at` first: `POST /v1/maintenance/retry-failed-jobs` reads up to 20 rows with `jobs.status = 'failed'`, and `POST /v1/maintenance/retry-failed-matching-evaluations` reads up to 50 evaluation rows where any of `evaluation_status`, `report_status`, `resume_status`, or `cover_letter_status` is `failed` and the linked `user_jobs.is_hidden` is false.

Job analysis uses `force=true` on `QueueAnalysisBulk`; failed compatibility evaluations are re-queued by passing their evaluation IDs to `POST /v1/matching-evaluations/bulk-generate-evaluation`, which sets `failed → queued_retry` only for rows that are actually `failed` without repeating the insert step. Reports and resumes use their bulk force paths; cover letters use variant `short` because the database does not record which variant failed. A single evaluation row can have several failed fields, and each failed field is re-queued. With one evaluation per user/job, the sweep selects failed evaluation rows directly, so the old `failed_only` fan-out by job slug and the risk of creating a fresh queued row for a user whose latest evaluation was not `failed` are gone. Items a usecase rejects are skipped and excluded from the reported counts, so one bad row does not block the rest of the sweep. The host cron calls both endpoints every 10 minutes; see `hermes/COMMON.md` in the homelab repo.

The sweep only touches `failed` rows. Rows stuck in `queued` or `queued_retry` can be re-queued with `force=true` through the normal bulk endpoints, but only when the row's `updated_at` is more than 30 minutes old and its deterministic Temporal workflow ID is not running. The five guarded workflow IDs are `job-analyze-{job_id}`, `match-job-{evaluation_id}`, `report-gen-{evaluation_id}`, `resume-gen-{evaluation_id}`, and `cover-letter-gen-{evaluation_id}`. Rows at `pending` keep their existing force rules and are not covered by the stale guard.
