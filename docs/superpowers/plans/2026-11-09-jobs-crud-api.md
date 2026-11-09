# Jobs CRUD API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Go backend API for the `jobs` table per `docs/database/jobs-data-model.md`: single and bulk insert (first-source-wins via `ON CONFLICT (slug) DO NOTHING`), read by slug, list recent, search by company, and a restricted description update. A scrape cron authenticates with a static API key; user routes use Clerk sessions. New columns `status` (VARCHAR), `analyze_result` (JSONB), `failure_message` (TEXT), `updated_by_type` (TEXT), and `updated_by_id` (TEXT) track Temporal analysis state and audit attribution with lifecycle `new` → `analyze_pending` → `analyzed` / `failed` (failed can return to `analyze_pending` for retry). API key → identity mapping is server-side (no client-supplied user identity).

**Architecture:** Follow existing backend conventions — migration → model → repository interface+impl → usecase interface+service → HTTP handler → route registration in `handler.go` → contract wiring in repository/usecase contracts. Machine (cron) auth via `Authorization: Bearer` API key middleware; user auth via existing `ValidClerkSession`.

**Tech Stack:** Go 1.27.3, `net/http` + `httpx.Router` (method-aware ServeMux), `pgx`/`pgxpool`, PostgreSQL (Neon), Goose-style numbered SQL migrations, `log/slog`, `testify`.

**Spec:** `docs/database/jobs-data-model.md` (jobs table schema + ingestion rules). Decisions captured in plan: added `status`/`analyze_result`/`failure_message` columns; `PATCH /v1/jobs/{slug}` updates only `description` when description IS NULL OR `status != 'analyzed'`; bulk insert is per-row with a 100-row payload cap.

## Global Constraints

- Go 1.27.3 toolchain baseline (from `apps/backend/go.mod`).
- Standard library `net/http` + method-aware `http.ServeMux` via `httpx.Router`. No Chi/Echo/Gin/Fiber.
- `Request.PathValue()` for path parameters (existing `httpx.ExtractPathParam`).
- Handler signature: `func(http.ResponseWriter, *http.Request) error` (`httpx.HandlerFunc`).
- Error envelope: `httpx.Error` with codes `internal`/`invalid_request`/`not_found`/`conflict`/`unauthorized`/`forbidden`; `httpx.RespondJSON` for success.
- Clerk auth on user routes via `middleware.AuthMiddleware.ValidClerkSession`; principal via `ctxval.PrincipalFromContext`.
- Machine auth via static API key in `Authorization: Bearer <key>` header. Keys loaded from env vars (`CRON_API_KEY`, `HERMES_API_KEY`, `PLAYWRIGHT_API_KEY`, …), each mapped server-side to an `ActorType`/`ActorID`. The API key determines the caller's identity — client headers like `X-User-Name` are never trusted for identity.
- Migration numbering: next is `000003_jobs` (after `000001_users`, `000002_user_experiences_projects`). Table mirrors `users` migration style: plain columns with `DEFAULT NOW()` timestamps, no trigger, no extra indexes.
- `status` is `VARCHAR(32) NOT NULL DEFAULT 'new'` with values `new` / `analyze_pending` / `analyzed` / `failed`. Lifecycle: `new` (just inserted) → `analyze_pending` (claimed by Temporal worker, locked from others) → `analyzed` (success) or `failed` (failure; `failed` can return to `analyze_pending` for retry).
- `analyze_result` is `JSONB`; `failure_message` is `TEXT`.
- `updated_by_type` is `TEXT NOT NULL DEFAULT 'system'`; `updated_by_id` is `TEXT NOT NULL DEFAULT 'system'`. Set from the authenticated principal (Clerk user or API key mapping) on every INSERT/UPDATE.
- Package structure follows existing conventions: `models/` (structs + params), `repository/` (data access), `usecase/` (business logic), `handler/` (HTTP delivery), `middleware/` (auth), `contract/` (dependency wiring).
- Naming conventions: interface names are nouns (`Jobs`, `Job`), concrete structs are lowercase-prefixed (`jobsRepository`, `jobService`), constructors are `NewXxx`, handler methods are Pascal-case (`CreateJob`, `GetJob`), route groups under `/v1/jobs`.
- Swaggo: handler methods carry `@Summary`/`@Description`/`@Tags` annotations; `docs/docs.go` is updated with the new `/v1/jobs` routes.
- Bulk insert: per-row `ON CONFLICT (slug) DO NOTHING`, max 100 rows per request.
- Update: only `description` mutable; allowed only when description IS NULL OR `status != 'analyzed'`; identity fields (`slug`, `title`, `company`, `location`, `source`, `external_id`, URLs) immutable.
- No code changes to `docs/database/jobs-data-model.md` itself; this plan is the implementation record of its schema plus the agreed additions.

