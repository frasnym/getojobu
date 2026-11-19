# Manual Job Insertion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to manually add jobs — either by filling in details or by pasting a URL for LLM-powered scraping — with private ownership so manual jobs are invisible to other users.

**Architecture:** Add a nullable `owner_id` column to the shared `jobs` table. Jobs with `owner_id IS NULL` remain shared/scraped (unchanged behavior). Jobs with a non-null `owner_id` are private to that user. Manual job slugs are prefixed with `m-{first8 of user UUID}-` to prevent collision with scraped jobs. A new `UserJobScrapeWorkflow` on the Python worker handles URL-based extraction via LLM. Manual jobs enter the same analysis pipeline as scraped jobs.

**Tech Stack:** Go 1.27.3, PostgreSQL (Neon), Temporal, Python worker, Next.js + TypeScript

**Spec:** This plan — brainstorming conversation approved the design inline.

## Global Constraints

- Go 1.27.3 toolchain; standard library `net/http` routing with `http.ServeMux`
- No new frameworks or dependencies beyond what's already in the project
- Swagger annotations on all new handlers; regenerate `docs/swagger.json` via `swag init`
- All new SQL uses parameterized queries; no string interpolation
- For Clerk callers, the `owner_id` visibility filter must never leak a private job to another user; API-key callers keep full access
- Slug prefix `m-{first8_of_user_uuid}-` for all manual/user-scrape jobs
- Manual jobs go through the full existing pipeline: `new → queued → analyze_pending → analyzed`
- Migration 000018 is applied as a controlled deployment step before the new endpoints are released. It is not run by API replicas at startup.

## Review Focus

1. **Visibility leak on shared endpoints** — for Clerk callers, `GET /v1/jobs` and `GET /v1/jobs/{slug}` must filter `owner_id IS NULL OR owner_id = :user_id`; a private job slug guessed by another user must return 404, not the job. API-key callers keep full access. Add test in Task 2.
2. **Slug collision between manual and scraped jobs** — a scraped job with slug `senior-engineer-acme-berlin` must not conflict with manual slug `m-abcd1234-senior-engineer-acme-berlin`. Verified by the `m-` prefix and unique constraint. Add test in Task 2.
3. **Slug collision between two users adding the same manual job** — `m-{user1_8chars}-title-co-loc` vs `m-{user2_8chars}-title-co-loc` are distinct. If the same user adds the same job twice, `ON CONFLICT (slug) DO NOTHING` deduplicates correctly. Add test in Task 2.
4. **Scrape-pending skeleton job left orphaned** — if the scrape workflow fails, the job sits at `scrape_pending` with minimal data. The `scrape_failed` status and `failure_message` capture this; the user can retry or delete. Add test in Task 5.
5. **Status transition validation for new statuses** — `scrape_pending` and `scrape_failed` must only transition where permitted. Existing terminal states must not break. Add test in Task 1.

## Plan review corrections (2026-11-18)

These decisions were agreed before implementation. They override any conflicting wording in the task steps below.

- Visibility: the `owner_id` filter applies only to Clerk callers, whose internal user ID is non-empty. API-key and internal Go callers see all jobs, including private ones. `UpdateStatus`, `QueueAnalysisBulk`, and the Python worker read with an empty user ID and depend on this.
- Ownership: `CreateManual` and `ScrapeURL` call the jobs usecase `Create`, not `repo.Jobs.Create`, so the existing transaction and `user_jobs` link creation are reused.
- Scrape dispatch: `UserJobScrapeWorkflow` is started through `StartLimiterWorkItem` with a new `LimiterJobTypeUserJobScrape`. The workflow and activities are registered on the queue the limiter dispatches to. There is no direct `ExecuteWorkflow` on `job-limit`.
- Job updates: `UpdateStatus` stays a single method and takes `models.UpdateJobParams` everywhere. There is no separate scrape-specific update method. All call sites, generated mocks, and tests are updated.
- Python API client: `patch_job_status` is extended to carry the scraped fields in `workers/python/repository/getojobu_api.py`.
- Frontend: the My Jobs detail view is `JobDetailDrawer`, not `[slug]/page.tsx`. `lib/api/types.ts` gains `source`, `failure_message`, and `visa_relocation` on `Job`.
- Slug parity: Python uses Go's `untitled` fallback and Unicode-aware normalization so regenerated slugs match `BuildManualJobSlug`.
- Commits: one commit per task as listed below. Each commit requires explicit approval before it is made. Every `git add` names files individually; never `git add .` or a whole directory.

---

### Task 1: Database Migration + Status Constants + Slug Builder

**Files:**
- Create: `apps/backend/migrations/000018_jobs_owner_id.sql`
- Modify: `apps/backend/internal/utils/constants/jobs_status.go`
- Modify: `apps/backend/internal/models/job.go`
- Test: `apps/backend/internal/utils/constants/jobs_status_test.go`
- Test: `apps/backend/internal/models/job_test.go`

**Interfaces:**
- Consumes: nothing (foundation task)
- Produces:
  - `Job.OwnerID *uuid.UUID` field on the `Job` struct
  - `CreateJobParams.OwnerID *uuid.UUID` field
  - `CreateJobParams.Status string` field (empty means the existing `new` default)
  - `StatusScrapePending JobStatus = "scrape_pending"` constant
  - `StatusScrapeFailed JobStatus = "scrape_failed"` constant
  - `BuildManualJobSlug(userID, title, company, location string) string` function
  - Updated `ValidateJobStatusTransition` accepting `scrape_pending` and `scrape_failed`

- [ ] **Step 1: Write the migration file `apps/backend/migrations/000018_jobs_owner_id.sql`**

```sql
-- +goose Up
ALTER TABLE jobs ADD COLUMN owner_id UUID REFERENCES users(id) ON DELETE CASCADE;
CREATE INDEX idx_jobs_owner_id ON jobs (owner_id) WHERE owner_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_owner_id;
ALTER TABLE jobs DROP COLUMN IF EXISTS owner_id;
```

