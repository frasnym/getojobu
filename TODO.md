# Jobs CRUD API — Task Execution (inline, executing-plans skill)

BASE: `feat/jobs-crud-api` at 6267770 (Tasks 1-4 committed)

## Remaining tasks
- [ ] Task 5: Job repository + contract wiring (`repository/jobs.go`, `contract/repository/contract.go`, tests)
- [ ] Task 6: Job usecase + contract wiring (`usecase/jobs/jobs.go`, `contract/usecase/contract.go`, tests)
- [ ] Task 7: Job HTTP handler + tests (`handler/jobs/jobs.go`, `handler/jobs/jobs_test.go`)
- [ ] Task 8: Route registration + docs.go (`handler/handler.go`, `docs/docs.go`)
- [ ] Task 9: Full build/test/vet + final whole-branch review

## Progress ledger (SDD)
- See `.superpowers/sdd/2026-11-09-jobs-crud-api/progress.md` (Tasks 1-4 complete)

## Notes
- Module: `github.com/frasnym/getojobu/apps/backend`, Go 1.27.3
- No local Postgres; repository tests requiring a DB may not run locally (integration tests for CI)
- `docs/database/jobs-data-model.md` must NOT be modified
