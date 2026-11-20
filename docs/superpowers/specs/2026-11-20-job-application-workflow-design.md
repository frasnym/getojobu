# Job application workflow — click, review, approve, submit

## 1. Intent

getojobu can find jobs, evaluate them, and generate CVs and cover letters. The last step is still manual: opening the apply link and filling the form. This design adds a Temporal workflow that opens the apply URL, works out how the form works, proposes every answer, and then waits for the user to review and approve before an LLM-driven browser step attempts the submission.

Only forms that do **not** require login are in scope for this first version. If the page needs a login, the workflow stops and records `needs_login` instead of trying to sign in.

The workflow is deliberately split at the approval point. Everything before approval only reads and proposes. Only after an explicit, attributable approval does the browser step touch the submit control.

## 2. What the user decided

- The agent opens the apply URL and checks for a login requirement.
- If no login is needed, the agent analyzes the form.
- If the form is not known, the agent works out "how to apply on this form" and saves that knowledge.
- If the form is known, the agent loads that knowledge and works out the input values.
- Input values are generated from the user's career profile, skills, and the job.
- The user gets back a list of every field and the proposed value.
- The user can edit those values in the web app.
- After the user approves, an LLM drives the browser to try to submit.
- Most steps run async through Temporal.
- The review, edit, and approval screen lives in the web app, on the user-job detail page, as a new tab.
- The LLM, not fixed selector code, performs the final submit click.

## 3. What I decided (my calls, not the user's)

- The submit click stays LLM-driven, but only inside a narrow contract: the LLM may pick and click the submit control only after every required field is resolved and approval is verified. It must read the on-page result and stop on CAPTCHA, MFA, or ambiguity. Page text is treated as untrusted data, never as instructions that can change the task.
- Any edit to the reviewed values bumps a version and hash and voids the previous approval. The user must approve again after editing.
- This is one `JobApplicationWorkflow` with deterministic ID `job-application-{user_job_id}`. Approval is a durable signal with an expiry; the workflow does not poll the UI.
- "How to apply on this form" is stored as a structured, versioned form playbook in Neon: field map, field order, input hints, decision summaries, model/prompt version, and last-verified time. Raw model reasoning is never stored.
- Stored playbooks are hints only. Every run re-identifies fields on the live page before use; a stale selector is never trusted blindly.
- The browser step runs in the separate browser worker on the `job-browser` queue, with its own concurrency and resource limits. The Python worker owns form analysis and field-value generation on `job-evaluation`.
- Missing confidential values (work authorization, visa, salary, notice period) are marked unanswered and block submit. The system never invents them.
- A retry reuses the same application row and workflow ID with `WorkflowIDReusePolicy.ALLOW_DUPLICATE_FAILED_ONLY`. The row gets an `attempt` number; `application_events` keeps the full history across attempts. This preserves the unique `user_job_id` and the deterministic workflow ID.
- The Python worker keeps its current pattern: no direct database access. Every write (review, playbook, outcome, events) goes through new machine-key Go endpoints following the existing `ApiKeyAuth` pattern.
- Field values reuse the existing machine-key profile-snapshot endpoint that `MatchJobWorkflow` already calls, plus job data from `user_jobs`/`jobs`. No new profile read endpoint is added.
- CV and cover-letter bytes reach the browser worker through a new machine-key presigned-download endpoint resolved from the file keys in the approved review payload. If no CV exists, the review shows a blocking missing-file state rather than generating one here.
- The `apply` tab requires an explicit `Start application` button, not auto-start on tab open. Navigation must not silently launch a browser session.

## 4. Goals

- One link between a user and a job moves from "apply URL known" to a recorded apply outcome.
- Review is readable: the user sees every field, the proposed value, and why it was chosen before anything is submitted.
- Approval is attributable to the authenticated user and is required for any submit attempt.
- Re-running the same step does not create duplicate applications or double submissions.

## 5. Non-goals

- Forms that require login, CAPTCHA solving, MFA, or bypassing site restrictions.
- Submitting without recorded approval.
- Retrying a possibly-successful submission.
- A Go Temporal worker, a new queue beyond `job-browser`, or changes to other deployables besides web, backend, Python worker, and browser worker.
- Historical evaluation changes, document storage in Neon, or a generic prompt-chat UI.
- Auto-converting an ambiguous result into `applied`.

## 6. Anti-goals — what would make this a failure

- A submit happens with no approval, or after an edit invalidated approval.
- The LLM clicks something it cannot confirm is the real submit control, or ignores a CAPTCHA.
- An ambiguous submit is retried and creates a second application.
- One user sees another user's form values, answers, or application state.
- The workflow waits forever for approval with no expiring path.