- [ ] **Step 2: Add `OwnerID` field to `Job` struct and `CreateJobParams` in `apps/backend/internal/models/job.go`**

Add `OwnerID *uuid.UUID` to the `Job` struct (after `UpdatedByID`). Add `OwnerID *uuid.UUID` and `Status string` to `CreateJobParams`, both tagged `json:"-"`. The repository maps an empty `Status` to `"new"` so existing callers are unchanged.

- [ ] **Step 3: Add `BuildManualJobSlug` to `apps/backend/internal/models/job.go`**

```go
// BuildManualJobSlug prefixes the standard slug with "m-{first8 of userID}-"
// to namespace manual jobs away from scraped jobs and from other users.
func BuildManualJobSlug(userID, title, company, location string) string {
    prefix := "m-"
    if len(userID) >= 8 {
        prefix += userID[:8] + "-"
    } else if userID != "" {
        prefix += userID + "-"
    }
    return prefix + BuildJobSlug(title, company, location)
}
```

- [ ] **Step 4: Write tests for `BuildManualJobSlug` in `apps/backend/internal/models/job_test.go`**

Test cases:
- Normal: `BuildManualJobSlug("abcdef12-...", "Senior Engineer", "Acme", "Berlin")` → `"m-abcdef12-senior-engineer-acme-berlin"`
- Different user same job: different user UUID produces different slug
- Short userID: handled gracefully
- Empty fields: falls back to `untitled`, `unknown-company`, `unspecified-location` with prefix

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd apps/backend && go test ./internal/models/ -v -run TestBuildManualJobSlug`
Expected: PASS

- [ ] **Step 6: Add `StatusScrapePending` and `StatusScrapeFailed` to `apps/backend/internal/utils/constants/jobs_status.go`**

```go
StatusScrapePending JobStatus = "scrape_pending"
StatusScrapeFailed  JobStatus = "scrape_failed"
```

Update `ValidateJobStatusTransition` to add:
```
scrape_pending → new          (scrape succeeded, enter normal flow)
scrape_pending → scrape_failed (scrape failed)
scrape_failed  → scrape_pending (retry scrape)
```

- [ ] **Step 7: Write tests for the new status transitions in `apps/backend/internal/utils/constants/jobs_status_test.go`**

Test cases:
- `scrape_pending → new`: allowed
- `scrape_pending → scrape_failed`: allowed
- `scrape_pending → queued`: rejected
- `scrape_failed → scrape_pending`: allowed (retry)
- `scrape_failed → new`: rejected
- Existing transitions unchanged (regression): `new → queued`, `queued → analyze_pending`, `analyzed → *` rejected

- [ ] **Step 8: Run all status tests**

Run: `cd apps/backend && go test ./internal/utils/constants/ -v -run TestValidateJobStatusTransition`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add apps/backend/migrations/000018_jobs_owner_id.sql \
  apps/backend/internal/models/job.go \
  apps/backend/internal/models/job_test.go \
  apps/backend/internal/utils/constants/jobs_status.go \
  apps/backend/internal/utils/constants/jobs_status_test.go
git commit -m "feat: add owner_id column, scrape statuses, manual slug builder"
```

---

### Task 2: Repository Layer — owner_id in Create, Visibility Filter in List/GetBySlug

**Files:**
- Modify: `apps/backend/internal/repository/jobs.go`
- Test: `apps/backend/internal/repository/jobs_test.go` (create if needed)

**Interfaces:**
- Consumes: `Job.OwnerID`, `CreateJobParams.OwnerID` from Task 1
- Produces:
  - `Create` INSERT includes `status` and `owner_id` columns
  - `BulkCreate` INSERT includes `status` and `owner_id` columns
  - `GetBySlug` SQL adds `WHERE (j.owner_id IS NULL OR j.owner_id = $N)` visibility filter
  - `List` SQL adds `WHERE (owner_id IS NULL OR owner_id = $N)` visibility filter + count filter
  - `scanJob` / `scanJobWithOwnership` scan `owner_id` column

- [ ] **Step 1: Update `Create` in `apps/backend/internal/repository/jobs.go`**

Add `status` and `owner_id` columns to the INSERT statement and RETURNING clause. Pass `statusOrNew(params.Status)` and `params.OwnerID`. The status column already has a database default of `new`, but an explicit value keeps the skeleton flow clear.

- [ ] **Step 2: Update `BulkCreate` to include `status` and `owner_id`**

Each row's value placeholders grow from 14 to 16 parameters. Append `statusOrNew(j.Status)` and `j.OwnerID` to args. Add `statusOrNew(s string) string` next to `actorTypeID`, returning `"new"` for an empty string.

- [ ] **Step 3: Update `scanJob` and `scanJobWithOwnership` to scan `owner_id`**

Add `ownerID *uuid.UUID` to the scan variables. Scan it after `updated_by_id`. Assign to `j.OwnerID`.

- [ ] **Step 4: Update `GetBySlug` to add visibility filter**

Both branches (with and without userID) need the filter:
- With userID: `WHERE j.slug = $1 AND (j.owner_id IS NULL OR j.owner_id = $2)` — reorder params so userID is used for both the LEFT JOIN and the WHERE
- Without userID (API key / internal caller): keep `WHERE slug = $1` with no owner filter. Machine callers see all jobs, including private ones.

- [ ] **Step 5: Update `List` to add visibility filter**

Both count and data queries need:
- With userID: `WHERE (j.owner_id IS NULL OR j.owner_id = $1)` added to both COUNT and SELECT
- Without userID: no owner filter on either query; machine callers see all jobs

- [ ] **Step 6: Update `getBySlug` (internal, used within transactions) to include `owner_id` in its SELECT and scan**

No visibility filter here — it's called within Create/BulkCreate where the caller already owns the row.

