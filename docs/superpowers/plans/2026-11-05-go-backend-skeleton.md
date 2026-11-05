# Go Backend Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the foundational Go API scaffold under `apps/backend/` — module layout, dependency configuration, configuration, structured logging, HTTP server wiring, routing layer, standardized error envelope, request validation, and tests for each layer — so the domain modules (identity, profiles, skills, jobs, …) can be added on top.

**Architecture:** A Go 1.27.3+ modular monolith with one `cmd/api` binary that wires an HTTP server onto a method-aware `net/http.ServeMux`. The request pipeline is: router → middleware chain (recovery, correlation ID, logging, authorization from authenticated context, validation) → HTTP handler → usecase → repository → pgx-generated SQL. Database access uses `pgx` with typed SQL; migrations use `golang-migrate/migrate` (per the Amartha `go-ap-ppob` reference, not Goose). No external web framework; Go's built-in router is the decision the plan locks in.

**Amartha reference:** This skeleton deliberately mirrors conventions from the adjacent Amartha service `bitbucket.org/Amartha/go-ap-ppob` (located at `documents/dev/amartha/go-ap-ppob`): the `internal/contract` wiring layer, `deliveries/api` vs `deliveries/consumer` split, `pkg/utils/response` error envelope, `go-config-loader-library` prefix config, `database.Connection{Primary, Replica}`, custom `go-playground/validator/v10`, `golang-migrate/migrate/v4`, and `testcontainers-go/modules/postgres` for integration tests.

**Tech Stack:** Go 1.27.3 (method-aware `ServeMux`), standard `net/http`, `pgx/v5`, `golang-migrate/migrate/v4`, `github.com/go-playground/validator/v10`, `github.com/google/uuid`, structured `slog`, `testify` + `testcontainers-go/modules/postgres` integration tests, `golangci-lint`, Docker.

**Spec:** The design travels with this plan: `docs/architecture/architecture.md`, `docs/architecture/repository-structure.md`, `docs/api/api-design.md`, `docs/deployment/deployment.md`, `docs/security/security.md`, `docs/workflows/temporal-workflows.md`, `README.md`.

## Global Constraints

- **Language/runtime:** Go 1.27.3 — method-aware `net/http.ServeMux` (exact: `ServeMux.Handle` now dispatches on method as well as path; path wildcards via `*` supported since 1.22).
- **Deployment target:** Docker Compose on VPS; `apps/backend` is a self-contained deployable with its own Dockerfile and health check.
- **Database:** `pgx/v5` for PostgreSQL; typed SQL queries generated with `sqlc`; migrations via `golang-migrate/migrate/v4` (per the Amartha `go-ap-ppob` reference, not Goose).
- **Authentication boundary:** Clerk session token verification is wired in at the HTTP layer; tenant/authorization is derived from the authenticated request context, never from an untrusted request body (security.md §1).
- **Error contract:** one consistent JSON error envelope with machine-readable `code`, safe human-readable `message`, optional `request_id`, optional field-level validation errors; never stack traces, SQL errors, secrets, internal paths, or raw provider responses to clients (api-design.md §5).
- **Logging:** structured `slog` with request/workflow correlation IDs (api-design.md §7).
- **Routing decision:** use Go 1.27.3 `net/http.ServeMux` (not Chi) as the plan's base — the user's instruction explicitly flags the standard router's method-aware routing and says Chi "may not even be needed"; Chi remains available as a drop-in replacement.
- **No premature dependencies:** no Redis, no microservices, no framework bloat beyond what the skeleton layer requires.
- **Tenant safety:** every repository method that touches tenant-owned rows receives the authenticated tenant/user context; cross-tenant read/write tests must exist (security.md §2).
- **Migrations:** schema migrations run as a controlled one-off job, not on every container startup (deployment.md §6).

## Amartha Reference Notes

The adjacent service `bitbucket.org/Amartha/go-ap-ppob` (at `documents/dev/amartha/go-ap-ppob`) is the canonical Go API reference for the Amartha platform. This skeleton deliberately mirrors its conventions; when in doubt, copy the pattern from there.