## 7. Constraints from the repository

- Backend: Go 1.27.3, `net/http` with method-aware `http.ServeMux`, `Request.PathValue()`, `pgx`, Goose migrations, `log/slog`. No new HTTP framework.
- Auth: Clerk only for user calls. Derive the internal user from the verified token; never trust a browser-supplied user ID. Machine keys are separate and never impersonate a user.
- Neon is the system of record. No Supabase, `auth.uid()`, or RLS assumptions.
- Current latest migration is `000020_user_profile_cache.sql`; this work uses `000021`.
- Swagger is generated from Go struct comments with `swag init`. Do not hand-edit generated files.
- Existing tab list on `/user-jobs/[slug]` is `details`, `report`, `resume`, `cover-letter`, `chat` in `apps/web/app/(app)/user-jobs/[slug]/page.tsx`; the new tab is `apply`.
- Template for this document: `docs/superpowers/specs/2026-11-18-evaluation-lifecycle-design.md`.
- Reference safety rules: `docs/workflows/temporal-workflows.md` §5–7 and the legacy `apps/temporal/job-apply-flow.md` (ideas only, not implementation).
- Every user-owned read and write must be scoped to the authenticated internal user ID.

## 8. The flow

```text
start (POST /v1/applications, Clerk user, user_job_id)
  -> workflow job-application-{user_job_id}
     1. open apply URL                [browser activity]
     2. detect login?  yes -> needs_login, stop
                       no  -> continue
     3. analyze form                  [browser extract + LLM map]
     4. playbook known? no  -> save new form playbook and continue
                         yes -> load playbook
     5. resolve every field value     [profile + skills + LLM generation]
     6. write review payload to Neon
     7. wait for approval signal (with expiry)
        user reviews / edits in web -> version + hash bump on edit
     8. on approval: verify version still current, then
        LLM-driven browser fill + submit
     9. verify on-page result -> applied | failed | submitted_unknown
```

Steps 1–6 and 8–9 are activities. Step 7 is a Temporal wait condition plus signal. No step blocks an HTTP request.

## 9. Data model — migration 000021

### 9.1 `form_playbooks`

One row per apply domain or platform. This is the saved "how to apply on this form" knowledge. It is shared across users only for non-personal structure; never store a user's answers here.

| Column | Type | Meaning |
|---|---|---|
| `id` | uuid PK | |
| `platform_key` | text unique | e.g. `greenhouse`, `ashby`, `company-x` |
| `domain_patterns` | jsonb | list of apply URL domains this playbook matches |
| `field_mappings` | jsonb | canonical field -> locator hints and input type |
| `field_order` | jsonb | canonical order for the review card |
| `form_hints` | jsonb | submit button hints, file fields, cover-letter field |
| `decision_summary` | text | short structured summary of how the form works, no raw reasoning |
| `model_version` | text | model used to produce the playbook |
| `prompt_version` | text | prompt used to produce the playbook |
| `status` | text | `proposed`, `verified`, `stale`, `disabled` |
| `confidence` | numeric | |
| `last_verified_at` | timestamptz | |
| `created_at`, `updated_at` | timestamptz | |

Indexes on `platform_key` and `status`. A GIN index on `domain_patterns` only if the lookup query needs it.

### 9.2 `applications`

One row per user job apply attempt.

| Column | Type | Meaning |
|---|---|---|
| `id` | uuid PK | |
| `user_job_id` | uuid FK, unique | one application per user job link |
| `attempt` | integer default 1 | incremented on each explicit retry |
| `form_playbook_id` | uuid FK nullable | playbook used |
| `apply_url` | text | URL that was opened |
| `status` | text | lifecycle status, see §12 |
| `review_version` | integer default 0 | bumps on every edit |
| `review_hash` | text | sha256 of canonical review payload |
| `review_payload` | jsonb | fields, values, sources, files, unanswered, unknowns |
| `approval_user_id` | uuid FK nullable | internal user who approved |
| `approval_at` | timestamptz | |
| `approval_version` | integer | version that was approved |
| `submit_result` | jsonb | on-page verification result |
| `external_application_id` | text | ATS receipt when present |
| `applied_at` | timestamptz | |
| `created_at`, `updated_at` | timestamptz | |

The unique constraint on `user_job_id` prevents duplicate applications. Approval is valid only while `approval_version = review_version` and `approval_at` is after the last edit.

### 9.3 `application_events`