- [ ] **Step 7: Write repository tests verifying visibility isolation**

Test cases:
- Create a job with `owner_id = user_A` → `GetBySlug` with `user_A` returns it
- Create a job with `owner_id = user_A` → `GetBySlug` with `user_B` returns `ErrNotFound`
- Create a job with `owner_id = NULL` → `GetBySlug` with any user returns it
- Create a job with `owner_id = user_A` → `GetBySlug` with an empty userID returns it (machine caller)
- `List` with `user_A` returns shared jobs + user_A's private jobs, not user_B's
- `List` with empty userID returns shared jobs + all private jobs

- [ ] **Step 8: Run repository tests**

Run: `cd apps/backend && go test ./internal/repository/ -v -run TestJobs`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add apps/backend/internal/repository/jobs.go \
  apps/backend/internal/repository/jobs_test.go
git commit -m "feat: owner_id in job CRUD, visibility filter on List/GetBySlug"
```

---

### Task 3: Usecase + Handler — Manual Job Creation Endpoint

**Files:**
- Modify: `apps/backend/internal/models/job.go`
- Modify: `apps/backend/internal/usecase/user_jobs/user_jobs.go`
- Modify: `apps/backend/internal/contract/usecase/contract.go` (inject jobs usecase)
- Create: `apps/backend/internal/deliveries/api/handler/user_jobs/manual.go`
- Modify: `apps/backend/internal/deliveries/api/handler/user_jobs/user_jobs.go` (Route registration)
- Modify: `apps/backend/internal/deliveries/api/dto/job.go` (add `source` to response)
- Test: `apps/backend/internal/usecase/user_jobs/user_jobs_test.go`

**Interfaces:**
- Consumes:
  - `CreateJobParams.OwnerID` from Task 1
  - `BuildManualJobSlug(userID, title, company, location)` from Task 1
  - Visibility-filtered `GetBySlug` from Task 2
  - `ucJobs.Create` from the existing jobs usecase (creates the `user_jobs` link and transaction)
- Produces:
  - `UserJobs.CreateManual(ctx, userID string, params models.CreateManualJobParams) (*models.CreateJobResult, error)` usecase method
  - `POST /v1/jobs/manual` handler (Clerk auth)
  - `createManualJobRequest` DTO: `title`, `company`, `location`, `description`, `source_url`, `apply_url`, `posted_date`, `is_remote`, `visa_relocation`
  - `JobResponse.Source string` added to API response DTO

- [ ] **Step 1: Add `CreateManualJobParams` to `apps/backend/internal/models/job.go`**

```go
// CreateManualJobParams holds user-supplied fields for a manually inserted job.
type CreateManualJobParams struct {
    Title          string
    Company        string
    Location       string
    Description    *string
    SourceURL      *string
    ApplyURL       *string
    PostedDate     time.Time
    IsRemote       *bool
    VisaRelocation *bool
}
```

- [ ] **Step 2: Add `CreateManual` method to `UserJobs` interface and `userJobsService` in `apps/backend/internal/usecase/user_jobs/user_jobs.go`**

The method must go through the jobs usecase, not `repo.Jobs.Create`, because that is where the `user_jobs` link and transaction are handled. Add `jobsUC ucJobs.Job` to the `userJobsService` struct; do not add a `jobsRepo` field.

```go
// CreateManual creates a job with owner_id set to the calling user, source="manual",
// generates the prefixed slug, and creates the user_jobs ownership link.
CreateManual(ctx context.Context, userID string, params models.CreateManualJobParams) (*models.CreateJobResult, error)
```

Implementation:
1. Build slug via `models.BuildManualJobSlug(userID, params.Title, params.Company, params.Location)`
2. Construct `models.CreateJobParams` with `Source: "manual"`, `OwnerID: &userUUID`, `Slug: slug`, all user fields, `UpdatedByType: "user"`, `UpdatedByID: userID`, `UserIDs: []string{userID}`
3. Call `s.jobsUC.Create(ctx, createParams)` (returns `*models.CreateJobResult`, including the `user_jobs` link side effect)
4. Return result

- [ ] **Step 3: Update `NewUserJobsService` constructor to inject the jobs usecase**

Modify the struct and constructor in `apps/backend/internal/usecase/user_jobs/user_jobs.go`:
```go
type userJobsService struct {
    repo    repo.UserJobs
    jobsUC  ucJobs.Job
}

