# Self-service Account Settings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a signed-in user view and edit their own `users` row and deactivate their account through `/settings/account`.

**Architecture:** Add a Clerk-protected `/v1/me` handler group in the Go modular monolith that reads identity from the request principal and calls the existing users usecase. Add a `UserProfileInput` path through repository and usecase for the six editable columns. Build a single form page under a `/settings` section in Next.js, reusing the existing API client, hooks, and design tokens.

**Tech Stack:** Go 1.27.3, `net/http` + method-aware `http.ServeMux`, pgx, Goose, gomock, swag; Next.js 15, TypeScript, Tailwind, Clerk, Vitest.

**Spec:** `docs/superpowers/specs/2026-11-18-self-service-account-settings-design.md`

## Global Constraints

- Go toolchain baseline: Go 1.27.3. Use standard library `net/http` and method-aware `http.ServeMux`; use `r.PathValue()` for path params. No Chi, Echo, Gin, or Fiber.
- Derive the internal user ID from the verified Clerk principal in request context (`ctxval.PrincipalFromContext`). Never accept a user ID from the body or query string.
- `email`, `role`, `onboarding_completed_at`, and timestamps are read-only in this feature. Never accept them from a frontend request.
- Swagger/OpenAPI is generated from Go struct comments with `swag init`. Never hand-edit `apps/backend/docs/swagger.json`, `swagger.yaml`, or `docs.go`.
- Do not add a migration, service, framework, or npm dependency. Do not change the Clerk webhook handlers in this slice.
- Clerk-side session revocation on deactivation is out of scope; the frontend signs out locally. Retention and hard deletion remain undefined.
- Reuse `apps/web/DESIGN.md` tokens and existing components. No new palette or component library.
- Never delete files. Never run destructive commands. Ask for explicit approval before every commit; commit messages follow the repo's Conventional Commits rules.
- Markdown paragraphs stay on one physical line.

## Review Focus

- `PATCH /v1/me` carrying `role`, `email`, or an unknown key must fail with `VALIDATION_FAILED` and make no database write; accepting it would let a user promote themselves or diverge from Clerk. Pinned in Task 2 `TestUpdateMe_rejects_role_email_and_unknown_fields`.
- A request body with `user_id` or any client-supplied identity must not change whose record is read or written; a wrong guess here is a cross-tenant data breach. Pinned in Task 2 `TestGetMe_uses_principal_id` and `TestUpdateMe_ignores_client_supplied_user_id`.
- Invalid profile input (non-URL string in a URL field, whitespace-only name, over-long headline) must return field-level `VALIDATION_FAILED` and keep the form entries intact, not a 500 or a silent save. Pinned in Task 2 `TestUpdateMe_validates_fields` and Task 4 `validateAccountForm` tests.
- The deactivate confirm dialog's Cancel must make no API call, and Confirm must call the endpoint then sign out; an accidental deactivation is not recoverable here. Pinned in Task 4 `account-form.test.tsx`.
- `GET /v1/me` must not expose `clerk_user_id`; it is an integration identifier with no place in a browser response. Pinned in Task 2 `TestGetMe_uses_principal_id`.

---

### Task 1: Storage and usecase for the six editable fields

**Files:**
- Modify: `apps/backend/internal/models/user.go`
- Modify: `apps/backend/internal/repository/users.go`
- Modify: `apps/backend/internal/usecase/users/users.go`
- Regenerate: `apps/backend/internal/repository/mocks/users_mock.go`, `apps/backend/internal/usecase/users/mocks/user_mock.go`
- Test: `apps/backend/internal/usecase/users/users_test.go` (create), `apps/backend/internal/repository/users_test.go` (create)

**Interfaces:**
- Consumes: existing `models.User`, `repository.Users`, `usecase/users.User`.
- Produces: `models.UserProfileInput` with exactly six `*string` fields — `FullName`, `Headline`, `Phone`, `PortfolioURL`, `LinkedInURL`, `GitHubURL`, JSON names `full_name`, `headline`, `phone`, `portfolio_url`, `linkedin_url`, `github_url`. `repository.Users.UpdateProfile(ctx context.Context, id string, p models.UserProfileInput) (*models.User, error)` and the same signature on `usecase/users.User`.