- **Package layout:** `internal/contract` (wiring), `deliveries/api` (HTTP) vs `deliveries/consumer` (Kafka), `pkg/utils/*` (shared helpers). The skeleton maps this to `apps/backend/internal/` + `apps/backend/internal/deliveries/api/`.
- **Error envelope:** Amartha wraps the payload as `{"response": {"code": <string>, "message": <string>, "info": <pagination?>}, "error": <string>}` with a `Meta` struct and `ErrorBuilder`. The skeleton's `internal/http/error.go` produces an envelope per `api-design.md §5` (`code`, `message`, `request_id`, optional `fields`); mirror Amartha's `Meta.Code`/`Meta.Message` naming where it helps consistency. Field-level validation errors (Amartha uses `Validation` error + `CustomValidator` with go-playground/validator/v10) map to the skeleton's `Fields []FieldError`.
- **Validation:** Amartha registers custom validator tags (e.g. `slug`) on `validator.Validate` and enforces via `CustomValidator.Validate`. The skeleton's `internal/http/validation/validate.go` should expose the same hook so domain tags (e.g. `clerk-subject`, `job-id`) can be added later.
- **Correlation ID:** Amartha uses `X-Correlation-ID` via `ctxval.SetCorrelationID` + `xlog` (go-x/log), derived from `xlog/ctxdata`. The skeleton's `internal/http/middleware/requestid.go` should set `X-Request-Id` (or `X-Correlation-ID`) and inject it into `slog` fields so it flows to all logs.
- **Config:** Amartha loads config via `go-config-loader-library` with a service prefix (e.g. `GO_AP_PPOB`) and structured `database.Connection{Primary, Replica}`. The skeleton's `internal/config/config.go` can adopt the same loader and the `Primary/Replica` structure.
- **Database:** Amartha uses `pgx/v4` + `lib/pq` + `golang-migrate/migrate/v4`; the skeleton upgrades to `pgx/v5` and keeps `golang-migrate/migrate/v4` (the plan replaces the earlier Goose proposal).
- **Integration tests:** Amartha boots PostgreSQL with `testcontainers-go/modules/postgres`. The skeleton's `make test-integration` should use the same approach and a `TEST_PG_DSN` env override.
- **Graceful shutdown:** Amartha's `pkg/utils/graceful` wires signal handling; the skeleton's `cmd/api/main.go` must handle SIGINT/SIGTERM and finish in-flight requests within `cfg.Server.ShutdownTimeout`.

## Review Focus

1. **HTTP method + path disambiguation** — a `GET /v1/me/jobs/{id}` and a `POST /v1/me/jobs/{id}/verdicts` must not collide; the `ServeMux` must dispatch on both path and method. Test: registered handlers for overlapping path patterns with different verbs each return the right handler.
2. **404 vs 405 routing** — unknown path → 404 (with the standard envelope); known path + wrong method → 405 (with the envelope), instead of leaking `http.NotFound` text.
3. **Tenant leakage through context** — a request built without a tenant context must be rejected by the authorization middleware before any DB call; test: call a handler with an empty context and assert 401/403 and zero database queries.
4. **Error envelope fidelity** — every error path returns JSON with `code` + `message` + `request_id`; test: trigger 4 validation failures and assert the envelope shape and that no stack trace or SQL text appears in the body.
5. **Cross-tenant row access** — a repository query for tenant A's jobs must never return tenant B's rows even if the SQL lacks a filter; test: assert tenant-scoped query results strictly equal one tenant's IDs.

---

## Task 1: Workspace scaffolding — module, go.mod, vendor-free layout, Docker, linter config

**Files:**
- Create: `apps/backend/go.mod`
- Create: `apps/backend/go.sum` (after deps)
- Create: `apps/backend/Dockerfile`
- Create: `apps/backend/Makefile`
- Create: `apps/backend/.golangci.yml`
- Create: `apps/backend/README.md`
- Create: `apps/backend/.dockerignore`
- Modify: `Makefile` (add `api/` build/test targets at the monorepo root — or confirm root `Makefile` already exists)