Append-only audit log: `application_id`, `event_type`, `actor_type`, `actor_id`, `event_json`, `created_at`. One row for each of: started, login detected, form analyzed, playbook saved, playbook loaded, values generated, review created, review edited, approval received, approval expired, submit started, submit verified, submit failed, submit ambiguous, skipped.

## 10. Status values

Add a Go status type next to the existing ones:

| Status | Meaning |
|---|---|
| `starting` | workflow started, browser not opened yet |
| `needs_login` | apply page requires login; stopped |
| `analyzing` | form analysis or value generation running |
| `awaiting_review` | review payload written, waiting for approval |
| `submitting` | approved, LLM-driven submit in flight |
| `applied` | submission confirmed on-page |
| `failed` | submission confirmed failed; retryable under policy |
| `submitted_unknown` | result ambiguous; quarantined, never auto-retried |
| `skipped` | user chose not to apply |
| `expired` | approval window elapsed |

## 11. API contract (Go, Swagger generated)

All routes use Clerk auth and derive the internal user from the token.

| Method | Route | Purpose |
|---|---|---|
| `POST` | `/v1/applications` | start workflow for a `user_job_id`; idempotent by deterministic workflow ID |
| `GET` | `/v1/applications/{id}` | current application, review payload, status, events |
| `PATCH` | `/v1/applications/{id}/review` | save user edits; bumps version and hash; invalidates approval |
| `POST` | `/v1/applications/{id}/approve` | record approval for the current version; signal the workflow |
| `POST` | `/v1/applications/{id}/skip` | stop and mark skipped; signal the workflow |

Machine-key routes for the workers, all behind `ApiKeyAuth`, with the target application passed explicitly and never a user ID supplied by the caller:

| Method | Route | Purpose |
|---|---|---|
| `POST` | `/v1/machine/applications/review` | upsert review payload, version, hash |
| `POST` | `/v1/machine/applications/playbooks` | save a new form playbook |
| `POST` | `/v1/machine/applications/events` | append an audit event |
| `GET` | `/v1/machine/applications/{id}/context` | profile snapshot, job data, approved file keys |
| `POST` | `/v1/machine/applications/{id}/outcome` | write terminal status and result |
| `GET` | `/v1/machine/applications/{id}/documents/{type}/download` | presigned URL for CV or cover letter bytes |

Ownership is checked on every route: the application's `user_job_id` must belong to the token user. A mismatch is 404, not 403, to avoid leaking existence. Machine keys may read and drive workflows only with an explicit user context; they cannot approve on behalf of a user.

Handler structs get Swagger comments; regenerate `docs/swagger.json`, `swagger.yaml`, and `docs.go` with `swag init`. The generated TypeScript client is updated from the contract.

## 12. Web UI — `apply` tab

Add `apply` to `ReportTab` and the tab allowlist in `apps/web/app/(app)/user-jobs/[slug]/page.tsx`, with a new `ApplyTab` component. It shows, in one screen:

- current status and a short explanation, including `needs_login` and `submitted_unknown` states,
- a table of every form field: label, proposed value, source (`profile`, `skills`, `generated`, `user`), and editable input,
- files to upload (CV, cover letter) and their status,
- unanswered fields highlighted, with a clear note that they block submit,
- the review version, and a warning that editing voids approval,
- buttons `Start application`, `Save changes`, `Approve and submit`, and `Skip`. `Start application` is the only action that calls `POST /v1/applications`.

`Approve and submit` calls the approve endpoint only after the user has had a chance to review; no click elsewhere is treated as approval. After approval the tab shows `submitting`, then polls the application endpoint until a terminal state. Loading, empty, validation, network, and retry states are handled explicitly. The app never displays another user's application.

## 13. Workflow and activities

`JobApplicationWorkflow` lives in the Python worker. Input: `user_job_id`, `user_id`, `apply_url`, internal IDs for profile and job data. It does not receive CV contents, credentials, or secrets.

| Activity | Owner | Work |
|---|---|---|
| `open_apply_url` | browser | open page, detect login wall, return page structure |
| `analyze_form` | browser + LLM | extract structured DOM fields; LLM maps to canonical fields |
| `save_or_load_playbook` | python | match domain, save new playbook or load existing |
| `resolve_field_values` | python + LLM | fill from profile/skills, generate text answers, mark unanswered |
| `write_review` | python | upsert application review payload, version, hash |
| `verify_approval` | python | re-check version/hash/approval user before submit |
| `llm_submit_form` | browser + LLM | LLM-driven fill and submit click under the contract in §14 |
| `verify_submission` | browser | read confirmation text, receipt ID, or redirect URL |
| `record_outcome` | python | write terminal status, result, events, report to API |

