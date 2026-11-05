# Repository Structure

The project is a monorepo with four application deployables and shared contracts.

```text
go-get-job/
├── apps/
│   ├── web/                         # Next.js + TypeScript (Vercel)
│   └── api/                         # Go modular monolith (VPS)
│       ├── cmd/api/
│       ├── internal/
│       │   ├── identity/
│       │   ├── profiles/
│       │   ├── skills/
│       │   ├── jobs/
│       │   ├── evaluations/
│       │   ├── applications/
│       │   ├── documents/
│       │   ├── activities/
│       │   ├── workflows/
│       │   └── admin/
│       ├── migrations/
│       └── sql/
├── workers/
│   ├── python/                      # Python Temporal worker
│   │   ├── worker/
│   │   ├── activities/
│   │   ├── workflows/
│   │   └── tests/
│   └── browser/                     # Isolated Playwright worker
│       ├── worker/
│       ├── activities/
│       └── tests/
├── packages/
│   ├── openapi/                     # OpenAPI source of truth
│   └── api-client-ts/               # Generated TS client (do not hand-edit)
├── infra/
│   ├── compose/
│   ├── caddy/
│   ├── temporal/
│   └── monitoring/
├── scripts/
├── docs/
│   ├── architecture/
│   ├── database/
│   ├── api/
│   ├── workflows/
│   ├── deployment/
│   ├── security/
│   └── rollout/
├── .github/
│   └── workflows/
├── Makefile
└── README.md
```

## Ownership rules

- `apps/web`: UI and client-side presentation. It may contain form schemas and display logic, but not authoritative business rules.
- `apps/api`: authoritative HTTP boundary, identity verification, authorization, validation, and short transactions.
- `workers/python`: Python workflows/activities that do not require a dedicated browser isolation boundary.
- `workers/browser`: browser automation activities, browser contexts, downloads, screenshots, and ATS interactions.
- `packages/openapi`: API contract source. Generated client output is derived from this.
- `infra`: deployment and operational configuration. No production secrets committed.
- `docs`: decisions, contracts, runbooks, and rollout plans.

## Worker separation

Keep `worker-browser` as a separate container/process from the beginning of the target architecture. It can share Python libraries with `worker-python`, but should have separate dependencies, resource limits, concurrency settings, and task queue registration. This reduces the risk that browser memory pressure or browser crashes affect non-browser activities.

Do not create `workers/go` initially. The Go API starts workflows and serves requests; a Go Temporal worker can be added only if there is a concrete Go-native activity or performance/ownership reason.

## Build and deployment boundaries

Each application deployable should have its own Dockerfile or Vercel build configuration and independent health checks. Shared contracts and generated clients should be versioned with the monorepo.