- [ ] **Step 1: Write the failing usecase test**

In `apps/backend/internal/usecase/users/users_test.go`, add `TestUserService_UpdateProfile_delegates` and `TestUserService_UpdateProfile_propagates_error`. Use `repoMocks.NewMockUsers(ctrl)` from `internal/repository/mocks`. The first expects `UpdateProfile(gomock.Any(), "usr-1", input)` and asserts the returned `*models.User` equals the mock return. The second expects the same call returning `errors.New("db down")` and asserts the error propagates. Add a local `strPtr(s string) *string` helper.

- [ ] **Step 2: Run the test to verify it fails**

Run from `apps/backend`: `go test ./internal/usecase/users/... -run TestUserService_UpdateProfile -count=1`
Expected: compile failure — `UpdateProfile` undefined on `users.User` and `repository.Users`.

- [ ] **Step 3: Implement the type and methods**

In `internal/models/user.go` add `UserProfileInput` with the six `*string` fields and the exact JSON tags listed in Interfaces. Do not add `email`, `role`, or `clerk_user_id` to it.

Add `UpdateProfile` to the `repository.Users` and `usecase/users.User` interfaces. In `internal/repository/users.go`, implement it as one `UPDATE users SET full_name = $2, headline = $3, phone = $4, portfolio_url = $5, linkedin_url = $6, github_url = $7, updated_at = NOW() WHERE id = $1 RETURNING <the same columns as the existing Update>`. In `internal/usecase/users/users.go`, implement `UpdateProfile` as a direct delegation to the repository, matching the existing `Update` method shape. Leave the existing `Update` method untouched — the Clerk webhook still calls it.

- [ ] **Step 4: Regenerate the mocks**

Run from `apps/backend`:
`mockgen -source=internal/repository/users.go -destination=internal/repository/mocks/users_mock.go -package=mocks -mock_names=Users=MockUsers`
`mockgen -source=internal/usecase/users/users.go -destination=internal/usecase/users/mocks/user_mock.go -package=mocks -mock_names=User=MockUser`
Expected: both files now contain `UpdateProfile` recorder methods.

- [ ] **Step 5: Run the usecase tests**

Run: `go test ./internal/usecase/users/... -count=1`
Expected: PASS.

- [ ] **Step 6: Add the repository integration test**

In `internal/repository/users_test.go`, add `TestUsersRepository_UpdateProfile_persists_and_clears` using the existing `testJobDB(t)` helper from `jobs_test.go` (it skips when no test DSN is set). Create a user with a unique `clerk_user_id`, call `UpdateProfile` with all six fields set, assert each column on the returned record, then call `UpdateProfile` with an empty `UserProfileInput` and assert all six are nil while `email` is unchanged. This test is skipped locally without a DSN; do not claim it passed unless `JOBS_TEST_DATABASE_URL` or `DATABASE_URL` was set and the run was inspected.

- [ ] **Step 7: Verify and commit**

Run: `make test` from `apps/backend`. Expected: PASS, with the repository test reported as SKIP if no DSN. Commit only after explicit approval: `git add apps/backend/internal/models/user.go apps/backend/internal/repository/users.go apps/backend/internal/repository/mocks/users_mock.go apps/backend/internal/repository/users_test.go apps/backend/internal/usecase/users/users.go apps/backend/internal/usecase/users/mocks/user_mock.go apps/backend/internal/usecase/users/users_test.go` then `git commit -m "feat: add profile update path for own user record"`.

---

### Task 2: Clerk-protected `/v1/me` handler and contract docs

**Files:**
- Create: `apps/backend/internal/models/me.go`
- Create: `apps/backend/internal/deliveries/api/handler/me/me.go`
- Create: `apps/backend/internal/deliveries/api/handler/me/request.go`
- Modify: `apps/backend/internal/deliveries/api/handler/handler.go`
- Test: `apps/backend/internal/deliveries/api/handler/me/me_test.go` (create)
- Regenerate: `apps/backend/docs/swagger.json`, `apps/backend/docs/swagger.yaml`, `apps/backend/docs/docs.go`
- Modify: `docs/api/api-design.md`, `docs/database/users-data-model.md`

