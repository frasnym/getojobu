# go-get-job — Implementation and Product Rollout Plan

**Target:** Public multi-user v1  
**Database:** Fresh Neon PostgreSQL  
**Identity:** Clerk  
**Deployment:** Next.js on Vercel; Go API and Temporal workers on VPS  
**Legacy:** Existing Supabase deployment retained temporarily as a reference/transition system; no wholesale data migration.

## 1. Release scope

V1 includes:
1. Registration, login, and user profile.
2. Career profile and skills management.
3. Job discovery and scraping.
4. AI job matching/evaluation with history.
5. Human review of matched jobs.
6. CV and cover-letter generation.
7. Application tracking and activity history.

Application submission is approval-gated. Do not enable unattended application submission for public users in v1.

## 2. Delivery principles

- Build the backend API and data model first.
- Keep the existing automation running until the replacement path is verified.
- Integrate existing Python automation through Temporal rather than rewriting it immediately.
- Establish the OpenAPI contract early so the frontend can proceed independently.
- Dogfood with the owner's account before inviting external users.
- Do not expose the product publicly until tenant isolation, recovery, and operational limits are tested.

## 3. Phases and exit criteria

### Phase 0 — Baseline and inventory

Tasks:
- Freeze and document current working behavior.
- Inventory all scripts, cron jobs, Temporal workflows/activities, readers, writers, and data dependencies.
- Identify which legacy data is worth selectively seeding.
- Record current environment variables by name only; do not copy secret values into docs.
- Define staging and production environments.

Exit criteria:
- Every existing writer/reader has an owner and migration disposition.
- A rollback/cutback procedure exists for the legacy automation.
- No unknown scheduled writer remains.

### Phase 1 — Architecture and contracts

Tasks:
- Commit architecture and repository structure.
- Create OpenAPI skeleton and error/pagination conventions.
- Define internal user identity mapping from Clerk.
- Finalize entity relationships and tenant ownership.
- Define Temporal workflow/task-queue contracts.
- Create threat model and environment/secrets plan.

Exit criteria:
- API contract reviewed.
- Tenant boundaries and machine-auth model reviewed.
- No Supabase-specific assumptions remain in the new architecture.

### Phase 2 — Foundation and infrastructure

Tasks:
- Create monorepo directories and CI checks.
- Provision Neon development/staging/production branches.
- Configure Clerk development/staging/production.
- Provision R2 buckets and access policies.
- Prepare VPS Compose services, private networks, Caddy routes, and monitoring.
- Deploy Temporal with persistent storage and private UI access.

Exit criteria:
- Staging environment deploys reproducibly.
- Health checks and basic alerts work.
- Secrets are not committed or exposed to frontend bundles.

### Phase 3 — Database and Go API

Tasks:
- Implement versioned Neon migrations.
- Implement user/Clerk identity mapping.
- Implement profile and skills modules.
- Implement shared job listing storage.
- Implement evaluations/verdicts and append-oriented history.
- Implement applications, status transitions, documents metadata, and activities.
- Add cross-tenant integration tests.
- Publish OpenAPI and generate TypeScript client.

Exit criteria:
- Migrations pass from an empty database.
- Tenant isolation tests pass.
- API integration tests pass.
- No browser or worker can bypass authorization boundaries.

### Phase 4 — Temporal and worker integration

Tasks:
- Configure Temporal client in Go API.
- Port/adapt existing Python activities incrementally.
- Implement discovery and evaluation workflows.
- Implement document-generation workflow.
- Implement isolated browser worker and application workflow.
- Add idempotency, retry policies, timeouts, heartbeats, cancellation, and workflow versioning.
- Ensure ambiguous external submission becomes `submitted_unknown`.

Exit criteria:
- End-to-end staging flow works: discover → evaluate → review → generate documents → approve → track.
- Retried activities do not create duplicate evaluations/applications.
- Browser worker failures do not destabilize other workers.
- No submission occurs without explicit approval.

### Phase 5 — Frontend

Tasks:
- Build Clerk sign-in/sign-up and account onboarding.
- Build profile and skills screens.
- Build job discovery/results and evaluation evidence views.
- Build review/approve/skip interactions.
- Build document generation and preview/download.
- Build application list/detail/activity timeline.
- Integrate generated API client.

Exit criteria:
- Main v1 journeys work against staging API.
- Loading, empty, error, and retry states are handled.
- Frontend has no direct database access.

### Phase 6 — Dogfooding and controlled cutover

Tasks:
- Use the new product with the owner's account.
- Selectively seed only approved profile/skills/job data.
- Switch one automation path at a time to Neon/API/Temporal.
- Compare results against the legacy process.
- Stop legacy writers for each domain once the new path is verified.
- Avoid dual-write as a long-term operating mode.

Exit criteria:
- New system is the sole writer for migrated domains.
- Data reconciliation passes.
- Legacy cutback instructions are tested.
- No critical workflow depends on an undocumented legacy table/column.

### Phase 7 — Private beta

Tasks:
- Invite a small group (for example, 3–5 users).
- Validate onboarding, tenant isolation, profile import, job relevance, document generation, and application tracking.
- Add per-user quotas and rate limits.
- Test account export/deletion/anonymization.
- Review support and incident procedures.

Exit criteria:
- No cross-tenant data exposure.
- Costs and resource usage are predictable.
- Workflow recovery is understandable to users/operators.
- Critical bugs are resolved before broader release.

### Phase 8 — Public v1

Tasks:
- Publish product terms/privacy information and support path.
- Enable controlled signups.
- Monitor onboarding conversion, workflow success, queue backlog, browser failures, and cost per active user.
- Keep application submission approval-gated.
- Expand sources and concurrency only after measuring reliability.

Exit criteria:
- Operational owner and incident response are defined.
- Backup/restore and data deletion procedures are exercised.
- Capacity and spend limits are configured.

## 4. Suggested implementation order

1. Repository and CI foundation.
2. Neon schema and migrations.
3. Go API foundation + Clerk verification.
4. Profile and skills.
5. Shared jobs and ingestion.
6. Evaluations/verdict history.
7. Applications and activity history.
8. Temporal integration.
9. Python worker migration/adaptation.
10. Browser worker isolation.
11. OpenAPI-generated frontend client.
12. Next.js user journeys.
13. Dogfooding, beta, public release.

## 5. Definition of done for every phase

- Tests added and passing.
- OpenAPI/docs updated when contracts change.
- Tenant authorization reviewed.
- Logs contain correlation identifiers but no secrets or sensitive document content.
- Metrics and alerts exist for new background workflows.
- Deployment and rollback/cutback notes updated.
- No unapproved production migration or destructive data operation.

## 6. Explicitly deferred

- Go Temporal worker, unless a concrete need appears.
- Microservices.
- Redis, until justified by a measured need.
- Unattended application submission.
- Wholesale Supabase history migration.
- Complex billing/plan enforcement until product usage and pricing are defined.