## Review Focus

1. **Slug conflict on single insert** — `ON CONFLICT (slug) DO NOTHING` must not error; caller must be able to distinguish inserted vs. ignored (return `created` boolean or existing row).
2. **Bulk insert with mixed conflicts** — per-row semantics: non-conflicting rows insert, conflicting rows are skipped, no rollback of the whole batch; response reports per-row outcome.
3. **PATCH blocked when `description` non-null AND `status = 'analyzed'`** — must return 400/409, not silently skip.
4. **Missing/invalid API key** — 401 unauthorized. The key is validated server-side against the key→principal map; client-supplied `X-User-Name` or similar headers are never trusted for identity.
5. **Status lifecycle integrity** — insert defaults to `new`; `analyze_pending` means claimed by a worker (other workers must skip it); `analyzed` blocks further description updates; `failed` allows return to `analyze_pending` for retry. The API only reads/writes `status` via the PATCH rule; worker-driven transitions (`new`→`analyze_pending`→`analyzed`/`failed`) are Temporal worker responsibility, out of scope for this API plan.
6. **Audit attribution** — every INSERT/UPDATE sets `updated_by_type`/`updated_by_id` from the authenticated principal. Clerk users map to `type=user`, `id=<clerk_user_id>`; API keys map to their configured type/id.
7. **Slug normalization** — `BuildJobSlug(title, company, location string) string` in `usecase/jobs` applies the `docs/database/jobs-data-model.md` normalization (trim, kebab-case, `unknown-company` / `unspecified-location` placeholders for missing values) so slug is never empty. API validates normalized slug is non-empty.

---

### Task 1: Migration — create `jobs` table

**Files:**
- Create: `apps/backend/migrations/000003_jobs.up.sql`
- Create: `apps/backend/migrations/000003_jobs.down.sql`

**Interfaces:**
- None (pure SQL). Later tasks consume the `jobs` table and its columns.

- [ ] **Step 1: Write `000003_jobs.up.sql`**

```sql
-- +goose Up
CREATE TABLE jobs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,

    slug TEXT NOT NULL UNIQUE,
    status VARCHAR(32) NOT NULL DEFAULT 'new',
    analyze_result JSONB,
    failure_message TEXT,

    source TEXT NOT NULL,
    external_id TEXT,
    source_url TEXT,
    apply_url TEXT,

    title TEXT NOT NULL,
    company TEXT NOT NULL,
    location TEXT NOT NULL,
    description TEXT,
    posted_date DATE,
    is_remote BOOLEAN,
    visa_relocation BOOLEAN,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by_type TEXT NOT NULL DEFAULT 'system',
    updated_by_id TEXT NOT NULL DEFAULT 'system'
);
```

- [ ] **Step 2: Write `000003_jobs.down.sql`**

```sql
-- +goose Down
DROP TABLE IF EXISTS jobs;
```

- [ ] **Step 3: Verify migration applies against an empty test database**

Run: `go test ./...` (backend package compiles) and, if a local Postgres is available, run the up migration then the down migration and confirm both succeed. If no local Postgres, validate SQL syntax with `psql --dry-run` equivalent or inspect by eye against `000002_user_experiences_projects.up.sql` conventions.

- [ ] **Step 4: Commit**