**Interfaces:**
- Consumes: Go 1.27.3+ toolchain; nothing from earlier tasks.
- Produces: a buildable module `github.com/getojobu/api` with `go build ./...` and `go test ./...` succeeding.

- [ ] **Step 1: Define the module layout** — pin `go 1.27.3` in `go.mod`.

Decide the directory tree (one responsible file each):
```
apps/backend/
├── cmd/
│   └── api/
│       └── main.go          # server bootstrap, wiring, entrypoint
├── internal/
│   ├── config/              # env-backed config
│   ├── http/                # request pipeline
│   │   ├── middleware/
│   │   ├── router/
│   │   ├── handler/
│   │   ├── response/
│   │   └── error.go         # canonical error envelope
│   ├── logging/             # slog setup + context keys
│   ├── repository/          # data access (typed SQL / golang-migrate)
│   └── service/             # business rules (stubs for domain modules)
├── migrations/              # golang-migrate/migrate versioned migration files
└── sql/                     # placeholder SQL files (typed pgx by default; sqlc optional)
├── pkg/                     # optional shared client-side helpers (future)
├── Dockerfile
├── Makefile
├── .golangci.yml
└── README.md
```

- [ ] **Step 2: Write `go.mod`** — pin `go 1.27.3` in `go.mod`. Add dependency lines with exact versions: `github.com/jackc/pgx/v5`, `github.com/golang-migrate/migrate/v4`, `github.com/go-playground/validator/v10`, `github.com/google/uuid`, `github.com/stretchr/testify` (dev-only), and `github.com/testcontainers/testcontainers-go/modules/postgres` (dev-only for integration tests).

- [ ] **Step 3: Write `Dockerfile` (multi-stage: build → distroless/alpine runtime)**

Build stage copies `cmd/api` and runs `go build -o /api`. Runtime stage from `gcr.io/distroless/static` (or `alpine` for dev), exposes port 8080, `CMD ["/api"]`. Include a health-check-equivalent `--health-cmd` equivalent comment pointing at `/health`.

- [ ] **Step 4: Write `Makefile` targets**

`make build`, `make run`, `make test`, `make test-integration`, `make lint`, `make migrate-up`, `make docker-build`, `make docker-run` — each invoking Go tooling with exact commands.

- [ ] **Step 5: Write `.golangci.yml`**

Enable: `errcheck`, `gosimple`, `govet`, `ineffassign`, `staticcheck`, `unused`, `structcheck`, `varcheck`, `deadcode`, `typecheck`, `prealloc`, `goimports`, `gofmt`, `gocritic`. Set run timeout and exclude rules conservatively (e.g., allow `//`-style linter directives).

- [ ] **Step 6: Write `README.md`**

One-paragraph description of the deployable, how to build/run/test locally, where configs live, and how migrations run.

- [ ] **Step 7: Verify**

Run `go build ./...`, `go vet ./...`, `golangci-lint run ./...` — all must pass. Commit: `chore: scaffold api module and build/tooling`.

## Task 2: Configuration layer — typed, validated, env-backed config

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/config/env.go` (env loaders with defaults)

**Interfaces:**
- Consumes: nothing.
- Produces: `config.Config` struct with fields `Server.Addr`, `Server.ReadTimeout`, `Server.WriteTimeout`, `Server.ShutdownTimeout`, `DB.DSN`, `DB.MaxOpenConns`, `DB.MaxIdleConns`, `DB.MaxConnLifetime`, `Clerk.Audience`, `Clerk.Issuer`, `Clerk.PublicKeyPEM` (or JWK endpoint), `Temporal.TargetURL`/`Temporal.TargetName`, `Temporal.TaskQueue`; `Load()` (`(Config, error)`) that reads from env + validates and returns a ready-to-use value.

- [ ] **Step 1: Write failing tests first**

- `test_Load_returns_error_when_required_env_is_missing` — unset `API_DATABASE_DSN` → `Load()` error.
- `test_Load_applies_default_ports_and_timeouts` — unset `API_SERVER_ADDR` → default `0.0.0.0:8080`.
- `test_Load_parses_clerk_issuer_and_audience` — set `API_CLERK_ISSUER=https://auth.getojobu.com`, `API_CLERK_AUDIENCE=getojobu-api` → struct equality.
- `test_Timeouts_are_positive_and_reasonable` — set `API_SERVER_READ_TIMEOUT=5s` etc., assert durations > 0 and < 60s.

