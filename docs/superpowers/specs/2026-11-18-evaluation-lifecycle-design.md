# Evaluation lifecycle — split insert from trigger, one evaluation per user/job

## 1. Intent

Today `POST /v1/jobs/bulk-analyze-compatibility` does four things in one call: resolve a job by slug, fan out to the users linked to that job, insert a `matching_evaluations` row, and start a Temporal workflow. Because the row is created inside that call, the input has to be the job slug — there is no evaluation ID yet.

Two things are wrong with that shape. First, there is no way to create an evaluation row without starting the workflow. Second, if the insert succeeds but `StartLimiterWorkItem` fails, the row is left at `queued` with no workflow behind it; no worker ever runs, so nothing flips it to `failed`. That hole is rare today but becomes the normal failure case if the two steps are separate calls.

This design splits the call into an insert step and a trigger step, adds an explicit `new` state before `queued`, and reduces `user_jobs` to one evaluation row each.

## 2. What the user decided

- Split into two endpoints. `POST /v1/jobs/evaluation` inserts an evaluation with status `new` and stops. A second endpoint, named `POST /v1/matching-evaluations/bulk-generate-evaluation` in the request, starts Temporal.
- Trigger sets `queued` for a `new` row.
- Retry sets `queued_retry` for a `failed` row. Retry never goes through `new` and never creates a new row.
- `success` is terminal. No endpoint may re-evaluate a successful row.
- One `matching_evaluations` row per `user_jobs` row. The current one-to-many history is not needed soon.
- When collapsing existing duplicate rows, keep the latest by `created_at`, regardless of status.
- `user_jobs.latest_evaluation_id` is no longer needed and is dropped.
- The report-retry API accepts both Clerk and API key callers, like the other endpoints. Clerk callers are checked for ownership of the evaluation before report generation starts.
- Keep the second route name `bulk-generate-evaluation`.

## 3. What I decided (my calls, not the user's)

- Trigger order is start Temporal first, then flip the status with a conditional update — `new → queued`, `failed → queued_retry`, `WHERE evaluation_status IN ('new', 'failed')`. If the start fails, the row keeps its original status, so there is never a `queued` row without a workflow. To close the race where the worker begins before the flip lands, `new → pending` is added as an allowed transition. The workflow ID stays `match-job-{evaluation_id}`, so Temporal dedupes a double start.
- Insert is create-if-absent, not a blind upsert: if a row already exists for the `user_job_id`, return it unchanged whatever its status. That is what keeps `success` terminal and stops an insert from wiping an in-progress row. Only the trigger changes status.
- The insert stores the derived job profile — metadata stripped, `family` defaulted to `other`, and `title`/`company`/`location` copied from the jobs table — so the trigger can send the stored bytes straight to Temporal and the workflow input stays identical to today.
- Both endpoints follow the existing auth pattern: `ValidClerkOrAPIKey`. A Clerk caller's user comes from the token, ownership of the job link or evaluation is checked from the principal, and any supplied user ID is rejected. API key callers are trusted: they supply no `user_ids`/`user_id` and no per-request user ownership check is done. The insert resolves its target users from `user_jobs`, the trigger keys off the evaluation ID alone, and the retry sweep passes only evaluation IDs.
- Drop `force` and `failed_only` from this endpoint's request shape. With one row per user/job, `force=true on success → new row` is gone, and a failed row is retryable by definition. `failed_only` existed only because the old endpoint fanned out by slug across users; with evaluation IDs as input it has no meaning.
- Re-triggering a failed row clears only the five evaluation-result columns (`evaluation_data`, `overall_score`, `eligibility_status`, `recommendation_label`, `failure_message`) but leaves report, resume, and cover-letter columns and their statuses untouched. `candidate_profile_data` and `job_profile_data` are workflow inputs, not results: they are kept intact across retries so every retry rebuilds a non-empty workflow input. Those artifacts belong to the user/job, and the one-row model has nowhere else to put them.
- The insert endpoint keeps slug-based input and resolves users from `user_jobs`, so existing callers change as little as possible. The trigger endpoint takes evaluation IDs only.
- In-progress rows (`queued`, `queued_retry`, `pending`) passed to the trigger return a per-item error; they are not force-restarted.

## 4. Goals

- One call creates a row, one call starts work. A `new` row means "inserted, not started"; a `queued` row always has a workflow behind it.
- Exactly one evaluation per user/job, enforced by a database constraint.
- `success` is final; `failed` is retryable in place.
- Every existing caller (web, Python worker, retry sweep) uses the new two-step flow.

