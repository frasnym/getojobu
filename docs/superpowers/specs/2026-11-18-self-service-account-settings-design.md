# Self-service account settings — design

## 1. Intent

Let a signed-in user view and manage their own `users` row. This is the first slice of a broader "manage my data" area. It is not an admin tool and it is not a public profile.

The user needs one place to correct the personal details the app uses, and one place to deactivate their account. Today there is no such screen: the only user-facing pages are the jobs feed, user jobs, and their detail pages.

## 2. What the user decided

- The feature is for a normal user managing their own data, not for an admin managing other people.
- `/settings` is a section, not a single page. This first slice builds only `/settings/account`.
- Later, `/settings/career` will hold skills, experience, and education. Career stays one page with sections, not one route per entity.
- Do not build placeholder pages for the future routes.
- `full_name` is editable locally. This overrides the "Clerk snapshot" wording in `docs/database/users-data-model.md` for this field, and that document must be updated to match.

## 3. What I decided (my calls, not the user's)

- `email` is read-only in the UI. Clerk is authoritative for the verified email; the local column is a snapshot.
- `role`, `onboarding_completed_at`, and `created_at` are read-only metadata. Role changes must never come from a frontend request.
- Deactivation is a soft delete (`deleted_at`), behind an explicit confirmation. No hard delete: the retention and erasure policy is still undefined.
- One explicit "Save changes" button. No autosave and no unsaved-changes guard in this slice.
- The deactivation action signs the user out on success. Clerk-side session revocation is out of scope here and stays an open follow-up.

## 4. Goals

- A signed-in user can see all fields of their own `users` row that are safe to show.
- A signed-in user can change `full_name`, `headline`, `phone`, `portfolio_url`, `linkedin_url`, and `github_url`.
- A signed-in user can deactivate their own account with an explicit confirmation.
- Every read and write is scoped to the authenticated internal user ID, derived from the verified Clerk token.

## 5. Non-goals

- Admin user management, role changes, or listing other users.
- Changing `email`, `role`, or `onboarding_completed_at` from this screen.
- Skills, experience, education, preferences, or any table other than `users`.
- Hard delete, data export, or anonymization.
- Clerk password, session, or identity changes — those stay in Clerk's own UI.
- Placeholder routes for `/settings/career`.

## 6. Anti-goals — what would make this a failure

- A user reading or writing another user's row, even by crafting a request.
- A client-supplied user ID being trusted as identity.
- Silent saves, or a save that succeeds without visible confirmation.
- A deactivation that happens without an explicit confirmation, or one that looks reversible when it is not.
- A new visual style that clashes with the existing `apps/web/DESIGN.md` token system.

## 7. Constraints from the repository

- Backend: Go modular monolith, `net/http` with method-aware `http.ServeMux`, `Request.PathValue()` for path params, `pgx`, structured `slog` logging. No new HTTP framework.
- Frontend: Next.js + TypeScript under `apps/web`, Clerk Next.js integration, existing Tailwind config and CSS variables from `apps/web/DESIGN.md`.
- API contract: OpenAPI generated from Go struct comments via `swag init`. Do not hand-edit `apps/backend/docs/swagger.json` or `swagger.yaml`. Regenerate the TypeScript client types from the generated spec.
- Auth: verify the Clerk token, derive the internal user ID from the verified subject, never from the body. Authorization is enforced in Go, not by hiding UI.
- Schema of `users` today: `id`, `clerk_user_id`, `email VARCHAR(128)`, `full_name VARCHAR(128)`, `role`, `onboarding_completed_at`, `created_at`, `updated_at`, `deleted_at`, plus `headline VARCHAR(256)`, `phone TEXT`, `portfolio_url TEXT`, `linkedin_url TEXT`, `github_url TEXT` from migration 11.
- Existing code to reuse: `usecase/users.User` with `FindByID`, `Update(id, email, fullName)`, and `SoftDelete(id)`; the `repository.Users` interface; the auth middleware principal; the API response envelope in `docs/api/api-design.md`; `apps/web/lib/api/client.ts`; `components/layout/navbar.tsx`.
- No new service, dependency, or database migration is needed for this slice.

## 8. The how

### 8.1 Backend

The design doc for the API lives in `docs/api/api-design.md`, which already lists `GET /v1/me` and `PATCH /v1/me` but has no handler. Add:

- `GET /v1/me` — returns the authenticated user's own record.
- `PATCH /v1/me` — partial update of the editable fields only.
- `POST /v1/me/deactivate` — soft-deletes the authenticated user.

Handler rules:

- Read the internal user ID from the principal set by the auth middleware. Never accept a user ID from the body or query string.
- `PATCH` accepts only `full_name`, `headline`, `phone`, `portfolio_url`, `linkedin_url`, `github_url`. Reject unknown fields, `email`, and `role` rather than silently ignoring them.
- Validate types and lengths against the columns: `full_name` and `headline` are non-empty after trimming (or null to clear), URLs must be valid `http`/`https` absolute URLs or null, and lengths must fit the columns. Return `VALIDATION_FAILED` with a field-level `details` array using the existing error envelope.
- The `users` usecase currently updates only email and full name. Extend `Update` (or add a clearly named sibling) to cover the six editable columns, keeping Clerk out of it. Do not widen it to role or onboarding.
- Do not change the Clerk webhook in this slice. The `user.updated` and `user.deleted` handlers are already broken: they pass the Clerk ID into a UUID-keyed query. Fixing that is a separate task. When it is fixed, `user.updated` must sync email only so it cannot overwrite a locally edited `full_name`; `user.created` still seeds the initial name from Clerk.
- Return a response DTO that omits `clerk_user_id`. Include `id`, `email`, `full_name`, `role`, `onboarding_completed_at`, `created_at`, `updated_at`, and the five optional profile fields.
- `SoftDelete` already sets `deleted_at`, and `FindByClerkUserID` already rejects deactivated accounts on the next request. Do not add a second authorization path.
- Update handler struct comments, then regenerate Swagger. Add the three endpoints to `docs/api/api-design.md`.

### 8.2 Frontend

Route and navigation:

- `apps/web/app/(app)/settings/layout.tsx` holds a section shell with a left-hand nav listing "Account" only. Make it a single column on mobile.
- `apps/web/app/(app)/settings/account/page.tsx` renders the account form. `/settings` redirects to `/settings/account`.
- Add a "Settings" link to `components/layout/navbar.tsx` for signed-in users.

Page content:

- Heading "Account". One form, no cards of vanity metrics.
- "Account details" — labels and inputs for Full name, Headline, Phone, Portfolio URL, LinkedIn URL, GitHub URL. Each field has a visible label, inline validation, and its own error line. Empty input maps to null on save.
- "Clerk-managed" note next to a read-only email field.
- Read-only metadata line for role, onboarding completion, and member since. These are plain text, not fake inputs.
- "Save changes" primary action with loading, success toast, validation, and network-error states. Disable the button while saving; do not close or reset the form on failure.
- "Account state" section with a "Deactivate account" button. Clicking it opens a confirm dialog that says access stops immediately, then calls the deactivate endpoint and signs the user out. Cancel leaves the account untouched.

Client wiring:

- Add `getMe`, `updateMe`, and `deactivateMe` to `apps/web/lib/api/endpoints.ts`, using the existing `apiClient` and Clerk token flow.
- Add a `useMe` hook under `apps/web/hooks/` following the existing hook pattern.
- Hand-add the new `User` types to `lib/api/types.ts`, matching the regenerated Swagger schema exactly. No TypeScript generator exists in this repo; introducing one is a separate task that needs approval. Note the hand-maintained contract as a known deviation in that file.

Visual language:

- Reuse the zinc-neutral tokens, radii, type scale, and focus-ring rules in `apps/web/DESIGN.md`. No new palette, no new component library.
- Vector icons only, consistent stroke width, 44px minimum tap targets, one column at 390px.
- Never rely on color alone for status; read-only values carry text labels.
- Respect reduced motion and keep focus visible in the confirm dialog.

## 9. Tests

- Go handler tests: `GET /v1/me` returns only the authenticated user; `PATCH /v1/me` rejects `email`, `role`, and unknown fields; URL and length validation returns field-level errors; a valid patch returns the DTO without `clerk_user_id`; `POST /v1/me/deactivate` sets `deleted_at` and the next authenticated request is rejected.
- Use mocks of `usecase/users.User` in handler tests, following the existing handler test pattern.
- Frontend tests: form renders loaded values, validation blocks invalid URLs, save success shows a toast, save failure keeps entries, and deactivation requires confirmation before any request.
- Run the repository's documented Go tests, frontend typecheck, and frontend tests after implementation.

## 10. What is left to the builder

- Exact component and test file names, and whether the section nav is a separate component.
- The concrete URL-validation helper and error-message wording.
- Whether `Update` is extended in place or a new usecase method is added; keep it consistent with the existing repository method naming.
- Precise markup and Tailwind classes, as long as they stay inside the existing `DESIGN.md` system.
- Toast implementation, using whatever the app already has; do not introduce a new library for it.

## 11. Open items / approvals needed

- Retention and data-erasure policy is still undefined. This slice soft-deletes only; it does not anonymize or hard-delete.
- Clerk-side session revocation on deactivation is not in this slice. Decide before launch whether the API must call Clerk to invalidate active sessions.
- `full_name` ownership was decided by the user: locally editable. Updating `docs/database/users-data-model.md` so it stops calling this field a pure Clerk snapshot is part of the plan.
- The Clerk `user.updated` webhook is broken today (Clerk ID passed to a UUID-keyed query). Fixing it is a separate task, and that fix must sync email only so it cannot overwrite a locally edited `full_name`.
- No TypeScript client generator exists; hand-adding types is a known deviation and is recorded in the plan.
- No commit is included. Get approval before committing anything from this design.