- [ ] **Step 2: Run the tests — expect `Config` undefined.**

- [ ] **Step 3: Implement `Load() (Config, error)`**

Approach: read via `os.Getenv` with a small helper `mustEnv` that returns the value or `ErrMissingEnv`, convert timeouts via `time.ParseDuration`, validate each required field inline (return descriptive `errors.New` with the env key name), and assemble `Config`.

- [ ] **Step 4: Run tests — expect PASS.**

- [ ] **Step 5: Commit**

`feat(config): add typed validated config loader`.

## Task 3: Structured logging — slog setup, request ID injection, context keys

**Files:**
- Create: `internal/logging/logging.go`
- Create: `internal/logging/logging_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `logging.New(ctx, opts)` returning a `*slog.Logger`; context keys `RequestIDKey` (`string`), `TenantIDKey`, `WorkflowIDKey`; helper `WithContext(ctx, requestID, tenantID)` returning an `context.Context` with values set; `logging.FromContext(ctx)` returning the request ID; global `SetLogger`. Log fields at least `req_id`, `tenant_id`, `method`, `path`, `status`, `latency_ms`, `error`.

- [ ] **Step 1: Write failing tests**

- `test_WithContext_stores_and_retrieves_request_id` — set request id `abc123`, assert `FromContext` returns it.
- `test_New_logger_is_json_and_outputs_req_id` — capture a log line, assert it contains `"req_id":"abc123"`.
- `test_WithContext_composes_tenant_and_workflow_ids` — set both, assert both present.

- [ ] **Step 2: Run — expect undefined symbols.**

- [ ] **Step 3: Implement**

Approach: use `slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: ...}))`; use `context.WithValue` with exported unexported-type keys (`type requestIDKey struct{}`) to avoid collisions; build handler labels so every log line carries the shared fields.

- [ ] **Step 4: Run — expect PASS.**

- [ ] **Step 5: Commit**

`feat(logging): structured slog with request-id and tenant context`.

## Task 4: Standard error envelope and helper types

**Files:**
- Create: `internal/http/error.go`
- Create: `internal/http/error_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: types:
  - `type Code string` with constants `CodeInternal`, `CodeInvalidRequest`, `CodeNotFound`, `CodeUnauthorized`, `CodeForbidden`, `CodeConflict`, `CodeTooManyRequests`.
  - `type Error struct { Code Code `json:"code"`; Message string `json:"message"`; RequestID string `json:"request_id,omitempty"`; Fields []FieldError `json:"fields,omitempty"`; }`
  - `type FieldError struct { Field string `json:"field"`; Message string `json:"message"` }`
  - `func Envelope(code Code, message string, requestID string) *Error`
  - `func (e *Error) AddField(field, message string)`
  - `func (e *Error) Error() string`
  - `func (e *Error) StatusCode() int` (maps codes to 500/400/404/401/403/409/429)
  - `func (e *Error) WriteJSON(w http.ResponseWriter)` (sets status, content-type, encodes; never writes stack traces)
  - `func Wrap(err error, code Code) *Error`
  - `func IsAPIError(err error) bool` and `func AsAPIError(err error) *Error`

- [ ] **Step 1: Write failing tests**

- `test_Envelope_maps_internal_to_500_and_returns_json` — encode to bytes, assert `code`, `message`, `status` 500.
- `test_AddField_populates_fields_array` — add one field, assert length 1, field name and message.
- `test_Wrap_preserves_status_code_for_known_codes` — `Wrap(pgsql.ErrNoRows, CodeNotFound)` → status 404 (or whichever mapping the code list defines).
- `test_AsAPIError_returns_nil_for_builtin_errors` — assert `IsAPIError` false for `fmt.Errorf`.