## 5. Non-goals

- Evaluation history, an archive table, or comparing scores across prompt versions.
- Re-evaluation after a prompt or scoring-config change.
- Changing report, resume, or cover-letter generation endpoints.
- Frontend UI changes beyond swapping API calls.
- Adding a Temporal worker in Go.

## 6. Anti-goals — what would make this a failure

- A `queued` row with no workflow started.
- Two evaluation rows for the same user/job.
- A `success` row flipped back to a retryable state.
- A machine caller retrying an evaluation belonging to another tenant by supplying an ID.
- Losing report/resume/cover-letter artifacts when a failed evaluation is retried.

## 7. Constraints from the repository

- Backend: Go 1.27.3, `net/http` with method-aware `http.ServeMux`, `Request.PathValue()`, `pgx`, Goose migrations, `log/slog`. No new HTTP framework or database.
- Current statuses in `internal/utils/constants/matching_status.go`: `queued`, `queued_retry`, `pending`, `success`, `failed`. Only transitions into `pending` are validated today, and `queued_retry → pending` is already allowed.
- The Python worker accepts both `queued` and `queued_retry` and sets `pending`, then `success` or `failed` in `workers/python/workflows/match_job.py`. No worker change is needed for the status model.
- `matching_evaluations.user_job_id` has no unique constraint today. `user_jobs.latest_evaluation_id` from migration `000014` is the only foreign key pointing at `matching_evaluations.id`, and it is dropped in the same migration as the collapse, so no other table blocks row deletion.
- Latest migration is `000017`, so the new one is `000018`.
- API contract: Swagger is generated from Go struct comments via `swag init`. Do not hand-edit `apps/backend/docs/swagger.json`, `swagger.yaml`, or `docs.go`.
- Current callers to update: `apps/web/lib/api/endpoints.ts` (`triggerJobCompatibility`), `workers/python/repository/getojobu_api.py` (`trigger_bulk_analyze_compatibility`), and `internal/usecase/retry/retry.go` (`FailedMatchingEvaluations`).
- `docs/database/data-model.md` §4 explicitly requires multiple historical evaluations per user/job and says not to use a unique constraint on only `(user, job)`. That document must be updated as part of this work.

## 8. The how

### 8.1 Lifecycle

```
insert            new
trigger        -> queued  -> pending -> success   (terminal)
failed retry   -> queued_retry -> pending -> success | failed
```

The trigger endpoint accepts only `new` and `failed` input rows. Everything else is a per-item error in the response, not a whole-request failure.

### 8.2 Migration 000018

In one migration, in this order:

1. Delete all but the latest `matching_evaluations` row per `user_job_id`, ranked by `created_at DESC, id DESC`.
2. Add `UNIQUE (user_job_id)` on `matching_evaluations`.
3. Drop `user_jobs.latest_evaluation_id` and `idx_user_jobs_latest_eval`.
4. Drop `idx_matching_evaluations_user_job_id`; the unique constraint supplies that index.

Run from an empty database and against a staging copy with duplicates first. The delete is destructive and needs explicit approval before it runs on production.

### 8.3 Go backend

- Add `MatchingStatusNew` to `MatchingEvaluationStatus`.
- Extend `ValidateMatchingStatusTransition` to allow `new → queued`, `failed → queued_retry`, and `new → pending`.
- Add the two routes next to the existing jobs and matching-evaluation routes, both behind `auth.ValidClerkOrAPIKey`.
- Insert request: an array of items, each carrying only `slug`, returning the created evaluation IDs and statuses. Validate that a Clerk caller is allowed to create an evaluation for its token user; API key callers are trusted and the usecase resolves every linked user from `user_jobs`.
- Insert usecase: split the front half of `TriggerCompatibilityBulk` into its own method. Resolve job by slug, require `analyze_result`, resolve target users, compose the candidate profile and the derived job profile, load the active scoring config, then create a row with status `new` only when none exists for that `user_job_id`. Return a summary per item that includes the evaluation ID and its current status.
- Trigger request: an array of evaluation IDs only. Trigger usecase: load each row by ID, verify the row's `user_job_id` belongs to the authenticated Clerk user (API key callers are trusted and skip this per-item check), build `MatchJobWorkflowInput` from the stored candidate/job profile data and the scoring config referenced by the row, then start `match-job-{evaluation_id}` through the existing limiter. Only after a successful start, flip the status conditionally — `new → queued` or `failed → queued_retry`. Return a summary per ID with the new status.
- Repository: replace `GetLatestByUserAndJob` with `GetByUserJob`; make `Create` an upsert on `user_job_id`; remove the `latest_evaluation_id` update from `Create`.
- On a failed-row retry, clear the evaluation-result columns listed in §3 in the same update that sets `queued_retry`.
- Keep the existing envelope and partial-results behavior: one bad item does not stop the rest of the batch.
- Add a user-facing report-retry endpoint under matching evaluations, registered with `auth.ValidClerkOrAPIKey`, taking an evaluation ID. For a Clerk caller, verify the evaluation belongs to the authenticated user before starting report generation. Reuse the existing report-generation usecase. Do not expose the worker-only `bulk-generate-report` route to the web.
- Update handler struct comments, then regenerate Swagger with `swag init`.