**Interfaces:**
- Consumes: `contract.Contract`, `ctxval.PrincipalFromContext`, `httpx.Envelope`/`RespondJSON`/`ParseJSON`, `middleware.AuthMiddleware.ValidClerkSession`, `usecase/users.User.FindByID` / `UpdateProfile` / `SoftDelete`, and `Task 1`'s `models.UserProfileInput`.
- Produces: `models.MeResponse` DTO with fields `id string`, `email`, `full_name`, `headline`, `phone`, `portfolio_url`, `linkedin_url`, `github_url` as `*string` with `omitempty`, `role models.Role`, `onboarding_completed_at *time.Time` with `omitempty`, `created_at time.Time`, `updated_at time.Time`. No `clerk_user_id` field. `me.NewHandler(c *contract.Contract) *handler` registering `GET /v1/me`, `PATCH /v1/me`, `POST /v1/me/deactivate` on the `/v1` group with `ValidClerkSession`.

- [ ] **Step 1: Write the failing handler tests**

In `handler/me/me_test.go`, follow the `handler/user_jobs/user_jobs_test.go` pattern: `newHandlerWithMock(ctrl)` builds `&handler{cfg: &config.Config{}, userUC: mockUser}` with `ucUsersMock.NewMockUser`, and `principalCtx(id)` returns `&models.Principal{ID: id, Role: models.RoleUser}`. Add these tests:

- `TestGetMe_uses_principal_id`: principal `"usr-1"`; a request to `/v1/me?user_id=usr-2` with no body; expect `FindByID(gomock.Any(), "usr-1")` only; assert status 200 and that the JSON data key `clerk_user_id` is absent and `id` is `usr-1`.
- `TestUpdateMe_rejects_role_email_and_unknown_fields`: table of bodies `{"full_name":"Jane","role":"admin"}`, `{"full_name":"Jane","email":"x@y.z"}`, and `{"full_name":"Jane","surprise":1}`; assert each returns 400 with error code `VALIDATION_FAILED`, no `UpdateProfile` call, and no database write.
- `TestUpdateMe_ignores_client_supplied_user_id`: a valid body with all six keys sent to `/v1/me?user_id=usr-2` while the principal is `"usr-1"`; expect `UpdateProfile(gomock.Any(), "usr-1", <input>)`; assert 200. Do not put `user_id` in the body — unknown body keys must be rejected by the earlier test.
- `TestUpdateMe_validates_fields`: table of invalid values — `portfolio_url: "not-a-url"`, `linkedin_url: "ftp://example.com"`, `full_name: "   "`, `headline:` a 257-character string, `phone:` a 65-character string — each with the other five keys present as `null`; assert 400, code `VALIDATION_FAILED`, and `error.details` contains a `field` equal to the offending JSON key.
- `TestUpdateMe_all_six_fields_required`: body with one of the six keys missing; assert 400 with a detail for the missing field.
- `TestDeactivateMe_soft_deletes_principal`: principal `"usr-1"`; expect `SoftDelete(gomock.Any(), "usr-1")`; assert status 200.

Run: `go test ./internal/deliveries/api/handler/me/... -count=1`
Expected: compile failure — package and types do not exist yet.

- [ ] **Step 2: Define the DTO and request validation**

In `internal/models/me.go` add `MeResponse` exactly as described in Interfaces, with swagger-friendly JSON tags.

In `handler/me/request.go` add `decodeUpdateMeRequest(r *http.Request) (models.UserProfileInput, *httpx.Error)`. Decode the body twice from a byte slice: first into `map[string]json.RawMessage` to confirm all six keys are present, then with `json.NewDecoder(bytes.NewReader(body)).DisallowUnknownFields()` into `models.UserProfileInput` so `role`, `email`, and unknown keys fail. Do not use `httpx.ParseJSON` here — it accepts unknown fields. Validation rules: a non-null string is trimmed; `full_name` and `headline` must be non-empty after trimming and at most 128 and 256 characters respectively; `phone` at most 64; each URL must parse, use scheme `http` or `https`, have a non-empty host, and be at most 2048 characters. `null` clears a field. On any failure return `httpx.Envelope(httpx.CodeInvalidRequest, "invalid account details", "")` with one `AddField(jsonKey, message)` per bad field.

