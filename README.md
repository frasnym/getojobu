# go-get-job

A multi-user job-search and application-assistance platform. The product discovers job listings, evaluates them against each user's career profile, helps prepare application materials, and tracks applications. Application submission is approval-gated.

## Architecture at a glance

- **Web:** Next.js + TypeScript, deployed to Vercel.
- **API:** Go modular monolith, deployed to the VPS.
- **Authentication:** Clerk.
- **Application database:** Neon PostgreSQL.
- **Workflow orchestration:** Temporal, self-hosted on the VPS.
- **Workers:** Python worker and isolated browser worker, deployed to the VPS.
- **Object/file storage:** Cloudflare R2.
- **API contract:** OpenAPI; generate and share a TypeScript client with the web app.

The repository is a monorepo. The four application deployables are `web`, `api`, `worker-python`, and `worker-browser`. Temporal Server, its persistence database, Neon, R2, Caddy, and monitoring are infrastructure dependencies, not application deployables.

## Product boundary for v1

V1 includes account/profile management, skills, job discovery, AI matching/evaluation, human review, CV/cover-letter generation, and application tracking. Actual submission must require explicit user approval. Do not ship unattended automatic application submission as a default public-user capability.

## Documentation map

- [Architecture](docs/architecture/architecture.md)
- [Repository structure](docs/architecture/repository-structure.md)
- [Database design](docs/database/data-model.md)
- [API conventions](docs/api/api-design.md)
- [Temporal workflows](docs/workflows/temporal-workflows.md)
- [Deployment](docs/deployment/deployment.md)
- [Security](docs/security/security.md)
- [Rollout plan](docs/rollout/implementation-rollout.md)

## Core principles

1. Keep the backend a modular monolith until measured scale or team boundaries justify splitting it.
2. Keep long-running scraping, LLM, document, and browser work out of synchronous HTTP handlers.
3. Temporal coordinates workflows; workers execute activities.
4. Treat all user-owned data as tenant-scoped. Never trust a user ID supplied by the browser.
5. Keep shared job-listing facts separate from per-user evaluations and application state.
6. Make workflow activities idempotent and make human approval explicit and auditable.
7. Keep the existing Supabase deployment as a legacy reference during the transition. The new product starts with a fresh Neon database; no wholesale Supabase data migration.