### 8.4 Callers

- Temporal is the primary caller of both endpoints. `analyze_job_activity` calls insert and then trigger after a job reaches `analyzed`. Remove the `try/except` at `usecase/analyze_job/service.py:121–131` that currently swallows the trigger error, so a failed trigger fails the activity and Temporal retries both calls. The insert is create-if-absent, so the retry is safe.
- `apps/web/lib/api/endpoints.ts`: `triggerJobCompatibility` calls insert, then trigger. If the trigger call fails, the row stays `new` and a later click can retry it without creating a duplicate. The `report_status = failed` branch in `use-user-jobs.ts:366–374` calls the new report-retry endpoint instead of forcing a whole new evaluation.
- `workers/python/repository/getojobu_api.py`: replace `trigger_bulk_analyze_compatibility` with one call per new step. Update its test at `workers/python/tests/test_getojobu_api.py`.
- `internal/usecase/retry/retry.go`: `FailedMatchingEvaluations` now collects evaluation IDs instead of slugs. Remove the `seen[slug]` dedupe and the `FailedOnly` flag; pass failed IDs straight to the trigger usecase. The other retry batches (report, resume, cover letter) are unchanged.
- `docs/api/api-design.md`, `docs/database/data-model.md` §4, and `docs/workflows/temporal-workflows.md` § retry paragraph: update to match the new contracts.

## 9. Tests

- Migration test: runs clean from an empty database; collapses duplicates so only the latest row per user/job survives; the unique constraint rejects a second row.
- Go usecase tests: insert twice yields one row; insert on an existing `success` or in-progress row leaves its status unchanged; the stored job profile has metadata stripped and includes title/company/location; trigger on `new` starts a workflow; trigger twice starts one workflow; trigger on `success` returns an error; retry on `failed` sets `queued_retry` without creating a row; a failed-to-start workflow leaves the row at `new`, not `queued`; a tenant cannot trigger another user's evaluation.
- Go handler tests: both endpoints plus the report-retry endpoint, auth, validation, per-item partial failure, Clerk ownership rejection, and the response summaries.
- Python repository test: both new calls hit the expected paths and unwrap the responses.
- Run the repository's documented Go tests, Python tests, and `swag init` before reporting completion.

## 10. What is left to the builder

- Exact Go type, method, and file names, following the conventions in the matching usecase and jobs handler packages.
- The concrete upsert SQL and the statement that ranks duplicate rows.
- The response summary shape for each endpoint, as long as it includes the evaluation ID where the caller needs it.
- Error codes and messages, using the existing `httpx` envelope.

## 11. Open items / approvals needed

- The second endpoint keeps the name `bulk-generate-evaluation`. It reads oddly beside `bulk-generate-report` and `bulk-generate-resume`, but the name is settled.
- Resolved: API key callers are trusted and no per-request user check is done — neither insert (`user_ids`) nor trigger (`user_id`) accepts a user ID from the request. Clerk ownership is still checked. The insert derives users from `user_jobs` and the trigger keys off the evaluation ID, so a caller cannot impersonate another user by supplying an ID; a compromised API key remains a trusted-caller risk accepted by this design.
- The duplicate-row delete is destructive. Run it first against a backup/staging copy and get explicit approval before production.
- This reverses the documented decision at `docs/database/data-model.md:49`; updating that document is part of the work, not an optional follow-up.
- Dropping evaluation history is permanent. If history is wanted later, it needs a new table and can only contain data kept from this point forward.
- No commit is included. Get approval before committing anything from this design.