- [ ] **Step 3: Implement the handlers and routes**

In `handler/me/me.go`, define `handler` with fields `cfg *config.Config` and `userUC ucUsers.User`; `NewHandler(c *contract.Contract) *handler` sets both from `c.Cfg` and `c.Usecase.Users`. `Route(g *httpx.Router)` builds `auth := middleware.NewAuthMiddleware(h.cfg, h.userUC)` and registers `g.GET("/me", h.GetMe, auth.ValidClerkSession)`, `g.PATCH("/me", h.UpdateMe, auth.ValidClerkSession)`, `g.POST("/me/deactivate", h.DeactivateMe, auth.ValidClerkSession)`.

`GetMe`, `UpdateMe`, and `DeactivateMe` each start with `p := ctxval.PrincipalFromContext(r.Context())`; if nil, return `httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")`. All usecase calls use `p.ID` only. `GetMe` maps the found user to `models.MeResponse`; map `constants.ErrUserNotFound` to a 404 envelope and any other error to `httpx.CodeInternal`. `UpdateMe` calls `decodeUpdateMeRequest`, returns its error before any write, then calls `UpdateProfile(ctx, p.ID, input)` and maps to the 200 response. `DeactivateMe` calls `SoftDelete(ctx, p.ID)` and maps to the 200 response. Add swag annotations to each method: `@Summary`, `@Description`, `@Tags me`, `@Accept json`, `@Produce json`, `@Param body models.UserProfileInput true "Editable account fields"` on `UpdateMe`, `@Success 200 {object} models.MeResponse`, `@Failure 400 {object} httpx.Error`, `@Failure 401 {object} httpx.Error`, `@security ApiKeyAuth`, and the exact `@Router` paths listed in Interfaces.

- [ ] **Step 4: Register the routes**

In `internal/deliveries/api/handler/handler.go`, import the new package as `me` and add `me.NewHandler(h.contract).Route(v1)` next to the other `v1` registrations. Do not create a new `Group("")` — the handler registers full `/me` paths on `v1`.

- [ ] **Step 5: Run the handler tests**

Run: `go test ./internal/deliveries/api/handler/me/... -count=1`
Expected: PASS.

- [ ] **Step 6: Regenerate Swagger and update the contract docs**

Run from `apps/backend`: `make generate-swagger`. Confirm `docs/swagger.json` contains `/v1/me` and `/v1/me/deactivate`. Never edit generated files by hand.

In `docs/api/api-design.md`, add the three routes under "Current user and profile": `GET /v1/me`, `PATCH /v1/me`, `POST /v1/me/deactivate`, with a note that all six editable fields are required on PATCH and `null` clears a field. In `docs/database/users-data-model.md`, change section 3 and section 6 so `full_name` is described as locally editable by the user, with Clerk seeding it only on `user.created`; add the note that the broken `user.updated` handler must be fixed to sync email only so it cannot overwrite a local edit. Do not change the webhook code.

- [ ] **Step 7: Verify and commit**

Run: `make test` and `make vet` from `apps/backend`. Expected: PASS. Commit only after explicit approval: `git add apps/backend/internal/models/me.go apps/backend/internal/deliveries/api/handler/me apps/backend/internal/deliveries/api/handler/handler.go apps/backend/docs docs/api/api-design.md docs/database/users-data-model.md` then `git commit -m "feat: add self-service account API"`.

---

### Task 3: Frontend API types, endpoints, and `useMe` hook

**Files:**
- Modify: `apps/web/lib/api/types.ts`
- Modify: `apps/web/lib/api/endpoints.ts`
- Create: `apps/web/hooks/use-me.ts`
- Test: `apps/web/tests/hooks/use-me.test.ts` (create)

**Interfaces:**
- Consumes: `apiClient` from `lib/api/client.ts`, Clerk `useAuth().getToken`, and the Task 2 response/request JSON shapes.
- Produces: `AccountUser` and `UpdateAccountInput` TypeScript interfaces; `getMe(token?: string | null): Promise<AccountUser>`, `updateMe(input: UpdateAccountInput, token?: string | null): Promise<AccountUser>`, `deactivateMe(token?: string | null): Promise<AccountUser>`; `useMe(explicitToken?: string | null)` returning `{ user, form, setField, loading, saving, error, status, save, deactivate, refetch }`.

