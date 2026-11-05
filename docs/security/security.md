# Security and Tenant Isolation

## 1. Identity and authorization

- Clerk is the identity provider.
- The Go API verifies every browser access token.
- The API maps the verified Clerk subject to an internal `users.id`.
- Tenant ownership is derived from the authenticated request context, never accepted from an arbitrary request body/header.
- Every repository method accessing tenant-owned records must receive the authenticated tenant/user context.
- Machine credentials are separate from user session tokens, scoped, rotated, and bound to an explicit user/tenant context.

## 2. Database security

- Use separate runtime and migration roles.
- Runtime role must not own schema objects or have unnecessary DDL privileges.
- Use parameterized queries.
- Add composite FKs for relationships that must remain within a tenant.
- Add automated cross-tenant read/write tests.
- Do not copy Supabase RLS policies directly. Design Neon/PostgreSQL permissions for the actual runtime connection model. Application-layer authorization is mandatory.

## 3. Sensitive data

Treat CVs, contact details, application answers, browser sessions, and generated documents as sensitive.
- Store files in private R2 buckets.
- Use short-lived signed URLs where downloads are needed.
- Never log access tokens, cookies, passwords, full CV text, or application secrets.
- Apply retention rules to temporary browser artifacts and generated files.
- Restrict operational access to production data.

## 4. Application submission safety

- Require explicit user approval for each application attempt.
- Record who approved, when, and which application/document version was approved.
- Do not automatically retry an ambiguous submission.
- Use `submitted_unknown` for uncertain outcomes and require reconciliation.
- Provide a user-visible application history.

## 5. Worker hardening

- Run workers as non-root where possible.
- Give each worker only the secrets it needs.
- Isolate browser worker resources and temporary storage.
- Restrict outbound network access where practical.
- Do not expose browser remote-debugging endpoints.
- Keep worker images patched and dependencies pinned.

## 6. Operational controls

- Rate-limit authentication-sensitive and expensive endpoints.
- Apply per-user quotas/concurrency limits to scraping, LLM, and browser operations.
- Audit administrative actions.
- Alert on unusual failure rates, repeated retries, resource exhaustion, and cross-tenant authorization failures.
- Provide account export/deletion/anonymization procedures consistent with legal and operational retention requirements.