- [ ] **Step 2: Run — expect undefined.**

- [ ] **Step 3: Implement**

Approach: exact field tags as listed above; `WriteJSON` must always write `Content-Type: application/json` and the mapped status before encoding; `Wrap` checks `AsAPIError`.

- [ ] **Step 4: Run — expect PASS.**

- [ ] **Step 5: Commit**

`feat(http): standard JSON error envelope`.

## Task 5: Method-aware router on Go 1.27.3 ServeMux

**Files:**
- Create: `internal/http/router/router.go`
- Create: `internal/http/router/router_test.go`
- Create: `internal/http/router/wildcards.go` (path param extraction helpers)

**Interfaces:**
- Consumes: `internal/http/error.go`.
- Produces: `router.New(mux *http.ServeMux) *Router` returning `*Router` with `Handle(method, pattern string, h http.HandlerFunc)`, `group(path string) *Router`, and `Routes() []Route` for introspection; handler wrapper `router.MiddlewareChain([]func(http.Handler) http.Handler)(http.HandlerFunc)`. Route: `struct { Method string; Pattern string; HandlerName string }`.

- [ ] **Step 1: Write failing tests**

- `test_Router_registers_and_dispatches_GET_and_POST_to_different_handlers` — register `GET /v1/health` and `POST /v1/health`, assert both dispatch correctly.
- `test_Router_dispatches_method_aware_wildcard` — `GET /v1/me/jobs/*` matches `/v1/me/jobs/123`, returns 200 for a registered handler at `/*`.
- `test_Router_405_for_known_path_wrong_method` — POST to a GET-only endpoint returns 405.
- `test_Router_404_for_unknown_path` — unknown path returns 404.
- `test_Router_Introspection_lists_registered_routes` — after registering three routes, `Routes()` length is 3 with correct method/pattern.

- [ ] **Step 2: Run — expect `ServeMux` method dispatch to need a wrapper; confirm Go 1.27.3 (go version)**

Note: Go 1.27's `ServeMux` dispatches on method+path for `ServeHTTP`; to surface 405 and collect routes, wrap each handler so the mux's `HandleFunc` records it.

- [ ] **Step 3: Implement**

Approach: each registered `Handler` is wrapped by a closure that records itself in an internal slice keyed by `(method, pattern)`, calls the mux, and intercepts `http.StatusNotFound` → 404 and `StatusMethodNotAllowed` → 405, both encoded via `error.go`'s envelope (using a canonical `errRouteNotFound`/`errRouteMethodNotAllowed`). Path params are parsed manually from `{id}` placeholders using `strings.Split` and simple index math (no external lib).

- [ ] **Step 4: Run — expect PASS.**

- [ ] **Step 5: Commit**

`feat(http): method-aware ServeMux router with 404/405 envelope`.

## Task 6: Middleware chain — recovery, request ID, logging, tenant/authorization

**Files:**
- Create: `internal/http/middleware/recovery.go`
- Create: `internal/http/middleware/requestid.go`
- Create: `internal/http/middleware/logging.go`
- Create: `internal/http/middleware/tenant.go`
- Create: `internal/http/middleware/tenant_test.go`
- Create: `internal/http/middleware/logging_test.go`

**Interfaces:**
- Consumes: `internal/logging`, `internal/http/error.go`, `internal/http/router`.
- Produces: middleware functions all of type `func(http.Handler) http.Handler`: `Recovery(logger *slog.Logger)`, `RequestID()`, `Logging(logger *slog.Logger, latencyLabel string)`, `Tenant(requireTenant bool)`. `Tenant`: if `ctx` lacks a tenant ID → 401/403 with envelope.

- [ ] **Step 1: Write failing tests**

- `test_Recovery_captures_panic_and_returns_500` — handler panics; assert response 500 with envelope, not the panic string.
- `test_RequestID_generates_and_injects_context` — two requests produce distinct request IDs.
- `test_RequestID_propagates_to_response_headers` — response header `X-Request-Id` present.
- `test_Tenant_rejects_request_without_context_tenant` — assert 401/403, zero DB calls.
- `test_Tenant_allows_request_with_context_tenant` — same request with tenant set → passes through (200 stub).
- `test_Logging_records_method_path_status_latency` — capture output, assert fields.