```bash
git add apps/backend/migrations/000003_jobs.up.sql apps/backend/migrations/000003_jobs.down.sql
git commit -m "feat(db): add jobs table with status and analysis columns"
```

---

### Task 2: Config — add machine API key mapping

**Files:**
- Modify: `apps/backend/internal/config/config.go`

**Interfaces:**
- Consumes: `config.Config` struct (existing).
- Produces: `cfg.MachineAuth.APIKeys map[string]MachineKey` — map of API key string to `MachineKey{ActorType, ActorID}`.

- [ ] **Step 1: Add `MachineAuth` config and `MachineKey` type**

```go
type MachineKey struct {
    ActorType string
    ActorID   string
}
type MachineAuth struct {
    APIKeys map[string]MachineKey
}
```

Add `MachineAuth` field to `Config`. Match existing `ClerkConfig` style (plain struct fields populated from env in `Load()`).

- [ ] **Step 2: Populate `cfg.MachineAuth.APIKeys` in `Load()`**

Read named env vars and build the map. Example:
- `CRON_API_KEY` → `{ActorType: "cron", ActorID: "cron-scrape"}`
- `HERMES_API_KEY` → `{ActorType: "agent", ActorID: "hermes"}`
- `PLAYWRIGHT_API_KEY` → `{ActorType: "worker", ActorID: "playwright-worker"}`

Each is optional; unset keys are simply not added to the map. Empty map means no machine auth (only Clerk).

- [ ] **Step 3: Run `go build ./...` and `go test ./internal/config/...`**

Run: `go build ./... && go test ./internal/config/...` Expected: build passes; existing config tests still pass.

- [ ] **Step 4: Commit**

```bash
git add apps/backend/internal/config/config.go
git commit -m "feat(config): add machine API key to principal mapping"
```

---

### Task 3: API key middleware for cron/machine routes

**Files:**
- Create: `apps/backend/internal/deliveries/api/middleware/api_key.go`
- Modify: `apps/backend/internal/models/principal.go` (add `ActorType`, `ActorID`, `AuditActor()`)

**Interfaces:**
- Consumes: `*config.Config` (to read `MachineAuth.APIKeys`) and `ucUsers.User` (for Clerk principal resolution).
- Produces: `ClerkOrAPIKeyMiddleware` interface with `Require(next http.Handler) http.Handler`; constructor `NewClerkOrAPIKeyMiddleware(cfg *config.Config, userUC ucUsers.User) ClerkOrAPIKeyMiddleware`. Produces `Principal.ActorType`/`Principal.ActorID` fields and `Principal.AuditActor() (actorType, actorID string)` helper.

- [ ] **Step 1: Write `api_key.go` with `ClerkOrAPIKeyMiddleware`**

Logic:
1. Extract `Authorization: Bearer <token>` header.
2. Fast-path machine key first: check if `token` matches a configured key in `cfg.MachineAuth.APIKeys` using `crypto/sha256` + `subtle.ConstantTimeCompare`. If found, build machine principal `{ActorType: key.ActorType, ActorID: key.ActorID}`, set in context, proceed. This avoids expensive Clerk JWT parsing on every cron/worker request.
3. If no machine key match, try Clerk JWT verification (reusing `jwt.Verify` pattern from `clerk_auth.go`). If valid, resolve to local user via `userUC.FindByClerkUserID`, set principal via `ctxval.ContextWithPrincipal`, proceed.
4. Otherwise return `httpx.Envelope(httpx.CodeUnauthorized, "unauthorized", "")`.

Client headers like `X-User-Name` are never read for identity.

- [ ] **Step 2: Modify `models/principal.go` for machine actors**

Add `ActorType string` and `ActorID string` fields to `Principal`. Add `AuditActor() (actorType, actorID string)` helper that returns `("user", p.ID)` for Clerk users and `(p.ActorType, p.ActorID)` for machine actors. This lets handlers in Task 7 extract `UpdatedByType`/`UpdatedByID` without `if/else` branching.

- [ ] **Step 3: Write unit tests for `ClerkOrAPIKeyMiddleware`**

