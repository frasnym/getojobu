# Database Design — Neon PostgreSQL

**Provider:** Neon PostgreSQL  
**Database strategy:** Fresh database; no wholesale migration from Supabase.  
**Security model:** Clerk identity verified by Go API; tenant authorization enforced in the application and reinforced with database constraints.

## 1. Design principles

1. `jobs` is a shared pool of listing facts.
2. Evaluation/verdicts and application lifecycle are per-user.
3. Each user/job pair has exactly one matching evaluation. A retry updates that row in place; a successful evaluation is terminal.
4. Every user-owned row carries an internal user key.
5. Composite foreign keys should prevent cross-tenant references where a row relates to another user-owned row.
6. Preserve audit/activity history. Archive listings rather than deleting rows referenced by application history.
7. Store generated file binaries in R2; store metadata and object keys in Neon.

## 2. Proposed core entities

| Entity | Scope | Purpose |
|---|---|---|
| `users` | Tenant | Internal user record mapped to Clerk subject |
| `user_skills` | Tenant | Normalized skills and profile-derived skill facts |
| `user_skill_gaps` | Tenant | Skills or experience gaps identified for the user |
| `jobs` | Shared | Canonical job listing facts, source identifiers, URLs, description, first/last seen, archive state |
| `evaluations` | Tenant | Evaluation run metadata: prompt version, model, input hash, timestamps |
| `job_verdicts` | Tenant | Per-user, per-job evaluation outcome and evidence |
| `job_applications` | Tenant | User-specific application lifecycle |
| `cover_notes` | Tenant | Generated application documents/versions and R2 references |
| `application_activities` | Tenant | Audit timeline for evaluation, review, generation, and application events |

This entity list follows the revised database reference. Column-level details and constraints should be finalized in SQL migrations and reviewed against the API contract before implementation.

## 3. Identity mapping

Use an internal UUID primary key for users. Store the Clerk subject as a unique external identity, for example `clerk_user_id TEXT UNIQUE NOT NULL`. Do not use a public-facing slug or browser-provided user ID as the authorization key.

The API derives the current user from a verified Clerk token and resolves it to the internal `users.id`. All tenant-owned queries use that internal ID.

## 4. Evaluation idempotency and lifecycle

Each user/job pair has exactly one `matching_evaluations` row, keyed by `user_job_id`. Retrying a failed evaluation updates that row in place and never creates a second row. A `success` evaluation is terminal and is never re-evaluated.

Uniqueness rule:

```sql
UNIQUE (user_job_id)
```

This deliberately reverses the earlier `UNIQUE (user_id, job_id, evaluation_id)` rule and the multiple-historical-evaluations model. Migration `000018_single_evaluation.sql` collapses duplicate rows to the latest by `created_at`, adds the `user_job_id` unique constraint, and drops `user_jobs.latest_evaluation_id` (and its index); the evaluation is now joined directly through `user_job_id`.

The evaluation status lifecycle is:

```text
insert            new
trigger        -> queued  -> pending -> success   (terminal)
failed retry   -> queued_retry -> pending -> success | failed
```

The insert endpoint creates a row only when none exists for the `user_job_id` and leaves an existing row's status unchanged, which keeps `success` terminal. The trigger starts the workflow first and flips the status only after a successful start (`new → queued` on a first run, `failed → queued_retry` on a retry), so a `queued` row always has a workflow behind it. A retry clears the evaluation-result columns but leaves report, resume, and cover-letter columns and their statuses untouched.

Store audit metadata such as:
- `evaluation_id`
- `prompt_version`
- `model`
- `input_hash`
- `evaluation_time`
- `verdict_source`

Prefer structured evidence, concise decision summaries, and version metadata. Do not persist raw model chain-of-thought. If internal debugging needs additional traces, store safe structured diagnostics with access controls and retention limits.

## 5. Application lifecycle

Initial lifecycle:

```text
draft -> applying -> applied
                   -> failed
                   -> submitted_unknown

draft -> skipped
```

`submitted_unknown` is a quarantine state. Do not automatically convert it to `applied` or `failed`; require a deliberate recovery action after checking the external ATS state.

Transitions should be enforced in application logic and tested. A database CHECK constraint can validate allowed status values, but it does not by itself enforce transition history.

## 6. Tenant isolation

- Every tenant-owned table has a non-null internal `user_id`.
- All API repository methods require a user context for tenant-owned data.
- Composite FKs should ensure that an application cannot reference another user's verdict and an activity cannot reference another user's application/verdict.
- Add integration tests for cross-tenant reads and writes.
- Do not assume Supabase RLS policies or `auth.uid()` semantics apply to Neon. Neon is PostgreSQL; authorization must be designed for the actual connection and role model used by the Go API.
- Use separate DB roles for runtime and migrations where practical. Runtime should not have schema-owner privileges.

## 7. Listing retention

Jobs are shared listing records. Use `is_archived` and `expired_at` to retire old listings. Do not physically delete rows referenced by applications or activity history. Define a retention policy separately for unreferenced listing data and user data.

## 8. Migration approach

Use versioned SQL migrations in `apps/backend/migrations/`. Use a migration tool such as Goose. Migrations run from a controlled deployment job using a direct Neon connection, not from every API replica at startup.

Recommended order:
1. Users and identity mapping.
2. Skills and skill gaps.
3. Shared jobs and source identifiers.
4. Evaluations and verdicts.
5. Applications and lifecycle constraints.
6. Documents/cover notes and activity history.
7. Indexes, constraints, retention support, and operational metadata.

Run migrations against a disposable development branch and staging branch before production. Take a backup/restore point before production schema changes. Prefer forward fixes over destructive rollback after data has been written.

## 9. Data seeding from legacy Supabase

Fresh start means no wholesale import. A separate, explicit seed process may import:
- the owner's profile,
- normalized skills,
- selected job listings,
- selected document metadata if files are copied to R2.

Do not import legacy verdicts, application state, or raw reasoning by default. Any imported dataset must have a documented mapping, count checks, and a repeatable dry-run mode.

## 10. Cover-letter generation fields

The cover-letter feature adds three columns to `matching_evaluations` in migration `000016_cover_letter_schema.sql`, after the existing resume columns. These are user-specific generation state on the per-user evaluation row, not shared job facts, and no PDF object key is stored.

| Column | Type | Purpose |
|---|---|---|
| `cover_letter_status` | `VARCHAR(32)` | Lifecycle state: `queued`, `queued_retry`, `pending`, `success`, `failed` |
| `cover_letter_content` | `JSONB` | The finished letter text and its provenance metadata as one object |
| `cover_letter_failure_message` | `TEXT` | Append-only failure history, one timestamped line per failed attempt, newest last, capped at the last 10 lines |

`cover_letter_content` is a single object rather than separate text/model/version columns so provenance travels with the letter. Its shape is `{ "text": "...", "metadata": { "model": "...", "prompt_version": "..." } }`: `text` holds the plain letter with line breaks, and `metadata.model` / `metadata.prompt_version` use the same nested provenance shape as job analysis, evaluation, and report results. A forced regeneration sets status to `queued_retry` and keeps the previous `cover_letter_content` visible until the new letter succeeds, so a failed regeneration does not wipe a working letter. Failure messages are a fixed safe string with the specific exception logged server-side only; no prompt or CV contents are stored. See `docs/superpowers/specs/2026-11-17-cover-letter-design.md` for the full decision record.