- [ ] **Step 2: Run — expect undefined functions.**

- [ ] **Step 3: Implement**

Approach: standard `defer { recover() }` in recovery; `x-request-id` header round-trip; tenant extracted from `ctx.Value(logging.TenantIDKey)`; logging uses `slog.With` plus `WithAttrs` around `h.ServeHTTP`, recording `status` via a `*statusCodeRecorder` wrapper implementing `http.ResponseWriter` that intercepts `WriteHeader`.

- [ ] **Step 4: Run — expect PASS.**

- [ ] **Step 5: Commit**

`feat(middleware): recovery, request id, logging, tenant chain`.

## Task 7: HTTP handler plumbing — request decoding, response encoding, validation gate

**Files:**
- Create: `internal/http/handler/handler.go`
- Create: `internal/http/handler/handler_test.go`
- Create: `internal/http/response/encoder.go`
- Create: `internal/http/response/encoder_test.go`
- Create: `internal/http/validation/validate.go`
- Create: `internal/http/validation/validate_test.go`

**Interfaces:**
- Consumes: `error.go`, `middleware`.
- Produces:
  - `handler.Base` embedding common logic: `ParseJSON(r, v) error` (decodes, returns 400 on malformed), `ExtractPathParam(r, name string) (string, error)`, `RespondJSON(w, code int, v any)` and `RespondError(w, err *Error)`.
  - `response.EncodeJSON(w, v)` (sets headers, encodes).
  - `validation.Validate(body map[string]any, rules map[string]Rule)` returning `[]FieldError`; `type Rule struct{ Required bool; MinLength int; MaxLength int; Pattern string }`.

- [ ] **Step 1: Write failing tests**

- `test_ParseJSON_returns_400_on_bad_json` — body `not json` → `ParseJSON` error; response body is envelope, no raw JSON error text.
- `test_ExtractPathParam_returns_error_when_missing` — `/v1/me/jobs/` with `/{jobID}` → 400.
- `test_RespondError_writes_envelope_as_json` — encode a `CodeNotFound` error, assert content-type and shape.
- `test_Validate_returns_field_errors_when_required_missing` — rules `{"email": {Required: true}}`, body `{"name":"x"}` → one field error on `email`.

- [ ] **Step 2: Run — expect undefined.**

- [ ] **Step 3: Implement**

Approach: `ParseJSON` uses `json.NewDecoder(r.Body).Decode(v)` and catches `*json.SyntaxError`; path params use the router's param extraction; validation uses reflection-free direct map lookups matching the allowlisted field names (api-design.md §6 allowlist principle).

- [ ] **Step 4: Run — expect PASS.**

- [ ] **Step 5: Commit**

`feat(http): request decode, response encode, validation gate`.

## Task 8: Health endpoint and readiness

**Files:**
- Create: `internal/http/handler/health.go`
- Create: `internal/http/handler/health_test.go`

**Interfaces:**
- Consumes: nothing domain-specific.
- Produces: `GET /health` → 200 `{status:"ok"}`, `GET /ready` → 200 when all registered dependencies are initialized, 503 otherwise; wired into the router under path `/` (outside `/v1`), not tenant-protected.

- [ ] **Step 1: Write failing tests**

- `test_health_returns_200_with_status_ok` — GET `/health` → 200, body `{"status":"ok"}`.
- `test_ready_returns_503_when_dependencies_uninitialized` — before wiring a DB check, `/ready` → 503.
- `test_health_is_not_tenant_protected` — call with empty tenant context → 200 (not rejected).

- [ ] **Step 2: Run — expect undefined.**

- [ ] **Step 3: Implement**

Approach: `*HealthChecker` with `SetReady(fn func() bool)`; `/ready` delegates to the latest ready predicate; no tenant middleware applied to these two paths.

- [ ] **Step 4: Run — expect PASS.**