Test cases: valid Clerk token → proceeds with user principal; valid API key → proceeds with machine principal (type/id from map); invalid/missing token → 401; wrong API key → 401; empty key map → 401 for any key.

Run: `go test ./internal/deliveries/api/middleware/... -run TestClerkOrAPIKey -v` Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add apps/backend/internal/deliveries/api/middleware/api_key.go apps/backend/internal/deliveries/api/middleware/api_key_test.go apps/backend/internal/models/principal.go
git commit -m "feat(middleware): add Clerk-or-API-key auth with server-side key-to-identity mapping"
```

---

### Task 4: Job models and params

**Files:**
- Create: `apps/backend/internal/models/job.go`

**Interfaces:**
- Consumes: none new.
- Produces: `Job` struct, `CreateJobParams`, `CreateJobResult` (with `Created bool`), `BulkCreateJobParams` / `BulkCreateJobResult`, `UpdateJobParams`, `GetJobParams`, `ListJobsParams`, `SearchJobsParams`. `Job` includes `UpdatedByType` and `UpdatedByID` string fields for audit attribution. Also produces `BuildJobSlug(title, company, location string) string` helper.

- [ ] **Step 1: Write `job.go` model and request/response param structs**

Follow `models/project.go` conventions: plain structs, `time.Time` for timestamps, `*string` for nullable string fields, `sql.NullString`/`sql.NullTime` handled in repository layer. Include JSON tags for API serialization.

- [ ] **Step 2: Run `go build ./...`**

Run: `go build ./...` Expected: compiles.

- [ ] **Step 3: Commit**

```bash
git add apps/backend/internal/models/job.go
git commit -m "feat(models): add job model and params"
```

---

### Task 5: Job repository

**Files:**
- Create: `apps/backend/internal/repository/jobs.go`
- Modify: `apps/backend/internal/contract/repository/contract.go` (add `JobsRepository` field + constructor wiring)

**Interfaces:**
- Consumes: `database.DB` (existing), `models.CreateJobParams`, etc.
- Produces: `Jobs` interface: `Create(ctx, params) (*models.Job, error)` (returns `Created` flag via result), `BulkCreate(ctx, params) ([]*models.BulkCreateJobResult, error)`, `GetBySlug(ctx, slug) (*models.Job, error)`, `ListRecent(ctx, limit) ([]*models.Job, error)`, `SearchByCompany(ctx, company, limit) ([]*models.Job, error)`, `UpdateDescription(ctx, slug, description, updatedByType, updatedByID) (*models.Job, error)`.
- Produces: concrete `jobsRepository` + `NewJobsRepository(db) Jobs`.

- [ ] **Step 1: Write `repository/jobs.go`**

Implement each method with parameterized pgx queries. `Create` uses `ON CONFLICT (slug) DO NOTHING RETURNING ...` and detects whether a row was returned (`sql.ErrNoRows` means ignored; fetch existing row to return it with `Created=false`). `BulkCreate` uses `pgx.Batch` to send all `INSERT ... ON CONFLICT (slug) DO NOTHING RETURNING ...` statements in a single network round-trip inside one transaction, collects per-row results as `{Slug string, Created bool}` per item. `UpdateDescription` uses `UPDATE ... SET description = $3, updated_by_type = $4, updated_by_id = $5, updated_at = NOW() WHERE slug = $1 AND (description IS NULL OR status <> 'analyzed') RETURNING ...` and distinguishes `sql.ErrNoRows` (row exists but guard blocked → caller handles as 409) from actual `not found` (repository returns `sql.ErrNoRows` only when no row matches slug at all).

**Important:** `UpdateDescription` must NOT combine slug-existence and state-guard into one ambiguous `ErrNoRows`. Instead, repository returns `sql.ErrNoRows` only when the slug does not exist at all. The usecase (Task 6) checks `GetBySlug` first to distinguish 404 vs 409.

- [ ] **Step 2: Wire `JobsRepository` into `contract/repository/contract.go`**

Add field `JobsRepository repo.Jobs` and `repo.NewJobsRepository(comm.DB)` in `NewRepository`.

- [ ] **Step 3: Write repository tests against a test database**

Tests: insert new job returns created=true; duplicate slug returns created=false with existing row; bulk insert with one conflict returns `{Slug, Created}` per item (inserted=true, ignored=false); get by slug; list recent ordering; search by company ILIKE; update description allowed when null; update description blocked when non-null + status=analyzed.

Run: `go test ./internal/repository/... -run TestJobs -v` Expected: PASS (needs test DB; if unavailable, mark as integration test to run in CI).

- [ ] **Step 4: Commit**

```bash
git add apps/backend/internal/repository/jobs.go apps/backend/internal/repository/jobs_test.go apps/backend/internal/contract/repository/contract.go
git commit -m "feat(repository): add jobs data access layer"
```

---

### Task 6: Job usecase

**Files:**
- Create: `apps/backend/internal/usecase/jobs/jobs.go`
- Create: `apps/backend/internal/usecase/jobs/jobs_test.go`
- Modify: `apps/backend/internal/contract/usecase/contract.go` (add `Jobs` field + constructor wiring)

**Interfaces:**
- Consumes: `repository.Jobs` (from Task 5).
- Produces: `Job` interface: `Create(ctx, params)`, `BulkCreate(ctx, params)`, `GetBySlug(ctx, slug)`, `ListRecent(ctx, limit)`, `SearchByCompany(ctx, company, limit)`, `UpdateDescription(ctx, slug, description, updatedByType, updatedByID)`.
- Produces: concrete `jobService` + `NewJobService(repo) Job`.

- [ ] **Step 1: Write `usecase/jobs/jobs.go`**

Validation: `CreateJobParams.Slug` is normalized via `BuildJobSlug(title, company, location)` — if `Slug` is empty it is generated from title/company/location with `unknown-company`/`unspecified-location` placeholders; result must be non-blank. `UpdateDescription` validates description non-empty; business rule (description null OR status != analyzed) enforced at repository layer, usecase translates `sql.ErrNoRows` to `constants.ErrNotFound` and state-guard violation to `constants.ErrConflict`.

- [ ] **Step 2: Wire `Jobs` into `contract/usecase/contract.go`**

Add field `Jobs ucJobs.Job` and `ucJobs.NewJobService(r.JobsRepository)` in `NewUsecase`.

- [ ] **Step 3: Write usecase unit tests**

Test validation (blank title/company/location → error, `BuildJobSlug` produces `unknown-company`/`unspecified-location` placeholders), pass-through to repository, error mapping (`ErrNotFound` → 404, `ErrConflict` → 409). Use repository mock if one exists pattern, or test service directly with a fake repo.

Run: `go test ./internal/usecase/jobs/... -v` Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add apps/backend/internal/usecase/jobs/jobs.go apps/backend/internal/usecase/jobs/jobs_test.go apps/backend/internal/contract/usecase/contract.go
git commit -m "feat(usecase): add jobs business logic layer"
```

