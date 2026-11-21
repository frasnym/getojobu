# User preferences self-service UI — design

Date: 2026-11-20

## Intent

Let a signed-in user manage their own job-search preference sets (`user_preferences`) from Settings. Today the table is read only, from backend scraping code; there is no API and no UI. The page must let a user add, edit, and remove search sets, and control auto-hide of low-match evaluations.

## Decisions

These are settled; they do not need further input.

- Audience: the authenticated user only. No admin screen, no cross-user access.
- A user may have several preference sets. `boards` is an attribute of a set, not part of its identity. No unique index is kept, so duplicate sets are allowed and the API does not reject them.
- Auto-hide is a **user-level** setting. It moves from `user_preferences` to `users`, and the UI shows one switch, not one per set.
- UI/UX approach is the builder's call per user direction "adjust best UI UX", detailed below.
- `is_active` is removed. It currently filters scraping queries, backs the unique index, and gates the auto-hide check, so all of those change with it. Any old `is_active = FALSE` row becomes live; the application database is fresh, so this is expected to affect no data.

## Goals

- User can create, read, update, and delete only their own preference sets.
- Scraping keeps working with the simplified schema.
- Locations and boards are chosen, not typed, so values always match what the worker understands.
- API contract lives in OpenAPI generated from Go comments; the TypeScript client follows it.
- UI follows the existing Settings visual language and is keyboard accessible.

## Non-goals / anti-goals

- No admin or multi-user management view.
- No external job submission, scraping, or workflow trigger from this page.
- No hard-coded board list in the frontend; values come from the API catalog.
- No Supabase-specific auth or RLS assumptions.

## Constraints

- Go backend: `net/http` + method-aware `http.ServeMux`, Clerk-verified principal only, all user-owned reads/writes scoped to the principal's internal user id.
- New routes under `/v1/user-preferences`, Clerk auth per route.
- Swagger/OpenAPI regenerated with `make generate-swagger`; `docs/swagger.json`/`.yaml` are never hand-edited.
- Frontend: Next.js + TypeScript, existing Tailwind theme, Clerk token from `useAuth`.
- Shared `components/settings/section-nav.tsx` gains one link, affecting every Settings page.

## Shared scrape catalog

One canonical file: `config/scrape-catalog.json` at the `apps/getojobu` repo root. It holds:

- `locations`: value, label, and optional `aliases` map for output normalization (the current `LOCATION_MAP` contents plus `united states`, 24 country-only entries). `united states` must be added because `linkedin`/`glassdoor` need it as a search location even though the current output-normalization map has no US row. A board absent from `aliases` uses the raw location string, matching the current fallback.
- `boards`: value, label, and location behavior. `linkedin` and `glassdoor` accept any catalog location. `arbeitnow` accepts only `germany`, `united kingdom`, `france`, `netherlands`, `switzerland` (its `HOSTS_BY_LOCATION`). `remoteusa` ignores location entirely and only returns jobs whose `applicantLocationRequirements.name` is `United States`; its UI label makes the US-only scope explicit and the location picker is hidden when it is selected.

The location behavior lives on boards, not in the alias map, because the two mechanisms are different: aliases normalize returned job locations in `enrich_jobs`, while board location behavior controls which locations a user may pick and what the scraper accepts as input.

## Scrape payload interface

`normalizePreferences` is removed, along with its board-agnostic triple loop. Board-specific logic moves into a Go interface under `internal/usecase/jobs/scrape`:

```go
type JobScrape interface {
    Board() string
    GenerateWorkflowsPayload(prefs []models.UserPreference) []models.JobScrapeWorkflowInput
}
```

Each board has its own implementation, and each implementation owns its location rules:

- `LinkedInScrape`: one payload per keyword × any catalog location.
- `GlassdoorScrape`: one payload per keyword × any catalog location.
- `ArbeitnowScrape`: one payload per keyword × only the five countries it supports.
- `RemoteUsaScrape`: one payload per keyword with a fixed location; the stored location is ignored, so it no longer runs once per location.

There is no registry. `jobService` holds a plain `[]JobScrape`, built once in `NewJobService`. `StartJobScrape` ranges over that slice, gathers payloads, sorts deterministically, and starts one workflow per payload. It contains no board-specific branching. A lookup-by-board abstraction would add indirection without a second consumer; if one appears later, it can be added then.

Workflow IDs and search IDs are built by one shared unexported helper so the implementations stay small and consistent. IDs keep the current `job-scrape-{board}-{keyword}-{location}` format. Adding a board later means adding one implementation and one entry in the slice, not editing the scrape usecase.

Repository/usecase wiring: the catalog is loaded fail-fast in `contract.New`, attached to the repository contract, and passed to `NewJobService`, which builds the slice. Only `ArbeitnowScrape` needs catalog data (its five allowed countries); the other implementations are pure. `IsAutoHideLowMatchEnabled` and the preference CRUD methods stay on the repository as described above.

How both services read it:

- `docker-compose.yml` bind-mounts the file read-only into `go-api` and `python-worker` at `/app/config/scrape-catalog.json`, and sets `SCRAPE_CATALOG_PATH` in both.
- Go config gains `SCRAPE_CATALOG_PATH`, defaulting to `../../config/scrape-catalog.json` for local `go run` from `apps/backend`. A small catalog package loads and validates it at startup and fails fast on a missing or invalid file.
- Python `config.load()` reads the same env var, defaulting to `../../config/scrape-catalog.json` when run from `workers/python`, and builds `LOCATION_MAP` from the file.
- No generator, no derived copies, no sync target, no Dockerfile change. `docker-compose.integration.yml` only starts PostgreSQL, so it needs no change.
- The compose edit is deployment-affecting; show the exact diff and get approval before applying.

The API serves the catalog via `GET /v1/user-preferences/options` and validates create/update against it.

## Data model

New migration `apps/backend/migrations/000021_user_preferences_ui.sql`:

- `user_preferences`: drop `idx_user_preferences_user_active`, drop the partial `idx_user_preferences_unique`, and drop `is_active`. No replacement index.
- `users`: add `auto_hide_low_match_evaluation BOOLEAN NOT NULL DEFAULT FALSE`.
- Down migration restores the column and both original partial indexes.

Model changes:

- `models.UserPreference` drops `IsActive`.
- `models.User` and `MeResponse` gain `AutoHideLowMatchEvaluation bool` with JSON `auto_hide_low_match_evaluation`.
- `UserProfileInput` gains `AutoHideLowMatchEvaluation *bool` with `json:"auto_hide_low_match_evaluation,omitempty"` so existing PATCH callers are unaffected; `nil` means "leave unchanged".

Repository changes:

- `GetActivePreferences` becomes `ListByUserIDs`: it now returns all rows for the given users. Update interface, mock, scraping usecase, and tests.
- `IsAutoHideLowMatchEnabled(ctx, userID)` keeps its signature but reads from `users`; the matching usecase stays untouched.
- New methods on `UserPreferences`: `ListByUserID`, `Create`, `Update`, `Delete`. Update and Delete filter by both `id` and `user_id`; zero rows affected maps to not-found.

## Validation

- Item count cap: 20 per array. Item length cap: 80 characters. Enforced server-side; the UI mirrors the same numbers for inline feedback.
- Trim every value; reject empty entries; require at least one item in `keywords`, `locations`, and `boards`.
- `keywords` are free text. `locations` and `boards` must exist in the catalog.
- `user_id` is never accepted from a client.

## API

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/v1/user-preferences` | List the caller's sets |
| POST | `/v1/user-preferences` | Create a set |
| PATCH | `/v1/user-preferences/{id}` | Update an owned set |
| DELETE | `/v1/user-preferences/{id}` | Delete an owned set |
| GET | `/v1/user-preferences/options` | Supported locations and boards |

Request fields: `keywords`, `locations`, `boards` (string arrays) and `auto_hide_low_match_evaluation` is not part of a set — it is edited through `/v1/me`. Response DTO omits `user_id`.

Handlers are thin and use a new `usecase/userpreferences` service. Request/response models carry Swagger annotations.

## Frontend UX

- New route `/settings/preferences` rendering `PreferencesManager`; add a "Preferences" link to the Settings sidebar.
- Page-level "Auto-hide low-match jobs" switch at the top, saved through `PATCH /v1/me`, with one line of helper text. It applies to all the user's jobs.
- Below it, a list of search sets. Each set is a card with three groups: Keywords (free-text chips, Enter or comma to add), Locations (select from catalog options), Boards (select from catalog options, multi-select). Chips and selections are removable.
- Editing is inline per card (Edit → Save/Cancel). Adding a set appends a new editable empty card. Deleting uses a small confirm dialog consistent with the existing account deactivate dialog.
- States: loading skeleton, empty state with "Add your first search set" CTA, inline validation, error banner with retry, saving indicator with controls disabled, saved confirmation.
- Styling reuses existing theme classes (`bg-card`, `border-border`, `text-muted-foreground`, `primary`) and the account form's input/button patterns. Layout stacks on mobile and uses a grid on wider screens.
- New `useUserPreferences` hook mirrors `useMe`; types and endpoint functions go in `lib/api/types.ts` and `lib/api/endpoints.ts`; `AccountUser` and `UpdateAccountInput` gain the optional auto-hide field.

## Files touched (expected)

- Root catalog file and compose edit.
- Migration, models, repository + mock + tests, users/me paths for auto-hide.
- New config catalog package and new `usecase/jobs/scrape` package with the interface, four implementations, and their tests; `start_job_scrape.go` and `normalizePreferences` tests replaced.
- New preferences usecase package, handler + DTOs + tests, contract wiring, route registration, regenerated Swagger.
- Frontend: new page, new component, new hook, nav link, types, endpoints, tests.

## Verification

- Go: `make test`, `make vet`, and integration tests where applicable.
- Python: `make test`, `make lint`.
- Web: `make web-check` (typecheck, lint, test).
- Confirm Swagger diff is generated, not hand-written.
- Confirm no route or SQL path reaches another user's row.

## Risks / approvals needed

- Dropping `is_active` makes any pre-existing inactive rows active. No production migration is run as part of this work.
- `docker-compose.yml` changes are deployment-affecting; approval required before editing.
- Shared Settings nav is modified, so all Settings pages gain the new link.
- No commit, push, or deploy without separate approval.