- [ ] **Step 1: Write the failing hook tests**

In `tests/hooks/use-me.test.ts`, mock `@clerk/nextjs` with `useAuth: () => ({ getToken, isLoaded: true, signOut })` using `vi.hoisted` fns, and mock `@/lib/api/endpoints` with `getMe`, `updateMe`, `deactivateMe` as spies. Add:
- `Test useMe loads the current user` — `getMe` resolves to a sample `AccountUser`; `renderHook` then `waitFor`; assert `result.current.user.id` and that `form.full_name` mirrors the loaded value.
- `Test useMe.save sends all six fields` — call `setField("full_name", "New Name")`, then `await act(() => result.current.save())`; assert `updateMe` called once with an object containing all six keys and the new name; assert `status === "saved"`.
- `Test useMe.save keeps a failed submission` — `updateMe` rejects with `new Error("network")`; assert `error.message === "network"`, `status === "idle"`, and `form.full_name` unchanged.
- `Test useMe.deactivate signs out` — `deactivateMe` resolves; assert `signOut` called once.

Run: `cd apps/web && npm run test -- tests/hooks/use-me.test.ts`
Expected: FAIL — modules not found.

- [ ] **Step 2: Add the types and endpoints**

In `lib/api/types.ts`, add `AccountUser` with exactly the `MeResponse` JSON fields from Task 2 (`role: "user" | "admin"`, timestamps as `string | null` where nullable) and `UpdateAccountInput` with the six `string | null` fields. In `lib/api/endpoints.ts`, add the three functions using `apiClient<AccountUser>` with paths `/v1/me`, `/v1/me` with `method: "PATCH"` and `body: JSON.stringify(input)`, and `/v1/me/deactivate` with `method: "POST"`, each forwarding `token`.

- [ ] **Step 3: Implement the hook**

In `hooks/use-me.ts`, follow `hooks/use-user-jobs.ts`: `const { getToken, isLoaded, signOut } = useAuth()`; resolve the token with `explicitToken !== undefined ? explicitToken : await getToken()`. `load` sets `user`, maps it into `form`, clears `error`, and guards state updates with a `mountedRef`. `setField(field, value)` updates one form key. `save` sets `saving`, calls `updateMe(form, token)`, replaces `user` and `form` from the response, sets `status` to `"saved"`, and on failure sets `error` without touching `form` or `status`. `deactivate` calls `deactivateMe(token)` then `signOut()`. Expose `refetch: load`.

- [ ] **Step 4: Run the hook tests**

Run: `cd apps/web && npm run test -- tests/hooks/use-me.test.ts`
Expected: PASS.

- [ ] **Step 5: Verify and commit**

Run: `cd apps/web && npm run typecheck`. Expected: PASS. Commit only after explicit approval: `git add apps/web/lib/api/types.ts apps/web/lib/api/endpoints.ts apps/web/hooks/use-me.ts apps/web/tests/hooks/use-me.test.ts` then `git commit -m "feat: add account API client and hook"`.

---

### Task 4: Settings section, account form, and navbar link

**Files:**
- Create: `apps/web/app/(app)/settings/layout.tsx`
- Create: `apps/web/app/(app)/settings/page.tsx`
- Create: `apps/web/app/(app)/settings/account/page.tsx`
- Create: `apps/web/components/settings/section-nav.tsx`
- Create: `apps/web/components/settings/account-form.tsx`
- Create: `apps/web/lib/account-validation.ts`
- Modify: `apps/web/components/layout/navbar.tsx`
- Modify: `apps/web/tests/setup.ts`
- Test: `apps/web/tests/components/settings/account-form.test.tsx` (create)

**Interfaces:**
- Consumes: `useMe` from Task 3, `AccountUser` / `UpdateAccountInput` from Task 3, `usePathname`, Clerk `useAuth().signOut`, existing `DESIGN.md` tokens.
- Produces: `validateAccountForm(form: UpdateAccountInput): Record<string, string>` with keys using the six JSON field names; `AccountForm` client component; a `/settings` section layout with a single-item side nav; a `Settings` link in the navbar.