- [ ] **Step 5: Commit**

`feat(http): health and readiness endpoints`.

## Task 9: Database connection and migration runner

**Files:**
- Create: `internal/repository/db.go` (DB wrapper with `pgxpool`)
- Create: `internal/repository/db_test.go`
- Create: `migrations/000001_initial_schema.up.sql` (empty migration, just a version comment)
- Create: `migrations/000001_initial_schema.down.sql`
- Create: `internal/migrate/migrate.go` (migration runner using `golang-migrate/migrate/v4`)
- Create: `cmd/api/migrate.go` (Go subcommand hook: `go run cmd/api/main.go migrate up`)

**Interfaces:**
- Consumes: `config.Config`.
- Produces: `repository.NewDB(ctx, cfg) (*sqlc.DB, error)` returning a connection pool with `MaxOpenConns`, `MaxIdleConns`, `MaxConnLifetime` from config; `golang-migrate/migrate/v4` CLI wired as `cmd/api migrate up`; optional `sqlc` query package `sql/db` generated with `sqlc generate` if typed SQL is enabled.

- [ ] **Step 1: Write failing tests**

- `test_NewDB_closes_pool` — `Close()` no error.
- `test_DB_pool_applies_config_max_open_conns` — pool configured to N open connections.
- `test_MigrationRunner_applies_one_empty_migration` — migrate up against a test PostgreSQL instance (via `testcontainers-go/modules/postgres`), assert version 1.

- [ ] **Step 2: Run — expect pool creation to fail without a running DB; spin one in the test.**

For integration tests, start PostgreSQL via Docker in `make test-integration` or require `TEST_PG_DSN`; document in README.

- [ ] **Step 3: Implement**

Approach: `pgxpool.NewConfig` built from `cfg.DB`; `golang-migrate/migrate/v4` runner initialized with the pool and the `migrations/` directory, exposed as `go run cmd/api/main.go migrate up` (controlled one-off job, not on every container startup). Optional `sqlc` package `sql/db` generated with `sqlc generate` if typed SQL is enabled instead of plain `pgx.Query`.

- [ ] **Step 4: Run — expect PASS.**

- [ ] **Step 5: Commit**

`feat(db): pgx pool and golang-migrate migration runner`.

## Task 10: Cross-tenant isolation tests and security gate

**Files:**
- Create: `internal/repository/cross_tenant_test.go`
- Create: `internal/repository/fixtures_test.go` (tenant-scoped seed data helpers)

**Interfaces:**
- Consumes: `db.go`, `error.go`.
- Produces: tests that seed two tenants' rows and assert a tenant-scoped query for tenant A returns zero rows of tenant B; a test that omits the tenant ID from the query context panics/warns at lint time via a helper requiring the tenant argument; a `TestTenancy` suite gated behind `TEST_TENANCY=1`.

- [ ] **Step 1: Write the cross-tenant test suite**

- `test_query_tenant_a_returns_only_tenant_a_rows` — seed tenant A and B; assert count == tenant A rows.
- `test_missing_tenant_context_does_not_skip_authorization` — authorization middleware rejects the request before the query runs.

- [ ] **Step 2: Run against PostgreSQL — expect PASS (with a seeded DB).**

- [ ] **Step 3: Wire into CI via `make test-integration` and `make test-security`.**

- [ ] **Step 4: Commit**

`test(security): cross-tenant isolation suite`.

## Task 11: `cmd/api/main.go` — final wiring and entrypoint

**Files:**
- Create: `cmd/api/main.go`

**Interfaces:**
- Consumes: everything above.
- Produces: a single executable that: loads `config`, sets up `logging`, initializes `repository.NewDB` (migration runner runs once if instructed), builds the middleware chain, registers all handlers via `router`, starts `http.Server` on `cfg.Server.Addr` with the recovery + request-id + logging + tenant middleware wrapping, handles `SIGINT`/`SIGTERM` graceful shutdown within `cfg.Server.ShutdownTimeout`, and exits non-zero on startup failure.

- [ ] **Step 1: Write `main` wiring**

