go-get-job/
│
├── apps/
│ │
│ ├── web/ # Next.js frontend
│ │ ├── app/
│ │ │ ├── (public)/
│ │ │ │ ├── page.tsx
│ │ │ │ ├── pricing/
│ │ │ │ └── about/
│ │ │ │
│ │ │ ├── (auth)/
│ │ │ │ ├── sign-in/
│ │ │ │ └── sign-up/
│ │ │ │
│ │ │ ├── (dashboard)/
│ │ │ │ ├── layout.tsx
│ │ │ │ ├── dashboard/
│ │ │ │ ├── jobs/
│ │ │ │ │ ├── page.tsx
│ │ │ │ │ └── [jobId]/
│ │ │ │ ├── applications/
│ │ │ │ ├── profile/
│ │ │ │ ├── skills/
│ │ │ │ ├── workflows/
│ │ │ │ ├── notifications/
│ │ │ │ └── settings/
│ │ │ │
│ │ │ └── api/ # Only frontend-specific webhooks if needed
│ │ │
│ │ ├── components/
│ │ │ ├── ui/
│ │ │ ├── layout/
│ │ │ ├── jobs/
│ │ │ ├── applications/
│ │ │ ├── profile/
│ │ │ └── workflows/
│ │ │
│ │ ├── features/
│ │ │ ├── auth/
│ │ │ ├── job-search/
│ │ │ ├── job-verdict/
│ │ │ ├── applications/
│ │ │ └── profile/
│ │ │
│ │ ├── hooks/
│ │ ├── lib/
│ │ │ ├── api/
│ │ │ ├── auth/
│ │ │ └── utils/
│ │ │
│ │ ├── public/
│ │ ├── tests/
│ │ ├── next.config.ts
│ │ └── package.json
│ │
│ ├── api/ # Go backend
│ │ ├── cmd/
│ │ │ └── api/
│ │ │ └── main.go
│ │ │
│ │ ├── internal/
│ │ │ ├── platform/ # Infrastructure shared by modules
│ │ │ │ ├── database/
│ │ │ │ ├── temporal/
│ │ │ │ ├── logger/
│ │ │ │ ├── config/
│ │ │ │ ├── http/
│ │ │ │ ├── metrics/
│ │ │ │ └── storage/
│ │ │ │
│ │ │ ├── auth/
│ │ │ │ ├── clerk.go
│ │ │ │ ├── middleware.go
│ │ │ │ └── context.go
│ │ │ │
│ │ │ ├── user/
│ │ │ │ ├── handler.go
│ │ │ │ ├── service.go
│ │ │ │ ├── repository.go
│ │ │ │ └── model.go
│ │ │ │
│ │ │ ├── skill/
│ │ │ ├── job/
│ │ │ ├── verdict/
│ │ │ ├── application/
│ │ │ ├── covernote/
│ │ │ ├── activity/
│ │ │ ├── workflow/
│ │ │ ├── notification/
│ │ │ ├── integration/
│ │ │ └── admin/
│ │ │
│ │ ├── migrations/
│ │ ├── queries/ # SQL source for sqlc
│ │ │ ├── users.sql
│ │ │ ├── skills.sql
│ │ │ ├── jobs.sql
│ │ │ ├── verdicts.sql
│ │ │ └── applications.sql
│ │ │
│ │ ├── gen/ # Generated sqlc code
│ │ ├── api/ # OpenAPI specification
│ │ ├── tests/
│ │ │ ├── integration/
│ │ │ └── e2e/
│ │ ├── sqlc.yaml
│ │ ├── go.mod
│ │ └── Dockerfile
│ │
│ └── workers/ # Temporal worker processes
│ ├── go/
│ │ ├── cmd/
│ │ │ └── worker/
│ │ │ └── main.go
│ │ ├── workflows/
│ │ │ ├── job_search.go
│ │ │ ├── job_evaluation.go
│ │ │ └── application.go
│ │ ├── activities/
│ │ │ ├── job.go
│ │ │ ├── verdict.go
│ │ │ └── application.go
│ │ └── go.mod
│ │
│ ├── python/
│ │ ├── src/
│ │ │ ├── activities/
│ │ │ │ ├── scraping/
│ │ │ │ ├── matching/
│ │ │ │ ├── cover_letter/
│ │ │ │ └── profile/
│ │ │ ├── integrations/
│ │ │ │ ├── llm/
│ │ │ │ ├── job_boards/
│ │ │ │ └── storage/
│ │ │ ├── clients/
│ │ │ │ ├── backend_api.py
│ │ │ │ └── temporal.py
│ │ │ └── main.py
│ │ ├── tests/
│ │ ├── pyproject.toml
│ │ └── Dockerfile
│ │
│ └── browser/
│ ├── src/
│ │ ├── activities/
│ │ │ ├── prepare_application.py
│ │ │ ├── fill_form.py
│ │ │ ├── submit_application.py
│ │ │ └── verify_submission.py
│ │ ├── browser/
│ │ ├── profiles/
│ │ └── main.py
│ ├── tests/
│ └── Dockerfile
│
├── packages/
│ ├── api-client/ # Generated TypeScript API client
│ ├── api-contracts/ # OpenAPI-derived schemas/types
│ ├── design-system/ # Shared UI components, if needed
│ └── temporal-contracts/ # Cross-worker payload definitions
│
├── bin/ # Existing CLI / VPS scripts
│ ├── job-scrape.sh
│ ├── job-verdict.sh
│ ├── job-repair.sh
│ └── ...
│
├── infra/
│ ├── docker/
│ │ ├── api.Dockerfile
│ │ ├── worker-go.Dockerfile
│ │ └── worker-python.Dockerfile
│ │
│ ├── compose/
│ │ ├── docker-compose.yml
│ │ ├── docker-compose.dev.yml
│ │ └── docker-compose.prod.yml
│ │
│ ├── caddy/
│ │ └── Caddyfile
│ │
│ ├── temporal/
│ │ ├── config/
│ │ └── namespaces/
│ │
│ └── monitoring/
│ ├── prometheus/
│ └── grafana/
│
├── scripts/
│ ├── dev.sh
│ ├── test.sh
│ ├── generate-api-client.sh
│ └── migrate.sh
│
├── docs/
│ ├── architecture/
│ │ ├── overview.md
│ │ ├── backend.md
│ │ ├── temporal.md
│ │ ├── security.md
│ │ └── deployment.md
│ │
│ ├── database/
│ ├── api/
│ ├── workflows/
│ └── runbooks/
│
├── .github/
│ └── workflows/
│ ├── test.yml
│ ├── build.yml
│ └── deploy.yml
│
├── .env.example
├── .gitignore
├── Makefile
└── README.md
