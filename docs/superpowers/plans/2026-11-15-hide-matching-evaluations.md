# Show/Hide Per-Job Ack / "Not Relevant" — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let each user mark any matching evaluation as **acknowledged** (saw it, no action needed) or **not relevant** (ignore/fade it out). The setting is per-job, settable from the job-listing or report view, and honored by the user's job-list endpoint. Clerk authentication only.

**Architecture:** One flag column per user-job row (`user_jobs.acknowledged` or `user_jobs.is_not_relevant`, or a single `user_jobs.status` enum — see decision). The list endpoint for a Clerk user filters out rows the user has marked "not relevant" and visually distinguishes "acknowledged" rows. The API remains authoritative; the frontend is a presentation aid.

**Tech Stack:** Go 1.27.3 + standard library `net/http` + `http.ServeMux`, `pgx` for PostgreSQL, Goose for migrations, Next.js + TypeScript on the frontend.

**Spec:** This plan is the source-of-truth doc for this feature. Implement against the existing contract in `docs/api/api-design.md` and `apps/backend/migrations/`.

---

## Global Constraints

- **Authentication:** Clerk bearer tokens only for this toggle; machine API keys never use these flag endpoints.
- **Go router:** standard library `net/http` + method-aware `http.ServeMux`. No Chi, Echo, or other framework. (Note: `docs/api/api-design.md` says Chi — flag this conflict, don't silently adopt it.)
- **Go baseline:** Go 1.27.3.
- **Database:** Neon PostgreSQL; versioned migrations via Goose under `apps/backend/migrations/`.
- **Tenant isolation:** every repository method receives the authenticated user context; scope all reads/writes to it. Never trust a user ID from the request body.
- **API envelope:** unified success/error/pagination envelope per `docs/api/api-design.md` §5.
- **Idempotency:** flipping a flag back and forth is a no-op write with a 200; repeated same-value toggles must not create history rows.
- **Never delete or mutate live data without testing:** run migrations on a disposable database first; backward-compatible schema only.
- **Secrets:** no secrets or sensitive personal data in logs, workflow inputs, or generated artifacts.

## Decision to Confirm (ask now, before implementing)

Pick one; the plan branches on it:

- **Choice A (two booleans):** `user_jobs.acknowledged` + `user_jobs.is_not_relevant`. Explicit, easy to query, but they can be in a weird combination (both true) unless you add a CHECK constraint or enforce in the application layer.
- **Choice B (single status enum):** `user_jobs.list_status` VARCHAR with values `shown` / `acknowledged` / `not_relevant`. Mutually exclusive by definition; `shown` = visible in list, `acknowledged` = visible but faded/disabled, `not_relevant` = filtered out of the list. Cleaner semantics, one column.
- **Choice C (single boolean):** `user_jobs.hidden` or `is_not_relevant` plus a separate `acknowledged`. Same expressiveness as A, fewer columns.

I recommend **Choice B** (`list_status` enum): the three states map exactly to what the UI needs (show / fade / hide), the database enforces mutual exclusivity, and the list filter is a single `WHERE list_status != 'not_relevant'`. If you prefer two booleans, say so and I'll adjust the plan.

## Review Focus

1. **Unauthorized toggle:** any job flag endpoint called with no token or with another user's job ID → `401` or `403` (tenant mismatch). Test: POST a toggle with no token → 401; POST a toggle for a job owned by user B while authenticated as user A → 403.
2. **Machine-key callers unaffected:** worker/machine-list endpoints ignore the ack/not-relevant flags entirely; they see every row. Test: flag a job as not_relevant, then call the worker path and assert the row still appears.
3. **Default state:** every user-job row created before this feature defaults to `shown` (visible) so existing data is unaffected. Test: new row defaults to shown; pre-existing rows query back as shown.
4. **Filter is tenant-scoped:** user A toggling a job must not affect user B's view of the same shared job. Test: toggle as A, query as B, assert B's list is unchanged.
5. **Frontend consistency:** after toggling, the list must reflect the change on the next fetch or immediately; no cached UI showing a hidden row after the user flips it. Test: toggle → re-render asserts row gone (or faded).

---

## Task 1: Schema — add per-job ack / not-relevant flags

**Files:**
- Create: `apps/backend/migrations/NNNNN_add_job_feedback_flags.sql`

**Interfaces:**
- Consumes: existing `user_jobs` table (the per-user evaluation state table).
- Produces: the chosen flag design (Choices A/B/C above) with a default of `shown` for all rows, plus a `-- +goose Down` rollback.

- [ ] **Step 1: Write the migration**

Mirroring the style of `000009_matching_evaluations.sql` and `000010_match_report_schema.sql`:

*If Choice B (recommended, enum):*

```sql
-- +goose Up
ALTER TABLE user_jobs
  ADD COLUMN list_status VARCHAR(16) NOT NULL DEFAULT 'shown'
    CHECK (list_status IN ('shown', 'acknowledged', 'not_relevant'));

-- +goose Down
ALTER TABLE user_jobs
  DROP COLUMN list_status;
```

*If Choice A (two booleans):*

```sql
-- +goose Up
ALTER TABLE user_jobs
  ADD COLUMN acknowledged  BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN is_not_relevant BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE user_jobs
  DROP COLUMN acknowledged,
  DROP COLUMN is_not_relevant;
```

- [ ] **Step 2: Verify the migration runs cleanly**

Run Goose on a disposable Neon branch (or local PostgreSQL): apply from an empty schema, confirm defaults and the CHECK constraint, then run the down migration and confirm the table is unchanged.

- [ ] **Step 3: Commit**

```bash
git add apps/backend/migrations/NNNNN_add_job_feedback_flags.sql
git commit -m "migrations: add job ack / not-relevant flags to user_jobs"
```

## Task 2: Backend API — add per-job flag endpoints

**Files:**
- Modify: `apps/backend/internal/repository/` — add `SetJobFeedback(ctx, userJobID, status)` (Choice B) or `SetJobAcknowledged` / `SetJobNotRelevant` (Choice A).
- Create: `apps/backend/internal/deliveries/api/handler/jobfeedback/` — two endpoints:
  - `POST /v1/me/jobs/{jobID}/feedback` with body `{"status": "acknowledged" | "not_relevant"}`
  - `GET  /v1/me/jobs/{jobID}` returns the current `list_status` for the authenticated user
- Modify: `apps/backend/docs/docs.go` (regenerate via `swag init`)
- Test: `apps/backend/tests/...` handler test file for `jobfeedback`

**Interfaces:**
- Consumes: authenticated Clerk user context (tenant) + `jobID` from the URL path (never the body).
- Produces:
  - `GET /v1/me/jobs/{jobID}` returns `{ "user_job_id": "...", "list_status": "shown" | "acknowledged" | "not_relevant" }`.
  - `POST /v1/me/jobs/{jobID}/feedback` with valid `{"status": ...}` → `200` updated object; unknown status or missing status → `400` validation error; same status twice → `200` no-op.
  - All endpoints behind Clerk auth → `401` with no token; non-tenant job ID → `403`.

- [ ] **Step 1: Write the repository methods**

`SetJobFeedback(ctx, userJobID UUID, status string) error` — a single `UPDATE user_jobs SET list_status = $2 WHERE id = $1 AND user_id = $2` style write scoped by the authenticated user ID, with input validation of the enum values in Go before the query.

- [ ] **Step 2: Wire the GET + POST endpoints**

Handler stays thin: read `jobID` from the URL path → validate the path is a UUID → resolve tenant from the verified Clerk claim → repository call → write the envelope. No business logic in the handler.

- [ ] **Step 3: Write the failing test cases**

```go
func TestJobFeedback(t *testing.T) {
    // given job row owned by user A, list_status = 'shown'
    // when POST /v1/me/jobs/{jobA}/feedback {"status":"not_relevant"}
    // then 200 and list_status == 'not_relevant'

    // when POST again with the same status
    // then 200 no-op (no duplicate activity / no error)

    // when POST with a value from another user's job
    // then 403 (tenant mismatch)

    // when POST with no token
    // then 401

    // when POST {"status":"mystery"}
    // then 400 validation failed
}
```

- [ ] **Step 4: Run tests and verify**

Run `go test ./apps/backend/internal/deliveries/api/handler/jobfeedback/...` and the repository tests; assert PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/backend/internal/deliveries/api/handler/jobfeedback/ \
       apps/backend/internal/repository/ apps/backend/docs/swagger.yaml
git commit -m "api: add POST /v1/me/jobs/{id}/feedback to ack or mark not relevant"
```

## Task 3: Data layer — filter and surface feedback in the list

**Files:**
- Modify: `apps/backend/internal/repository/` — the job-list method behind `GET /v1/me/jobs` (and any matching-evaluation list method): add the list-status filter and include the flag in the projected row.
- Modify: `apps/backend/internal/deliveries/api/handler/jobs/jobs.go` — ensure the response schema carries `list_status` / feedback flag for each job row.
- Test: `apps/backend/internal/repository/*_test.go` plus a cross-tenant integration test.

**Interfaces:**
- Consumes: the flag column(s) added in Task 1.
- Produces:
  - `GET /v1/me/jobs` returns every row with a `list_status` field for Clerk users.
  - The list excludes rows with `list_status = 'not_relevant'` for the authenticated user; `acknowledged` rows stay visible (fade/disabled in UI).
  - Machine-key list paths ignore the flag and return all rows.

- [ ] **Step 1: Filter the list and project the flag**

Extend the tenant filter in the list query to `AND (list_status IS NULL OR list_status != 'not_relevant')` — note the `IS NULL` guard for any legacy or pre-migration rows; document why it's there. Project `list_status` in the SELECT.

- [ ] **Step 2: Update the response model / OpenAPI types**

Add `list_status` to the job row definition used by the list endpoint and its Swagger model.

- [ ] **Step 3: Write the repository/list tests**

Test: user A sets job J to `not_relevant`; querying as A returns J filtered out, querying as B (same shared job) returns J visible, and the worker/machine path returns J regardless. Re-run the existing cross-tenant test unchanged.

- [ ] **Step 4: Run tests and verify**

Run `go test ./apps/backend/internal/repository/...` and the handler list tests; assert PASS.

- [ ] **Step 5: Commit**

```bash
git add apps/backend/internal/repository/ apps/backend/docs/swagger.yaml
git commit -m "repo: filter not_relevant jobs from Clerk lists and project the flag"
```

## Task 4: Frontend — read the flag and add the toggle UI

**Files:**
- Modify: `apps/web/lib/api/endpoints.ts` — `getJobs()` returns `list_status` per job; add `getJobFeedback(jobID)` GET and `setJobFeedback(jobID, status)` POST.
- Modify: `apps/web/hooks/use-jobs.ts` — carry `list_status` through filtering and expose a `setFeedback(jobID, status)` function; re-fetch on success.
- Modify: `apps/web/components/jobs/job-card.tsx` and/or `job-report-header.tsx` — render `acknowledged` as faded/disabled and `not_relevant` as hidden (already filtered at the API, so just visual handling for `acknowledged`); add an "Acknowledge / Not relevant" toggle control.
- Test: `apps/web/tests/hooks/use-jobs.test.ts` and `apps/web/tests/components/jobs/job-card.test.tsx`

**Interfaces:**
- Consumes: `GET /v1/me/jobs` rows include `list_status`; `GET /v1/me/jobs/{jobID}/feedback` returns the current value.
- Produces: the job card shows an acknowledgement state (faded/disabled for `acknowledged`, visible for `shown`) and a toggle that calls `POST /v1/me/jobs/{id}/feedback`, updates local state, and refreshes the list; loading/error states handled.

- [ ] **Step 1: Expose the flag in the job type and endpoints**

Add `list_status?: "shown" | "acknowledged" | "not_relevant"` to `apps/web/lib/api/types.ts` `Job`; add `getJobFeedback` / `setJobFeedback` to `endpoints.ts` using the existing client + token flow.

- [ ] **Step 2: Render acknowledgement in the card**

In `job-card.tsx`: if `list_status === 'acknowledged'` render the card muted (opacity + disabled cursor, maybe an "acknowledged" badge); if `shown` render normally. `not_relevant` should not appear in the list, so no visual needed there.

- [ ] **Step 3: Add the toggle control**

Add a small action (button, dropdown, or icon toggle) on the card and/or in the report view: "Acknowledge" / "Mark not relevant". On success, call the hook's `setFeedback`, refresh the list, and show a brief inline confirmation; on failure show the existing error banner.

- [ ] **Step 4: Update the hook**

`useJobs` exposes `setFeedback(jobID, status): Promise<void>` that performs the POST and calls a `onListChanged` callback so the page re-fetches; ensure it re-runs the effect on Clerk session change.

- [ ] **Step 5: Write the frontend tests**

```ts
it("renders an acknowledged job as faded/disabled", () => {
  render(<JobCard job={{ ...base, list_status: "acknowledged" }} />);
  expect(card).toHaveClass(/muted|disabled/i);
});

it("toggles a job to not_relevant and refreshes the list", async () => {
  // assert click → POST called → row removed from list
});
```

- [ ] **Step 6: Run type check and tests**

Run the frontend type checker and `apps/web/tests/hooks/use-jobs.test.ts` + `job-card.test.tsx`; assert PASS with no new TypeScript errors.

- [ ] **Step 7: Commit**

```bash
git add apps/web/lib/api/endpoints.ts apps/web/lib/api/types.ts \
       apps/web/hooks/use-jobs.ts apps/web/components/jobs/job-card.tsx \
       apps/web/tests/hooks/use-jobs.test.ts apps/web/tests/components/jobs/job-card.test.tsx
git commit -m "frontend: add ack / not-relevant toggle to job cards with list refresh"
```

## Task 5: OpenAPI / docs — record the contract change

**Files:**
- Modify: `docs/api/api-design.md` — extend "Job discovery and review":

  ```text
  GET   /v1/me/jobs/{jobID}/feedback   → { user_job_id, list_status: shown|acknowledged|not_relevant }
  POST  /v1/me/jobs/{jobID}/feedback   body { "status": "acknowledged" | "not_relevant" } → 200
  ```

- Modify: `apps/backend/docs/docs.go` / `apps/backend/docs/swagger.yaml` via `swag init` (regenerate; never hand-edit).
- Update: `docs/rollout/implementation-rollout.md` — one line noting the per-job ack/not-relevant feature and that it ships in the frontend phase.

**Interfaces:**
- Consumes: endpoints defined in Task 2.
- Produces: contract docs that match the running implementation.

- [ ] **Step 1: Document the endpoints** in `docs/api/api-design.md`.
- [ ] **Step 2: Regenerate Swagger and diff** to confirm it reflects the new endpoints and `list_status`.
- [ ] **Step 3: Add a rollout note** and commit.

```bash
git add docs/api/api-design.md apps/backend/docs/swagger.yaml docs/rollout/implementation-rollout.md
git commit -m "docs: record job feedback contract and regenerate Swagger"
```

---

## Execution Handoff

Plan complete. Core changes: per-job `list_status` (shown / acknowledged / not_relevant) on `user_jobs`, new feedback endpoints behind Clerk auth, list filtering, and a toggle in the job card.

Which execution approach would you prefer?

- **Subagent-driven** — a fresh subagent implements each task and a fresh reviewer checks it before the next one starts, then a whole-branch review at the end. Most thorough; costs a fresh context per task and per review.
- **Native** — I implement every task myself in this session, then one fresh reviewer on the most capable model checks the whole branch. Cheapest and fastest; no independent review until the end.

For this plan I recommend **Native**, because the tasks are small, tightly chained (schema → API → repo filter → frontend), and the risk of a shipped mistake is low (a single enum flag with well-defined semantics and defaults). Does the plan capture what you want, and which approach should we use?

**Open items to note:**
1. `docs/api/api-design.md` says "Go router: Chi," conflicting with repo-wide `AGENTS.md` mandating `net/http` + `http.ServeMux`. Marked for your review; not resolved in the plan.
2. **Choice pending:** pick Choices A (two booleans) or B (single enum) before implementation — I drafted Task 1 and the repo tests around Choice B (recommended).
3. Frontend inspection was limited to the listing components; an actual reviewer should walk the full UI to confirm no second place renders matching evaluations that also needs to respect the flag.