Timeouts and retries: form analysis and value generation get bounded start-to-close timeouts with 2 attempts; browser activities use heartbeats and a hard execution cap; `verify_submission` gets 1 attempt and never runs after an ambiguous result; approval wait uses a durable timer with expiry. Concurrency on the browser queue is capped independently.

`resolve_field_values` reads the profile snapshot through the same machine endpoint `MatchJobWorkflow` uses, plus the job data already on `user_jobs`/`jobs`. It does not add a new profile data source. The browser worker fetches CV and cover-letter bytes at submit time from the machine presigned-download endpoint using the file keys bound to the approved review payload; it never holds document bytes in Temporal history.

## 14. LLM submit contract

This is the one place that differs from fixed-selector automation, so the guardrails are explicit.

- It runs only after `verify_approval` passes.
- The LLM sees a structured, sanitized description of the page: field labels, types, options, and candidate submit controls. It never receives raw page text as instructions, and page-embedded instructions are ignored.
- It proposes field values from the already-approved review payload. It does not invent new personal or confidential answers.
- It selects the submit control from the candidate list and clicks it once.
- On CAPTCHA, MFA, login redirect, unknown modal, or no clear submit control, it stops and raises. It does not try to work around the restriction.
- After clicking, `verify_submission` reads a reliable on-page signal. Success needs confirmation text, a receipt ID, or a thank-you/application URL. Ambiguity means `submitted_unknown`, no retry, and a human check.

## 15. Approval and edit behavior

`PATCH /review` recomputes the canonical JSON payload and its sha256, increments `review_version`, and clears `approval_at`/`approval_user_id`. `POST /approve` records the current version and sends the Temporal signal. The workflow re-verifies on the submit path; if the version changed after approval, it stops with `awaiting_review` and requires a new approval. The review TTL is left to the builder as a configuration choice, with a default of 24 hours.

## 16. Outcome and recovery

- `applied`: confirmation observed; record receipt and URL.
- `failed`: confirmation the submit did not go through. A user-triggered retry increments `attempt`, reuses the same row and workflow ID under `ALLOW_DUPLICATE_FAILED_ONLY`, and requires fresh approval.
- `submitted_unknown`: quarantine. No sweep may flip it to `applied`, and no automatic retry is allowed. A human reconciles on the target site.
- `needs_login`, `expired`, `skipped`: terminal for this attempt; user can start a new one after resolving the cause, using the same attempt rules as `failed`.

## 17. Security and privacy

- Scope every read and write to the authenticated internal user from the Clerk token.
- Never log CV contents, full form payloads, answers, cookies, or session material. Log IDs, status, and short diagnostics only.
- Browser contexts are per operation and cleaned up. No public debugging port.
- Playbooks contain no personal answers. Never reuse one user's values for another user.
- Treat job-site and ATS pages as untrusted input; only structured extraction is passed to the LLM.

## 18. Tests

- Migration from empty database; unique `user_job_id`; playbook uniqueness; FK constraints.
- Go handler tests: auth, ownership rejection as 404, edit bumps version and clears approval, approve on stale version rejected, idempotent start.
- Workflow tests: login path stops; unknown form saves a playbook; known form loads it; approval wait times out; edit voids approval; `submitted_unknown` is never retried.
- Activity tests: field resolution marks missing confidential values unanswered; LLM submit contract stops on CAPTCHA and ambiguity; verification requires a reliable signal.
- Frontend: tab renders, edit then approve flow, terminal states, and another user's application is not visible.
- Run the documented Go tests, Python tests, frontend type check and tests, and `swag init` before reporting completion.

## 19. Docs and contract updates

- `docs/api/api-design.md` with the five routes.
- `docs/workflows/temporal-workflows.md` with the new workflow family and task queue use.
- `docs/database/data-model.md` with the three tables.
- Regenerate Swagger from Go comments; update the generated TypeScript client.
- This design doc is the source of truth for decisions; the implementation plan follows after approval.

## 20. Open decision — browser worker

`workers/browser` does not exist yet. The repository has only `workers/python`, and the browser queue has no worker registered. The architecture names a separate browser/Playwright deployable, but this design currently assumes one is available. Before planning, decide whether this work scaffolds the browser worker from scratch (worker entrypoint, pinned Playwright image, per-operation browser contexts, concurrency and resource limits, deployment wiring) or whether the browser activities are stubbed first and the worker is built in a later phase. The answer sets the size of the largest single part of this work.