---

### Task 7: Job HTTP handler

**Files:**
- Create: `apps/backend/internal/deliveries/api/handler/jobs/jobs.go`
- Create: `apps/backend/internal/deliveries/api/handler/jobs/jobs_test.go`

**Interfaces:**
- Consumes: `*config.Config`, `usecase.Job`, `middleware.AuthMiddleware` (for userUC), `middleware.ClerkOrAPIKeyMiddleware` (from Task 3).
- Produces: `handler` struct with `Route(g *httpx.Router)` registering:
  - `POST ""` (single create, Clerk or API key auth)
  - `POST "/bulk"` (bulk create, Clerk or API key auth)
  - `GET "/{slug}"` (get by slug, Clerk or API key auth)
  - `GET ""` (list recent or search by company, Clerk or API key auth; optional `limit` query default `20`, max `100`; optional `company` query for ILIKE filter)
  - `PATCH "/{slug}"` (update description, Clerk or API key auth)

- [ ] **Step 1: Write `jobs.go` handler**

Follow `projects/projects.go` patterns: `ctxval.PrincipalFromContext`, `httpx.ParseJSON`, `httpx.ExtractPathParam`, `httpx.Envelope` for errors, `httpx.RespondJSON` for success. Add swaggo annotations (`@Summary`, `@Description`, `@Tags`, `@Param`, `@Success`, `@Failure`, `@Router`) on each method. Extract the authenticated principal from context to get `UpdatedByType`/`UpdatedByID` for audit fields on create/update. Single create returns `201 Created` with `Created=true` when inserted, `200 OK` with `Created=false` when slug conflict ignored (existing row returned). Bulk create validates `len(jobs) <= 100` else `400`; returns per-item `{Slug string, Created bool}` results. `GET /v1/jobs` accepts optional `limit` (default `20`, max `100`) and optional `company` (ILIKE filter). PATCH validates description present and calls usecase with `UpdatedByType`/`UpdatedByID`; `constants.ErrNotFound` → `404`; business-rule violation (description non-null AND status=`analyzed`) → `409 Conflict`.