func NewUserJobsService(r *repository.Contract, jobsUC ucJobs.Job) UserJobs {
    return &userJobsService{
        repo:   r.UserJobsRepository,
        jobsUC: jobsUC,
    }
}
```

Then update `apps/backend/internal/contract/usecase/contract.go`: `c.Jobs` must be assigned before `c.UserJobs`, so build the struct with `Jobs` first and set `c.UserJobs = ucUserJobs.NewUserJobsService(r, c.Jobs)` on the next line. Do not reference `c.Jobs` from inside the struct literal.

- [ ] **Step 4: Write the handler in `apps/backend/internal/deliveries/api/handler/user_jobs/manual.go`**

Request DTO:
```go
type createManualJobRequest struct {
    Title          string  `json:"title"`
    Company        string  `json:"company"`
    Location       string  `json:"location"`
    Description    *string `json:"description,omitempty"`
    SourceURL      *string `json:"source_url,omitempty"`
    ApplyURL       *string `json:"apply_url,omitempty"`
    PostedDate     string  `json:"posted_date,omitempty"`
    IsRemote       *bool   `json:"is_remote,omitempty"`
    VisaRelocation *bool   `json:"visa_relocation,omitempty"`
}
```

Handler `CreateManualJob(w, r) error`:
1. Extract principal from context (`ctxval.PrincipalFromContext`)
2. Resolve internal user ID via `h.userUC.ResolveOrCreate`
3. Decode and validate request (title, company, location required)
4. Parse `posted_date` if present
5. Call `h.ucJobs.CreateManual(ctx, userID, params)`
6. Return `dto.NewCreateJobResponse(result)` with 201

Swagger annotations: `@Summary Create manual job`, `@Tags user-jobs`, `@Security ApiKeyAuth`, `@Router /v1/jobs/manual [post]`

- [ ] **Step 5: Register the route in `apps/backend/internal/deliveries/api/handler/user_jobs/user_jobs.go`**

In the `Route` method, add:
```go
g.POST("/manual", h.CreateManualJob, auth.ValidClerkSession)
```

- [ ] **Step 6: Add `Source` field to `JobResponse` in `apps/backend/internal/deliveries/api/dto/job.go`**

```go
Source string `json:"source"`
```

Map `job.Source` in `NewJobResponse`.

- [ ] **Step 7: Write usecase tests for `CreateManual`**

Test cases:
- Happy path: title/company/location provided → `ucJobs.Create` called with `source=manual`, `owner_id=userUUID`, `UserIDs=[user]`, slug starts with `m-`
- Deduplication: same user, same title/company/location → `Created=false`
- Missing title: returns error before calling `ucJobs.Create`

Use the existing `internal/usecase/jobs/mocks` `MockJob` for the injected jobs usecase.

- [ ] **Step 8: Run tests**

Run: `cd apps/backend && go test ./internal/usecase/user_jobs/ -v -run TestCreateManual`
Expected: PASS

- [ ] **Step 9: Regenerate Swagger**

Run: `cd apps/backend && swag init -g cmd/api/main.go -o docs`

- [ ] **Step 10: Commit**

```bash
git add apps/backend/internal/models/job.go \
  apps/backend/internal/usecase/user_jobs/user_jobs.go \
  apps/backend/internal/usecase/user_jobs/user_jobs_test.go \
  apps/backend/internal/contract/usecase/contract.go \
  apps/backend/internal/deliveries/api/handler/user_jobs/manual.go \
  apps/backend/internal/deliveries/api/handler/user_jobs/user_jobs.go \
  apps/backend/internal/deliveries/api/dto/job.go \
  apps/backend/docs/docs.go \
  apps/backend/docs/swagger.json \
  apps/backend/docs/swagger.yaml
git commit -m "feat: POST /v1/jobs/manual endpoint for manual job insertion"
```

---

### Task 4: Usecase + Handler — URL Scrape Endpoint + Workflow Dispatch

**Files:**
- Modify: `apps/backend/internal/usecase/user_jobs/user_jobs.go`
- Create: `apps/backend/internal/deliveries/api/handler/user_jobs/scrape_url.go`
- Modify: `apps/backend/internal/deliveries/api/handler/user_jobs/user_jobs.go` (Route registration)
- Create: `apps/backend/internal/models/user_job_scrape.go`
- Modify: `apps/backend/internal/models/limiter.go` (new `LimiterJobTypeUserJobScrape`)
- Modify: `apps/backend/internal/repository/workflows.go` (add `StartUserJobScrape` method)
- Modify: `apps/backend/internal/repository/mocks/workflows_mock.go` (regenerate)
- Test: `apps/backend/internal/usecase/user_jobs/user_jobs_test.go`

**Interfaces:**
- Consumes:
  - `CreateJobParams.OwnerID`, `StatusScrapePending` from Task 1
  - `ucJobs.Create` from Task 3 (jobs usecase, handles `user_jobs` link)
  - `Workflows.StartLimiterWorkItem` from existing repository
- Produces:
  - `UserJobs.ScrapeURL(ctx, userID string, params models.ScrapeURLParams) (*models.CreateJobResult, error)` usecase method
  - `POST /v1/jobs/scrape-url` handler (Clerk auth)
  - `UserJobScrapeWorkflowInput` model: `{job_id, slug, source_url, user_id, owner_id}`
  - `Workflows.StartUserJobScrape(ctx, input UserJobScrapeWorkflowInput) (string, error)` repository method

- [ ] **Step 1: Create `apps/backend/internal/models/user_job_scrape.go`**

```go
package models

// ScrapeURLParams holds the user-supplied URL for scraping.
type ScrapeURLParams struct {
    SourceURL string
}

// UserJobScrapeWorkflowInput is the payload sent to the Python worker's
// UserJobScrapeWorkflow.
type UserJobScrapeWorkflowInput struct {
    JobID     int64  `json:"job_id"`
    Slug      string `json:"slug"`
    SourceURL string `json:"source_url"`
    UserID    string `json:"user_id"`
    OwnerID   string `json:"owner_id"`
}
```

- [ ] **Step 2: Add `StartUserJobScrape` to `Workflows` interface and implementation in `apps/backend/internal/repository/workflows.go`**

```go
StartUserJobScrape(ctx context.Context, input models.UserJobScrapeWorkflowInput) (string, error)
```

Add `LimiterJobTypeUserJobScrape LimiterJobType = "user_job_scrape"` to `apps/backend/internal/models/limiter.go`.

Implementation: a thin wrapper over the existing `StartLimiterWorkItem(ctx, "user-job-scrape-"+slug, models.LimiterJobTypeUserJobScrape, input)`. Do not call `ExecuteWorkflow` directly: the `job-limit` queue only polls `ConcurrencyLimiter`, so a workflow started there would never run. The Python side adds matching `user_job_scrape` entries to `QUEUE_BY_TYPE`, `WORKFLOW_BY_TYPE`, and the concurrency limit map in Task 5.

- [ ] **Step 3: Add `ScrapeURL` to `UserJobs` interface and implementation**

The method needs the workflows repository and the jobs usecase already injected in Task 3. Add `workflows repo.Workflows` to `userJobsService`; keep `jobsUC`, do not add `jobsRepo`.

```go
func (s *userJobsService) ScrapeURL(ctx context.Context, userID string, params models.ScrapeURLParams) (*models.CreateJobResult, error)
```

Implementation:
1. Generate a temporary slug: `fmt.Sprintf("m-%s-scrape-%d", userID[:8], time.Now().Unix())`
2. Build `CreateJobParams`: `Source="user_scrape"`, `Status=string(constants.StatusScrapePending)`, `OwnerID=userUUID`, `SourceURL=params.SourceURL`, `Title="Pending scrape"`, `Company="unknown"`, `Location="unknown"`, `UserIDs=[userID]`
3. Call `s.jobsUC.Create(ctx, createParams)` so the job is inserted directly at `scrape_pending` and the `user_jobs` link is created
4. Start `UserJobScrapeWorkflow` via `s.workflows.StartUserJobScrape(ctx, input)`, using the job ID, slug, source URL, user ID, and owner ID
5. Return the created job result

- [ ] **Step 4: Update `NewUserJobsService` to inject `r.WorkflowsRepository`**

```go
type userJobsService struct {
    repo      repo.UserJobs
    jobsUC    ucJobs.Job
    workflows repo.Workflows
}
```

- [ ] **Step 5: Write the handler in `apps/backend/internal/deliveries/api/handler/user_jobs/scrape_url.go`**

Request DTO:
```go
type scrapeURLRequest struct {
    SourceURL string `json:"source_url"`
}
```

Handler `ScrapeURL(w, r) error`:
1. Extract principal, resolve internal user ID
2. Decode and validate (source_url required, must start with `http://` or `https://`)
3. Call `h.ucJobs.ScrapeURL(ctx, userID, params)`
4. Return 202 Accepted with `dto.NewCreateJobResponse(result)`