Order: config → logger → DB → health checker → router → middleware → server. Each failure prints a one-line cause to stderr and exits 1.

- [ ] **Step 2: Wire the `/v1` routes for a smoke endpoint**

`GET /health`, `GET /ready`, `GET /v1/_smoke` → 200 with `{"status":"ok"}` (temporary; removed or marked deprecation before domain modules are added — or keep as a placeholder route only for smoke testing).

- [ ] **Step 3: Build, run, smoke test**

`make docker-build && make docker-run`; curl `/health`, `/ready`, `/v1/_smoke`, unknown path (404 envelope), known path + wrong method (405 envelope).

- [ ] **Step 4: Commit**

`feat: wire api entrypoint and HTTP server`.

## Task 12: Documentation — runbook for local development and rollout notes

**Files:**
- Create: `docs/superpowers/plans/go-backend-skeleton-scout.md` (keep this plan as a companion)
- Update: `docs/api/api-design.md` — note the routing decision: "Go 1.27.3 `net/http.ServeMux` method-aware routing is used for the v1 skeleton; Chi may be re-evaluated if handler composition complexity grows"
- Update: `README.md` — one-line note pointing to the skeleton plan.

**Interfaces:** none.

- [ ] **Step 1: Update `api-design.md` routing recommendation** (as above).
- [ ] **Step 2: Write local runbook** in `apps/backend/README.md`: environment variables table, `make` commands, how to run migrations (`go run cmd/api/main.go migrate up`), how to run unit vs integration tests (`TEST_PG_DSN`), how to regenerate typed SQL with `sqlc generate` if enabled, Docker compose entry for staging/production.
- [ ] **Step 3: Commit**

`docs: update api routing decision and local runbook`.

## Self-review checklist (run against this plan)

1. **Spec coverage:** architecture.md §5 request/flow → Tasks 5, 6, 11 (handler pipeline + tenant from context); §7 fresh database/migrations → Task 9; §8 tenant-scoping → Task 6 tenant + Task 10; api-design.md §2 auth → Task 6; §4 202 acceptance pattern → Task 11 leaves the shape (202 + `workflow_id`) wired as an envelope response type in Task 7/11; §5 error envelope → Task 4; §6 pagination → skeleton leaves pagination helpers in Task 7 as an empty `Paginated` response type stub (add if scope permits); §7 tooling recommendations (pgx/`golang-migrate`/validator/slog/testify) → all reflected; deployment.md §4 health checks + §6 one-off migrations → Tasks 8, 9.
2. **Step scan:** each step names a single unambiguous action with a checkable result; no step contains an undetermined algorithm body.
3. **Type consistency:** `Code`, `Error`, `Middleware` type families named identically across Tasks 4–7; `Load()` signature used only in Task 2; `repository.NewDB(ctx, cfg)` signature introduced in Task 9 and consumed only in Task 11.
4. **Review Focus:** all five bullet items have dedicated test steps in Tasks 5 (routing/404/405), 6 (tenant leakage), 4 (envelope fidelity), 10 (cross-tenant rows), 5 (method-aware wildcards).
5. **Proportion:** 12 tasks; each scoped to one file group; plan is decision-heavy, code-light.

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-11-05-go-backend-skeleton.md`. Please review the plan. Does it capture what you want — notably the routing decision to build on Go 1.27.3 `net/http.ServeMux` method-aware routing instead of Chi as api-design.md suggests?

Which execution approach would you prefer?

- **Subagent-driven** — a fresh subagent implements each task and a fresh reviewer checks it before the next one starts, then a whole-branch review at the end. Most thorough; costs a fresh context per task and per review.
- **Native** — I implement every task myself in this session, the way this harness runs work, then one fresh reviewer on the most capable model checks the whole branch. Cheapest and fastest; no independent review until the end. Runs well with a mid-tier session model, since the plan carries the design.

**For this plan I recommend Native**, because the tasks compose cleanly on earlier tasks' interfaces, there are no domain-business dependencies between them, and a mistake in the skeleton layer is cheap to fix before any domain module is written. Does the plan capture what you want, and which approach should we use?