- [ ] **Step 2: Write handler tests**

Use a mock usecase (following `repository/mocks/` pattern with `go.uber.org/mock`). Tests: create success (201 + created=true), create duplicate slug (200 + created=false), bulk over limit → 400, get by slug 200/404, list recent with default and custom limit, list with company ILIKE filter, list limit clamped to max 100, patch description allowed when null, patch blocked when description non-null AND status=analyzed → 409, patch not found → 404, unauthorized when no principal.

Run: `go test ./internal/deliveries/api/handler/jobs/... -v` Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add apps/backend/internal/deliveries/api/handler/jobs/jobs.go apps/backend/internal/deliveries/api/handler/jobs/jobs_test.go
git commit -m "feat(handler): add jobs HTTP handlers"
```

---

### Task 8: Route registration and API wiring

**Files:**
- Modify: `apps/backend/internal/deliveries/api/handler/handler.go` (register jobs group)
- Modify: `apps/backend/docs/docs.go` (swagger entries for `/v1/jobs` routes)

**Interfaces:**
- Consumes: `contract.Contract` (has Cfg, Repository, Usecase).
- Produces: routes registered on `v1 := rt.Group("/v1")` under `v1.Group("/jobs")`. All jobs routes use `ClerkOrAPIKeyMiddleware` (from Task 3) which accepts either a valid Clerk session or a valid server-side API key.

- [ ] **Step 1: Register jobs routes in `handler/handler.go`**

Add to `handler.Route()`:
```go
// Job routes (CRU only; no DELETE).
jobs.NewHandler(h.contract).Route(v1.Group("/jobs"))
```

The `jobs.Handler.Route` method (Task 7) applies `NewClerkOrAPIKeyMiddleware(h.cfg, h.userUC)` internally per route, matching the `projects.Handler.Route` pattern.

- [ ] **Step 2: Update `docs/docs.go` with new `/v1/jobs` routes**

Add swagger path entries for `POST /v1/jobs`, `POST /v1/jobs/bulk`, `GET /v1/jobs/{slug}`, `GET /v1/jobs` (with optional `limit` and `company` query params), `PATCH /v1/jobs/{slug}` following the existing swagger JSON structure in `docs/docs.go`. Add definition entries for request/response bodies.

- [ ] **Step 3: Run `go build ./...` and `go test ./...`**

Run: `go build ./... && go test ./...` Expected: all compile and pass.

- [ ] **Step 4: Commit**

```bash
git add apps/backend/internal/deliveries/api/handler/handler.go apps/backend/docs/docs.go
git commit -m "feat(api): register jobs routes with Clerk-or-API-key auth"
```

---

### Task 9: End-to-end verification and docs

**Files:**
- Create: `docs/superpowers/plans/2026-11-09-jobs-crud-api.md` (this file, already created)

**Interfaces:**
- None.

- [ ] **Step 1: Full build + test + vet**

Run: `go build ./... && go vet ./... && go test ./...` Expected: all pass. Note any tests requiring a test database that couldn't run locally.

---

## Self-Review

**Spec coverage:**
- `jobs` table schema per `docs/database/jobs-data-model.md` → Task 1 migration.
- Slug `{title}-{company}-{location}`, unique, first-source-wins `ON CONFLICT DO NOTHING` → Task 5 repository `Create`/`BulkCreate`.
- Find by slug → `GetBySlug` (Task 5), exposed via `GET /{slug}` (Task 7).
- List recent `ORDER BY created_at DESC LIMIT` → `ListRecent` (Task 5), `GET` (Task 7).
- Search by company `ILIKE` → `SearchByCompany` (Task 5), `GET /v1/jobs?company=` (Task 7).
- `status`/`analyze_result`/`failure_message`/`updated_by_type`/`updated_by_id` columns → Task 1, Task 5. Status lifecycle `new` → `analyze_pending` → `analyzed` / `failed` documented; worker-driven transitions are Temporal responsibility (out of scope for API). Audit fields set from authenticated principal on every INSERT/UPDATE.
- `PATCH` description only, when null OR status != analyzed → Task 5 repository `UpdateDescription`, Task 7 handler.
- Bulk insert per-row, 100-row cap → Task 5 `BulkCreate`, Task 7 validation.
- Clerk session OR server-side API key auth (`Authorization: Bearer`, key→principal map) → Task 3, Task 8. Client headers never trusted for identity.
- Contract wiring (repository + usecase) → Tasks 5, 6.
- Handler follows `httpx.HandlerFunc` + `httpx.Envelope`/`RespondJSON` conventions → Task 7.
- Swaggo annotations on handler methods + `docs/docs.go` route entries → Tasks 7, 8.
- Unit tests: middleware (Task 3), repository (Task 5), usecase (Task 6), handler (Task 7) — each task includes its own test cycle.

**Step scan:** Each step writes exactly one thing (a file, a test, a wiring change, a commit). No step carries "TBD" or vague "add validation" — validation rules are specified inline.

**Type consistency:** `Jobs` interface (Task 5) matches `jobService` (Task 6) matches handler method calls (Task 7). `models.Job` (Task 4) used consistently across repository/usecase/handler. `config.MachineAuth.APIKeys` (Task 2) used by middleware (Task 3).

**Review Focus coverage:**
1. Slug conflict → tested in Task 5 repository test (duplicate slug returns created=false) and Task 7 handler test.
2. Bulk mixed conflicts → tested in Task 5 (`BulkCreate` per-row results).
3. PATCH blocked when description non-null + status=analyzed → repository `UpdateDescription` WHERE clause + Task 7 handler test expecting 409/400.
4. Missing/invalid API key → Task 3 middleware unit tests (valid key → proceeds, wrong key → 401, no key → 401).
5. Client headers never trusted for identity → API key determines caller identity server-side; `X-User-Name` ignored.
6. Audit attribution → `updated_by_type`/`updated_by_id` set from principal on INSERT/UPDATE (Task 5 repository, Task 7 handler).
7. Empty slug components → normalization is client-side per jobs-data-model.md; API validates slug non-empty (Task 6 validation). Placeholder logic (`unknown-company`, etc.) belongs to the scrape worker, not this API — noted as out of scope.

**Proportion:** Plan is ~1.5x the spec length, appropriate for a multi-file CRUD feature with tests, middleware, and contract wiring. No function bodies are transcribed; signatures and test names only.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-11-09-jobs-crud-api.md`. Please review the plan. Does it capture what you want?

If yes, choose an execution approach:

- **Subagent-driven** — A fresh subagent implements each task and a fresh reviewer checks it before the next one starts, then a whole-branch review at the end. Most thorough; costs a fresh context per task and per review. (Recommended here: tasks depend heavily on each other's interfaces — migration → model → repository → usecase → handler → wiring — so independent review per task catches interface drift early.)
- **Native** — I implement every task myself in this session, then one fresh reviewer checks the whole branch. Cheapest and fastest.

Which approach should we use?