Swagger: `@Summary Scrape job from URL`, `@Tags user-jobs`, `@Router /v1/jobs/scrape-url [post]`

- [ ] **Step 6: Register the route**

```go
g.POST("/scrape-url", h.ScrapeURL, auth.ValidClerkSession)
```

- [ ] **Step 7: Update the handler struct to include workflows usecase**

The handler's `ucJobs` field (type `ucUserJobs.UserJobs`) already carries the new methods since they're on the interface. No structural change to the handler struct needed — just ensure the `UserJobs` interface has the new methods.

- [ ] **Step 8: Write tests for `ScrapeURL` usecase**

Test cases:
- Happy path: URL provided → `ucJobs.Create` called with `status=scrape_pending`, `source=user_scrape`, `owner_id=userUUID`, then `StartLimiterWorkItem` called with job type `user_job_scrape`
- Empty URL: returns error before any repository call
- Workflow start failure: job is still created and can be retried

Mock `Workflows` with `internal/repository/mocks` `MockWorkflows`.

- [ ] **Step 9: Run tests**

Run: `cd apps/backend && go test ./internal/usecase/user_jobs/ -v -run TestScrapeURL`
Expected: PASS

- [ ] **Step 10: Regenerate Swagger**

Run: `cd apps/backend && swag init -g cmd/api/main.go -o docs`

- [ ] **Step 11: Commit**

```bash
git add apps/backend/internal/models/user_job_scrape.go \
  apps/backend/internal/models/limiter.go \
  apps/backend/internal/repository/workflows.go \
  apps/backend/internal/repository/mocks/workflows_mock.go \
  apps/backend/internal/usecase/user_jobs/user_jobs.go \
  apps/backend/internal/usecase/user_jobs/user_jobs_test.go \
  apps/backend/internal/deliveries/api/handler/user_jobs/scrape_url.go \
  apps/backend/internal/deliveries/api/handler/user_jobs/user_jobs.go \
  apps/backend/docs/docs.go \
  apps/backend/docs/swagger.json \
  apps/backend/docs/swagger.yaml
git commit -m "feat: POST /v1/jobs/scrape-url endpoint with Temporal workflow dispatch"
```

---

### Task 5: Python Worker — UserJobScrapeWorkflow + LLM Extraction Activity

**Files:**
- Create: `workers/python/workflows/user_job_scrape.py`
- Create: `workers/python/deliveries/consumer/activities/user_job_scrape.py`
- Modify: `workers/python/repository/getojobu_api.py` (extend `patch_job_status`)
- Modify: `workers/python/workflows/concurrency_limiter.py` (new job type mappings)
- Modify: `workers/python/worker/main.py` (register workflow + activities on `job-scrape`)
- Test: `workers/python/tests/test_user_job_scrape.py`
- Test: `workers/python/tests/test_concurrency_limiter.py`

**Interfaces:**
- Consumes:
  - `UserJobScrapeWorkflowInput` JSON: `{job_id, slug, source_url, user_id, owner_id}`
  - Go API `PATCH /v1/jobs/{slug}` to update status + fields after scrape
  - Go API `POST /v1/jobs/bulk-queue-analyze` to queue analysis after scrape succeeds
  - `repository.getojobu_api.GetojobuAPI` for HTTP calls to the Go API
- Produces:
  - `UserJobScrapeWorkflow` Temporal workflow (registered name `"UserJobScrapeWorkflow"`)
  - `fetch_and_extract_job` activity: fetches URL, calls LLM, returns structured job data
  - `update_scraped_job` activity: PATCHes the job with extracted data, transitions status to `new`
  - `queue_scraped_job_analysis` activity: queues analysis for the scraped job

- [ ] **Step 1: Create the extraction activity in `workers/python/deliveries/consumer/activities/user_job_scrape.py`**

Define container init pattern matching existing activities (e.g. `init_job_scrape_container`).

Activity `fetch_and_extract_job(input: dict) -> dict`:
1. Fetch page content from `input["source_url"]` using `httpx` (with timeout, user-agent)
2. Extract text content (strip HTML tags, limit to reasonable length)
3. Call the existing `extract_job_analysis` LLM client with the `extract-job-v1` prompt, or a dedicated extraction prompt following the same pattern, to extract: `title`, `company`, `location`, `description`, `apply_url`, `posted_date`, `is_remote`, `visa_relocation`
4. Return structured dict with extracted fields