- [ ] **Step 1: Write the failing validation tests**

In `tests/components/settings/account-form.test.tsx`, add direct tests for `validateAccountForm`: a valid form returns `{}`; `"not-a-url"` in `portfolio_url` returns a keyed error; a whitespace-only `full_name` returns a keyed error; a valid `null` field is accepted.

Run: `cd apps/web && npm run test -- tests/components/settings/account-form.test.tsx`
Expected: FAIL — module not found.

- [ ] **Step 2: Implement the validator**

In `lib/account-validation.ts`, export `validateAccountForm` mirroring the Task 2 backend rules: trimmed non-empty `full_name` and `headline`, lengths 128 and 256, `phone` at most 64 after trimming, URLs absolute `http`/`https` with a host and at most 2048 characters, `null` always valid. Return a map keyed by the field's JSON name.

- [ ] **Step 3: Add the form behavior tests**

In the same test file, mock `@/hooks/use-me` with `vi.hoisted` spies for `setField`, `save`, `deactivate`, and a controllable `form`. Add:
- renders all six labeled inputs plus read-only email with a "Managed by Clerk" hint;
- clicking Save with an invalid URL shows the field error and does not call `save`;
- when `status === "saved"` a `role="status"` element is shown;
- when `error` is set, a `role="alert"` element is shown and the input values remain;
- the deactivate Cancel button makes no `deactivate` call, while Confirm calls `deactivate` once.

Run: `cd apps/web && npm run test -- tests/components/settings/account-form.test.tsx`
Expected: FAIL until the component exists.

- [ ] **Step 4: Build the settings section and account form**

`app/(app)/settings/layout.tsx` renders a `max-w-6xl` container with `aside` holding `<SectionNav />` and a content column; on mobile the `aside` stacks above the content. `components/settings/section-nav.tsx` uses `usePathname()` and renders exactly one link, "Account" → `/settings/account`, highlighted when active. Do not render placeholder links for future pages. `app/(app)/settings/page.tsx` calls `redirect("/settings/account")` from `next/navigation`. `app/(app)/settings/account/page.tsx` is a thin server component rendering `<AccountForm />`.

`components/settings/account-form.tsx` is `"use client"` and uses `useMe()`. Render a heading "Account", an "Account details" group with six labeled text inputs bound to `form` via `setField`, a read-only email value with the "Managed by Clerk" hint, and a read-only metadata line for role, onboarding completion, and member-since. Validate on submit with `validateAccountForm`; show each error under its own field and skip `save` when any error exists. Show a `role="status"` success message when `status === "saved"` and a `role="alert"` error when `error` is set. Add an "Account state" group with a "Deactivate account" button opening a confirm dialog (`role="dialog"`, `aria-modal="true"`) with Cancel and Confirm; focus Cancel on open; Cancel closes without calling `deactivate`, Confirm calls it. Use existing `DESIGN.md` tokens and classes only.

- [ ] **Step 5: Add the navbar link and test mock**

In `components/layout/navbar.tsx`, add a "Settings" link to `/settings` beside "Explore Jobs", using the same active-state classes and a `lucide-react` `Settings` icon; mark it active when `pathname?.startsWith("/settings")`. In `tests/setup.ts`, add `signOut: vi.fn().mockResolvedValue(undefined)` to the `useAuth` mock. Update `tests/components/layout/navbar.test.tsx` to assert the Settings link exists.

- [ ] **Step 6: Run the frontend checks**

Run: `cd apps/web && npm run test -- tests/components/settings/account-form.test.tsx tests/components/layout/navbar.test.tsx`
Expected: PASS. Then `npm run typecheck`, `npm run lint`, `npm run test`. Expected: all PASS.

- [ ] **Step 7: Verify and commit**

Run from the repo root: `make web-check`. Expected: PASS. Commit only after explicit approval: `git add apps/web/app/'(app)'/settings apps/web/components/settings apps/web/lib/account-validation.ts apps/web/components/layout/navbar.tsx apps/web/tests/setup.ts apps/web/tests/components/settings/account-form.test.tsx apps/web/tests/components/layout/navbar.test.tsx` then `git commit -m "feat: add self-service account settings page"`.
