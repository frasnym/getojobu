# API Design

**Base path:** `/v1`  
**Style:** REST/JSON  
**Contract:** OpenAPI source in `packages/openapi`; generate the TypeScript client for the Next.js app.

## 1. API responsibilities

The Go API is the authoritative public backend. It:
- verifies Clerk session tokens,
- maps the verified Clerk subject to an internal user,
- enforces tenant authorization,
- validates requests,
- performs short database transactions,
- starts/signals Temporal workflows,
- returns workflow/application state.

The web frontend must not connect directly to Neon or call worker activities directly.

## 2. Authentication

Browser requests use a Clerk-issued bearer token. The API verifies signature, issuer, audience/authorized party, expiry, and relevant claims according to Clerk's current token configuration. The API must not trust a user ID in the request body as identity.

Machine-to-machine callers (for example, a trusted cron or an existing VPS automation during migration) use a separate machine credential with narrow scopes. A machine credential must be bound to an explicit tenant/user context and must not allow arbitrary user impersonation.

## 3. Endpoint outline

### Current user and profile

```text
GET    /v1/me
PATCH  /v1/me
GET    /v1/me/skills
PUT    /v1/me/skills
GET    /v1/me/skill-gaps
```

### Job discovery and review

```text
GET    /v1/me/jobs
GET    /v1/me/jobs/{jobID}
GET    /v1/me/jobs/{jobID}/verdicts
POST   /v1/me/jobs/{jobID}/evaluations
POST   /v1/me/jobs/sync
```

The job listing is shared data, but the response may include the current user's latest verdict/application summary. The query must join only rows belonging to the authenticated user.

### Applications and documents

```text
GET    /v1/me/applications
POST   /v1/me/applications
GET    /v1/me/applications/{applicationID}
POST   /v1/me/applications/{applicationID}/approve
POST   /v1/me/applications/{applicationID}/skip
POST   /v1/me/applications/{applicationID}/generate-documents
GET    /v1/me/applications/{applicationID}/activities
```

Approval is an explicit action. It may start an application workflow, but should not imply that a successful external submission has already occurred.

### Workflow visibility

```text
GET    /v1/me/workflows/{workflowID}
POST   /v1/me/workflows/{workflowID}/cancel
```

Only expose workflow information belonging to the authenticated user's authorized operation. Do not expose arbitrary Temporal execution details or raw payloads.

## 4. Long-running operations

For work that runs asynchronously, return `202 Accepted`:

```json
{
  "workflow_id": "job-evaluation-user-job-run",
  "status": "accepted"
}
```

The exact response schema should be defined in OpenAPI. The API should use stable workflow IDs or explicit idempotency keys to prevent duplicate workflow starts from client retries.

## 5. Error conventions

Use a consistent error envelope with:
- machine-readable `code`,
- safe human-readable `message`,
- optional `request_id`,
- optional field-level validation errors.

Never return stack traces, SQL errors, secrets, internal file paths, or raw provider responses to clients.

## 6. Pagination and filtering

Use cursor pagination for potentially large job/application collections. Define stable sort order and opaque cursors. Validate and cap page sizes. Filters must be allowlisted and tenant-safe.

## 7. API implementation recommendations

- Go router: Chi.
- PostgreSQL access: pgx; sqlc for typed query generation where useful.
- Migrations: Goose.
- Validation: explicit request validation at the HTTP boundary.
- Logging: structured `slog`, with request/workflow correlation IDs.
- Tests: Testify plus integration tests against PostgreSQL.
- API docs: OpenAPI; generate the TypeScript client rather than maintaining duplicate frontend types.

These are implementation recommendations, not a reason to introduce additional services.
