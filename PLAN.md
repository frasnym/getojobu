# Jobs CRUD API - Execution Plan (Tasks 5-9)

## Goal
Implement Go backend CRUD API for the `jobs` table: single/bulk insert, read by slug, list recent, search by company, restricted description update, plus status/analysis/audit columns and static API key auth. Tasks 1-4 complete.

## Status (all complete)
- [x] Task 5a: test compile fix + DB Begin/Tx
- [x] Task 5b: repository jobs.go
- [x] Task 5c: contract/repository wiring
- [x] Task 5d: build + repo tests
- [x] Task 6: usecase
- [x] Task 7: handler
- [x] Task 8: routes + docs
- [x] Task 9: full verification

## Files changed
- apps/backend/internal/utils/database/postgres.go (DB.Begin + pgx.Tx)
- apps/backend/internal/models/job.go (UpdatedByType/UpdatedByID on CreateJobParams)
- apps/backend/internal/repository/jobs.go (new)
- apps/backend/internal/repository/jobs_test.go (fixed compile)
- apps/backend/internal/repository/mocks/jobs_mock.go (generated)
- apps/backend/internal/contract/repository/contract.go (JobsRepository wiring)
- apps/backend/internal/usecase/jobs/jobs.go (new)
- apps/backend/internal/usecase/jobs/jobs_test.go (new)
- apps/backend/internal/usecase/jobs/mocks/job_mock.go (generated)
- apps/backend/internal/contract/usecase/contract.go (Jobs usecase wiring)
- apps/backend/internal/deliveries/api/handler/jobs/jobs.go (new)
- apps/backend/internal/deliveries/api/handler/jobs/jobs_test.go (new)
- apps/backend/internal/deliveries/api/handler/handler.go (route registration)
- apps/backend/docs/docs.go (swagger entries)

## Verification
- go build ./... : PASS
- go vet ./... : PASS
- go test ./internal/... : PASS (repository/usecase/handler tests skip/pass)
- No local Postgres/Docker → repository integration tests skip (CI only)

## Follow-up
- regenerate docs with `swag init` when swag deps are available (manually updated)
- CI should run repository tests against a real Postgres (JOBS_TEST_DATABASE_URL/DATABASE_URL)