Reject non-http(s) URLs and private/link-local hosts before fetching. Treat page content as untrusted input and pass it through the existing prompt sanitizer.

Activity `update_scraped_job(input: dict) -> dict`:
1. Build updated slug via `m-{owner_id[:8]}-{build_job_slug(title, company, location)}`. Fix the Python helper to use Go's `untitled` fallback and Unicode-aware normalization so it matches `BuildManualJobSlug`.
2. Call Go API to update the job: `PATCH /v1/jobs/{old_slug}` with extracted fields + status transition `scrape_pending → new`. Extend `GetojobuAPI.patch_job_status` in `workers/python/repository/getojobu_api.py` to accept these optional fields, or add a dedicated `update_scraped_job` method there.
3. Return updated job info

Activity `queue_scraped_job_analysis(input: dict) -> dict`:
1. Call `POST /v1/jobs/bulk-queue-analyze` with the job slug
2. Return workflow IDs

- [ ] **Step 2: Create the workflow in `workers/python/workflows/user_job_scrape.py`**

```python
@workflow.defn(name="UserJobScrapeWorkflow")
class UserJobScrapeWorkflow:
    @workflow.run
    async def run(self, input: dict[str, Any]) -> dict[str, Any]:
        # 1. fetch_and_extract_job
        # 2. update_scraped_job (transitions scrape_pending → new)
        # 3. queue_scraped_job_analysis
```

Retry policy: 2 attempts for fetch, 3 for API calls. Timeouts: 60s for fetch+LLM, 30s for API calls.

On failure: catch exceptions, call `PATCH /v1/jobs/{slug}` with `status=scrape_failed` and `failure_message`.

- [ ] **Step 3: Register in `workers/python/worker/main.py`**

Add `user_job_scrape` entries to `QUEUE_BY_TYPE` (`job-scrape`), `WORKFLOW_BY_TYPE` (`UserJobScrapeWorkflow`), and the concurrency limit map in `workers/python/workflows/concurrency_limiter.py`. Register `UserJobScrapeWorkflow` and the three activities on the `job-scrape` worker in `worker/main.py`, and add `init_user_job_scrape_container` to the startup sequence. Do not register them on `job-limit`; that worker only polls `ConcurrencyLimiter`.

- [ ] **Step 4: Write tests in `workers/python/tests/test_user_job_scrape.py`**

Test cases:
- `fetch_and_extract_job`: mock HTTP response with HTML job page → returns structured fields
- `fetch_and_extract_job`: HTTP error → raises ActivityError
- `update_scraped_job`: mock API call → verifies PATCH payload and slug regeneration
- Limiter mapping: `user_job_scrape` resolves to `job-scrape` and `UserJobScrapeWorkflow` in `test_concurrency_limiter.py`
- Workflow integration: mock activities → verify sequence and failure handling

- [ ] **Step 5: Run tests**

Run: `cd workers/python && python -m pytest tests/test_user_job_scrape.py -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add workers/python/workflows/user_job_scrape.py \
  workers/python/deliveries/consumer/activities/user_job_scrape.py \
  workers/python/repository/getojobu_api.py \
  workers/python/workflows/concurrency_limiter.py \
  workers/python/worker/main.py \
  workers/python/tests/test_user_job_scrape.py \
  workers/python/tests/test_concurrency_limiter.py
git commit -m "feat: UserJobScrapeWorkflow — LLM-powered URL extraction in Python worker"
```

---

### Task 6: Go Backend — Support Job Update with New Fields (for scrape callback)

**Files:**
- Modify: `apps/backend/internal/models/job.go` (`UpdateJobParams`)
- Modify: `apps/backend/internal/repository/jobs.go` (`UpdateStatus` accepts params and persists extended fields)
- Modify: `apps/backend/internal/deliveries/api/handler/jobs/request.go` (extend `updateJobRequest`)
- Modify: `apps/backend/internal/deliveries/api/handler/jobs/jobs.go` (build `UpdateJobParams`)
- Modify: `apps/backend/internal/usecase/jobs/jobs.go` (`UpdateStatus` signature)
- Modify: `apps/backend/internal/usecase/jobs/analyze.go` (call sites)
- Modify: `apps/backend/internal/usecase/jobs/mocks/job_mock.go` (regenerate)
- Modify: `apps/backend/internal/repository/mocks/jobs_mock.go` (regenerate)
- Test: `apps/backend/internal/repository/jobs_test.go`
- Test: `apps/backend/internal/usecase/jobs/jobs_test.go`
- Test: `apps/backend/internal/usecase/jobs/analyze_test.go`
- Test: `apps/backend/internal/deliveries/api/handler/jobs/jobs_test.go`

**Interfaces:**
- Consumes:
  - `ValidateJobStatusTransition` with scrape statuses from Task 1
  - `UpdateStatus` repository method
- Produces:
  - Extended `UpdateStatus` that can also update `title`, `company`, `location`, `description`, `source_url`, `apply_url`, `posted_date`, `is_remote`, `visa_relocation`, and `slug` when transitioning from `scrape_pending → new`
  - Extended `updateJobRequest` with optional fields for the scrape callback

- [ ] **Step 1: Extend `updateJobRequest` in `apps/backend/internal/deliveries/api/handler/jobs/request.go`**

Add optional fields that the scrape worker can send when updating a scraped job:
```go
Title          *string `json:"title,omitempty"`
Company        *string `json:"company,omitempty"`
Location       *string `json:"location,omitempty"`
Description    *string `json:"description,omitempty"`
SourceURL      *string `json:"source_url,omitempty"`
ApplyURL       *string `json:"apply_url,omitempty"`
PostedDate     *string `json:"posted_date,omitempty"`
IsRemote       *bool   `json:"is_remote,omitempty"`
VisaRelocation *bool   `json:"visa_relocation,omitempty"`
Slug           *string `json:"slug,omitempty"`
```

- [ ] **Step 2: Create `models.UpdateJobParams` to carry the extended update fields**

```go
type UpdateJobParams struct {
    Slug           string
    Status         string
    AnalyzeResult  *string
    FailureMessage *string
    // Extended fields for scrape callback
    NewSlug        *string
    Title          *string
    Company        *string
    Location       *string
    Description    *string
    SourceURL      *string
    ApplyURL       *string
    PostedDate     *time.Time
    IsRemote       *bool
    VisaRelocation *bool
    UpdatedByType  string
    UpdatedByID    string
}
```

- [ ] **Step 3: Update `UpdateStatus` in the repository to accept and persist the extended fields**

Build a dynamic UPDATE SET clause that includes any non-nil extended fields. Use parameterized SQL with conditional SET columns. Accept `models.UpdateJobParams` in place of the current seven arguments.

- [ ] **Step 4: Update the usecase `UpdateStatus` signature everywhere**

Change the `UpdateStatus` method on both the `Job` usecase interface and the `Jobs` repository interface to take `models.UpdateJobParams`. Update the implementation in `apps/backend/internal/usecase/jobs/jobs.go` to keep the existing transition validation and failure-message concatenation, then pass the extended fields to the repository only when the current status is `scrape_pending`. Update the four call sites in `apps/backend/internal/usecase/jobs/analyze.go` and the PATCH handler call. Regenerate `internal/usecase/jobs/mocks/job_mock.go` and `internal/repository/mocks/jobs_mock.go`, then fix all gomock expectations in `jobs_test.go`, `analyze_test.go`, and the handler tests. There is no second update method.

- [ ] **Step 5: Update the handler to build `UpdateJobParams`**

In the PATCH handler, decode the new optional fields from the request body, parse `posted_date`, and build one `models.UpdateJobParams` for the single `UpdateStatus` call. No branching to a separate method.

- [ ] **Step 6: Write tests for extended update**

Test cases:
- `scrape_pending → new` with title/company/location/slug: all fields updated
- `scrape_pending → new` without extended fields: only status changes
- `queued → analyze_pending` with extended fields: extended fields ignored (only valid for scrape transitions)
- Slug update changes the row's slug column

- [ ] **Step 7: Run tests**

Run: `cd apps/backend && go test ./internal/repository/ ./internal/usecase/jobs/ -v -run TestUpdateStatus`
Expected: PASS

- [ ] **Step 8: Regenerate Swagger**

Run: `cd apps/backend && swag init -g cmd/api/main.go -o docs`

- [ ] **Step 9: Commit**

```bash
git add apps/backend/internal/models/job.go \
  apps/backend/internal/repository/jobs.go \
  apps/backend/internal/repository/mocks/jobs_mock.go \
  apps/backend/internal/deliveries/api/handler/jobs/request.go \
  apps/backend/internal/deliveries/api/handler/jobs/jobs.go \
  apps/backend/internal/deliveries/api/handler/jobs/jobs_test.go \
  apps/backend/internal/usecase/jobs/jobs.go \
  apps/backend/internal/usecase/jobs/analyze.go \
  apps/backend/internal/usecase/jobs/mocks/job_mock.go \
  apps/backend/internal/usecase/jobs/jobs_test.go \
  apps/backend/internal/usecase/jobs/analyze_test.go \
  apps/backend/internal/repository/jobs_test.go \
  apps/backend/docs/docs.go \
  apps/backend/docs/swagger.json \
  apps/backend/docs/swagger.yaml
git commit -m "feat: extend PATCH /v1/jobs/{slug} to accept scraped fields on scrape_pending→new"
```

---

### Task 7: Frontend — Add Job Form (Manual + URL Scrape)

**Files:**
- Create: `apps/web/app/(app)/user-jobs/add/page.tsx`
- Create: `apps/web/components/jobs/add-job-form.tsx`
- Create: `apps/web/components/jobs/scrape-url-form.tsx`
- Modify: `apps/web/lib/api/endpoints.ts` (add API functions)
- Modify: `apps/web/lib/api/types.ts` (add request types)
- Modify: `apps/web/app/(app)/user-jobs/page.tsx` (add "Add Job" button)

**Interfaces:**
- Consumes:
  - `POST /v1/jobs/manual` from Task 3
  - `POST /v1/jobs/scrape-url` from Task 4
  - `CreateJobResponse` DTO shape
- Produces:
  - `/user-jobs/add` page with tabbed form (Manual / Scrape URL)
  - `createManualJob(params)` API function
  - `scrapeJobFromURL(url)` API function
  - "Add Job" navigation button on My Jobs page

- [ ] **Step 1: Add TypeScript types in `apps/web/lib/api/types.ts`**

```typescript
export interface CreateManualJobRequest {
  title: string;
  company: string;
  location: string;
  description?: string;
  source_url?: string;
  apply_url?: string;
  posted_date?: string;
  is_remote?: boolean;
  visa_relocation?: boolean;
}

export interface ScrapeURLRequest {
  source_url: string;
}
```

- [ ] **Step 2: Add API functions in `apps/web/lib/api/endpoints.ts`**

```typescript
export async function createManualJob(params: CreateManualJobRequest): Promise<CreateJobResponse>
export async function scrapeJobFromURL(params: ScrapeURLRequest): Promise<CreateJobResponse>
```

Follow existing patterns for auth headers and error handling.

- [ ] **Step 3: Create the manual job form component `apps/web/components/jobs/add-job-form.tsx`**

Form fields: title (required), company (required), location (required), description (textarea), apply URL, source URL, posted date, is_remote checkbox, visa_relocation checkbox.
On submit: call `createManualJob`, redirect to the job detail page on success.
Show validation errors inline. Show loading state during submission.

- [ ] **Step 4: Create the URL scrape form component `apps/web/components/jobs/scrape-url-form.tsx`**

Single URL input field + "Scrape" button.
On submit: call `scrapeJobFromURL`, show success message with link to My Jobs (since the job will be `scrape_pending`).
Validate URL format before submission.

- [ ] **Step 5: Create the add job page `apps/web/app/(app)/user-jobs/add/page.tsx`**

Tabbed layout with two tabs: "Manual Entry" and "Scrape from URL".
Each tab renders its respective form component.

- [ ] **Step 6: Add "Add Job" button to `apps/web/app/(app)/user-jobs/page.tsx`**

Add a button/link in the page header area that navigates to `/user-jobs/add`.

- [ ] **Step 7: Verify types compile**

Run: `cd apps/web && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 8: Commit**

```bash
git add apps/web/app/\(app\)/user-jobs/add/page.tsx \
  apps/web/components/jobs/add-job-form.tsx \
  apps/web/components/jobs/scrape-url-form.tsx \
  apps/web/lib/api/endpoints.ts \
  apps/web/lib/api/types.ts \
  apps/web/app/\(app\)/user-jobs/page.tsx
git commit -m "feat: add job page with manual entry and URL scrape forms"
```

---

### Task 8: Frontend — Display Source Badge + Scrape Status on My Jobs

**Files:**
- Modify: `apps/web/lib/api/types.ts` (`Job.source`, `Job.failure_message`, `Job.visa_relocation`)
- Modify: `apps/web/components/jobs/job-card.tsx` (source badge + scrape status)
- Modify: `apps/web/components/jobs/job-detail-drawer.tsx` (source info)
- Modify: `apps/web/app/(app)/user-jobs/page.tsx` (retry wiring, if needed)

**Interfaces:**
- Consumes:
  - `JobResponse.source` field (added in Task 3 Step 6)
  - `JobResponse.status` for `scrape_pending` / `scrape_failed` states
- Produces:
  - Source badge on job cards: "Manual", "Scraped" (for user_scrape), or board name
  - Status indicator for `scrape_pending` (spinner/pending badge) and `scrape_failed` (error badge with retry option)

- [ ] **Step 1: Add source badge to the job card in the My Jobs list**

Add `source?: string`, `failure_message?: string`, and `visa_relocation?: boolean` to the `Job` interface in `apps/web/lib/api/types.ts`. `UserJobResponse.Job` is a `JobResponse`, so the field reaches the My Jobs list without changing the user-job DTO.

Display a small badge next to the job title in `JobCard`:
- `source === "manual"` → "Manual" badge (e.g., blue)
- `source === "user_scrape"` → "Scraped" badge (e.g., purple)
- Other sources → no badge (existing scraped jobs don't need it)

- [ ] **Step 2: Add scrape status indicators**

When `status === "scrape_pending"`: show a loading spinner or "Scraping..." text, disable click-through to detail.
When `status === "scrape_failed"`: show error indicator with the failure message, offer a "Retry" button that calls `POST /v1/jobs/scrape-url` with the same URL.

- [ ] **Step 3: Update the job detail drawer to show source info**

The My Jobs page renders `JobDetailDrawer` (`components/jobs/job-detail-drawer.tsx`), not `app/(app)/user-jobs/[slug]/page.tsx`. Show the source and source_url there when available.

- [ ] **Step 4: Verify types compile**

Run: `cd apps/web && npx tsc --noEmit`
Expected: no type errors

- [ ] **Step 5: Commit**

```bash
git add apps/web/lib/api/types.ts \
  apps/web/components/jobs/job-card.tsx \
  apps/web/components/jobs/job-detail-drawer.tsx \
  apps/web/app/\(app\)/user-jobs/page.tsx
git commit -m "feat: source badge and scrape status indicators on My Jobs page"
```

---

### Task 9: Integration Testing + Documentation

**Files:**
- Modify: `docs/database/jobs-data-model.md` (document owner_id, manual sources, scrape statuses)
- Create: `docs/architecture/manual-job-insertion.md` (feature architecture doc)

**Interfaces:**
- Consumes: all prior tasks
- Produces:
  - Updated data model documentation
  - Architecture decision record for manual job insertion
  - Verified end-to-end flow

- [ ] **Step 1: Run the full Go test suite**

Run: `cd apps/backend && go test ./... -v`
Expected: all tests pass, no regressions

- [ ] **Step 2: Run the Python worker tests**

Run: `cd workers/python && python -m pytest -v`
Expected: all tests pass

- [ ] **Step 3: Run frontend type check**

Run: `cd apps/web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 4: Update `docs/database/jobs-data-model.md`**

Document:
- `owner_id` column: nullable UUID FK to users, NULL = shared, non-null = private
- New sources: `manual`, `user_scrape`
- New statuses: `scrape_pending`, `scrape_failed` with their transition rules
- Slug namespacing: `m-{user8}-{title}-{company}-{location}`
- Visibility rule for Clerk callers only: `WHERE owner_id IS NULL OR owner_id = :current_user_id`
- API-key and internal callers see all jobs; the Python worker depends on this for status updates and analysis queueing

- [ ] **Step 5: Create `docs/architecture/manual-job-insertion.md`**

Document the feature architecture:
- Motivation and goals
- Design decisions (same table, nullable owner_id, slug prefix)
- API endpoints added
- Workflow: manual vs URL scrape, dispatched through the existing concurrency limiter to the `job-scrape` queue
- Visibility model (Clerk filter vs API-key full access)
- Status state machine diagram (including scrape statuses)

- [ ] **Step 6: Confirm migration deployment step**

Migration 000018 is applied separately from every API replica. Record the command and target environment in the deployment notes; do not run it against production without an approved backup and a staging test.

- [ ] **Step 7: Commit**

```bash
git add docs/database/jobs-data-model.md \
  docs/architecture/manual-job-insertion.md
git commit -m "docs: manual job insertion architecture and data model updates"
```
