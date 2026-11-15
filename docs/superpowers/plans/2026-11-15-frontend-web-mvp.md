# Job Match Report & Resume Web App Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a focused, modern Next.js 15+ web application in `apps/web` authenticated via Clerk, dedicated to exploring job opportunities, reading detailed AI match evaluation reports, and downloading tailored resumes.

**Architecture:** App Router with minimal layout shell (no dashboard bloat). Authenticated root `/` serves the instant-searchable Jobs List. `/jobs/[slug]` serves the full-width Job Match Report Workspace with header controls and 3 segmented tabs: Job Details, Match Report, and Tailored Resume. Backed by typed Go API client with a pluggable local mock adapter.

**Tech Stack:** Next.js 15+ (App Router), React 19, TypeScript, Tailwind CSS, Lucide React, `@clerk/nextjs`, Vitest, `@testing-library/react`.

**Spec:** [docs/superpowers/specs/2026-11-15-frontend-web-mvp-design.md](file:///Users/nyomanfrastyawan/Documents/dev/frasnym/homelab-01/apps/getojobu/docs/superpowers/specs/2026-11-15-frontend-web-mvp-design.md)

---

## Global Constraints

- Pinned toolchain: Node 24+, standard `npm` package manager.
- Directory location: strictly inside `apps/web`.
- No direct connection to PostgreSQL/Neon from Next.js. All data access occurs via Go backend `/v1/*` or `lib/api/mock-adapter.ts`.
- Auth integration: `@clerk/nextjs` with session protection via `middleware.ts`.
- Design aesthetics: Dark/light compatible, zinc neutral palette, high contrast, crisp typography, responsive layout.
- Per `AGENTS.md`: Never delete files without explicit confirmation; get explicit approval before creating git commits.

---

## Review Focus

1. **Unauthenticated Route Gating:** Accessing `/` or `/jobs/[slug]` without an active Clerk session redirects cleanly to `/sign-in`.
2. **Unevaluated Job Resilience:** Opening `/jobs/[slug]` when `latest_evaluation` is null renders a clean "Not yet evaluated" state on Match Report and Resume tabs with an active "Analyze Compatibility" trigger, without crashing.
3. **Regex/Special Character Search:** Typing characters like `c++`, `(senior)`, or `[remote]` in the search bar does not throw an uncaught RegExp error.
4. **Ineligible Match Presentation:** When `eligibility_status === "ineligible"`, the match badge displays an amber/red warning and clearly displays unmet mandatory requirements.
5. **Presigned Download Failure Handling:** If a presigned URL request fails or expires, Tab 3 presents a clear retry toast/error message instead of an unhandled rejection.

---

## Tasks

### Task 1: Project Scaffolding & Setup (`apps/web`)

**Files:**
- Create: `apps/web/package.json`
- Create: `apps/web/tsconfig.json`
- Create: `apps/web/next.config.ts`
- Create: `apps/web/postcss.config.mjs`
- Create: `apps/web/tailwind.config.ts`
- Create: `apps/web/app/globals.css`
- Create: `apps/web/vitest.config.ts`
- Create: `apps/web/tests/setup.ts`
- Create: `apps/web/.env.example`

**Interfaces:**
- Produces: Working Next.js development and test toolchain with Tailwind styling and Clerk SDK installed.

- [ ] **Step 1: Write `package.json` with required dependencies**
  Configure `next`, `react`, `react-dom`, `@clerk/nextjs`, `lucide-react`, `clsx`, `tailwind-merge`, and devDependencies (`typescript`, `tailwindcss`, `vitest`, `@testing-library/react`, `@vitejs/plugin-react`, `jsdom`).
- [ ] **Step 2: Configure TypeScript, Tailwind, PostCSS, and Globals CSS**
  Create `tsconfig.json` with `@/*` path aliases, `tailwind.config.ts` with dark mode `class` and zinc theme tokens, and `app/globals.css` with CSS variables.
- [ ] **Step 3: Configure Vitest and Test Setup**
  Create `vitest.config.ts` and `tests/setup.ts` with `@testing-library/jest-dom` matchers.
- [ ] **Step 4: Write failing baseline smoke test**
  Create `apps/web/tests/smoke.test.ts` asserting basic environment test setup.
- [ ] **Step 5: Run test to verify test harness works**
  Run: `cd apps/web && npm install && npm test`
  Expected: PASS
- [ ] **Step 6: Verify Next.js build configuration**
  Run: `cd apps/web && npx next info`

---

### Task 2: API Client, Type Definitions, and Mock Adapter

**Files:**
- Create: `apps/web/lib/api/types.ts`
- Create: `apps/web/lib/api/client.ts`
- Create: `apps/web/lib/api/mock-adapter.ts`
- Create: `apps/web/lib/api/endpoints.ts`
- Test: `apps/web/tests/lib/api/client.test.ts`

**Interfaces:**
- Produces:
  ```ts
  export interface Job {
    id: number;
    slug: string;
    status: string;
    title: string;
    company: string;
    location: string;
    description?: string;
    apply_url?: string;
    is_remote?: boolean;
    posted_date?: string;
    created_at: string;
    latest_evaluation?: EvaluationSummary | null;
  }

  export interface EvaluationSummary {
    id: string;
    evaluation_status: 'new' | 'pending' | 'success' | 'failed';
    overall_score?: number;
    eligibility_status?: string;
    recommendation_label?: string;
    report_status?: string;
    resume_status?: string;
    created_at?: string;
  }

  export interface MatchReport {
    id: string;
    user_job_id: string;
    evaluation_status: string;
    report_status?: string;
    overall_score?: number;
    eligibility_status?: string;
    recommendation_label?: string;
    report_data?: {
      summary?: string;
      strengths?: string[];
      gaps?: string[];
      cv_improvements?: string[];
    };
    evaluation_data?: {
      evaluations: Array<{
        category: string;
        score: number;
        reason: string;
        strengths?: string[];
        gaps?: string[];
        evidence?: Array<{
          candidate_evidence: string;
          job_requirement: string;
          relationship: string;
        }>;
      }>;
      mandatory_requirements: Array<{
        requirement: string;
        status: string;
        reason: string;
      }>;
    };
  }

  export interface DownloadResumeResponse {
    download_url: string;
    expires_at: string;
    filename: string;
  }
  ```
- Methods:
  - `getJobs(): Promise<Job[]>`
  - `getJobBySlug(slug: string): Promise<Job>`
  - `triggerJobCompatibility(slug: string, force?: boolean): Promise<EvaluationSummary>`
  - `getMatchReport(evalId: string): Promise<MatchReport>`
  - `getResumeDownloadUrl(evalId: string): Promise<DownloadResumeResponse>`

- [ ] **Step 1: Write the failing tests for API endpoints & mock adapter**
  Create `apps/web/tests/lib/api/client.test.ts` testing `getJobs`, `getJobBySlug`, `getMatchReport`, and `getResumeDownloadUrl`.
- [ ] **Step 2: Run test to verify it fails**
  Run: `cd apps/web && npx vitest run tests/lib/api/client.test.ts`
  Expected: FAIL (modules not found).
- [ ] **Step 3: Implement `types.ts`, `mock-adapter.ts`, `client.ts`, and `endpoints.ts`**
  Implement mock adapter with realistic job data (Stripe Go engineer, Vercel frontend, etc.) and fallback mechanism when `NEXT_PUBLIC_ENABLE_MOCK === "true"` or real backend is unavailable.
- [ ] **Step 4: Run test to verify it passes**
  Run: `cd apps/web && npx vitest run tests/lib/api/client.test.ts`
  Expected: PASS

---

### Task 3: Clerk Authentication & App Navigation Shell

**Files:**
- Create: `apps/web/middleware.ts`
- Create: `apps/web/app/layout.tsx`
- Create: `apps/web/app/(auth)/sign-in/[[...sign-in]]/page.tsx`
- Create: `apps/web/app/(auth)/sign-up/[[...sign-up]]/page.tsx`
- Create: `apps/web/app/(app)/layout.tsx`
- Create: `apps/web/components/layout/navbar.tsx`
- Test: `apps/web/tests/components/layout/navbar.test.tsx`

**Interfaces:**
- Consumes: `@clerk/nextjs` (`ClerkProvider`, `SignIn`, `SignUp`, `UserButton`, `clerkMiddleware`).
- Produces: Authenticated top navigation header with logo, title, and profile button; route protection for `/(app)/*`.

- [ ] **Step 1: Write failing test for `Navbar` component**
  Create `tests/components/layout/navbar.test.tsx` asserting logo, branding ("getojobu"), and navigation links render correctly.
- [ ] **Step 2: Run test to verify it fails**
  Run: `cd apps/web && npx vitest run tests/components/layout/navbar.test.tsx`
  Expected: FAIL.
- [ ] **Step 3: Implement `middleware.ts`, `app/layout.tsx`, `(auth)` pages, and `Navbar`**
  Wrap root layout with `ClerkProvider`. Implement `clerkMiddleware` protecting `/(app)(.*)` routes. Build clean `Navbar` with dark/light styling and Clerk's `<UserButton />`.
- [ ] **Step 4: Run test to verify it passes**
  Run: `cd apps/web && npx vitest run tests/components/layout/navbar.test.tsx`
  Expected: PASS

---

### Task 4: Jobs List View with Search & Status Filters

**Files:**
- Create: `apps/web/components/jobs/job-card.tsx`
- Create: `apps/web/components/jobs/job-filters.tsx`
- Create: `apps/web/hooks/use-jobs.ts`
- Create: `apps/web/app/(app)/page.tsx`
- Test: `apps/web/tests/components/jobs/job-card.test.tsx`
- Test: `apps/web/tests/hooks/use-jobs.test.ts`

**Interfaces:**
- Consumes: `Job`, `getJobs()` from `lib/api/endpoints.ts`.
- Produces: Instant-filter search bar, status filter pills (`All`, `Analyzed`, `Pending`, `Failed`), job cards with match score badges and company metadata.

- [ ] **Step 1: Write failing tests for `JobCard` and `useJobs` hook**
  Assert job title, company, location, remote tag, match score badge rendering, and client-side search query filtering (including regex special characters like `c++`).
- [ ] **Step 2: Run tests to verify they fail**
  Run: `cd apps/web && npx vitest run tests/components/jobs/job-card.test.tsx tests/hooks/use-jobs.test.ts`
  Expected: FAIL.
- [ ] **Step 3: Implement `use-jobs.ts`, `job-card.tsx`, `job-filters.tsx`, and `app/(app)/page.tsx`**
  Handle safe string normalization for search filtering. Render empty states and loading skeleton cards.
- [ ] **Step 4: Run tests to verify they pass**
  Run: `cd apps/web && npx vitest run tests/components/jobs/job-card.test.tsx tests/hooks/use-jobs.test.ts`
  Expected: PASS

---

### Task 5: Job Match Report Page Shell & Header Navigation

**Files:**
- Create: `apps/web/components/jobs/job-report-header.tsx`
- Create: `apps/web/hooks/use-job-detail.ts`
- Create: `apps/web/app/(app)/jobs/[slug]/page.tsx`
- Test: `apps/web/tests/components/jobs/job-report-header.test.tsx`

**Interfaces:**
- Consumes: `Job`, `getJobBySlug(slug)` from `lib/api/endpoints.ts`.
- Produces: Dedicated report page layout at `/jobs/[slug]`, header with back button, score badge, `[← Prev / Next →]` switcher, and tab switcher (`Job Details`, `Match Report`, `Tailored Resume`) synchronized with URL query `?tab=`.

- [ ] **Step 1: Write failing test for `JobReportHeader`**
  Assert breadcrumbs, company/title, match score badge, prev/next buttons, and tab selection change events.
- [ ] **Step 2: Run test to verify it fails**
  Run: `cd apps/web && npx vitest run tests/components/jobs/job-report-header.test.tsx`
  Expected: FAIL.
- [ ] **Step 3: Implement `use-job-detail.ts`, `job-report-header.tsx`, and `app/(app)/jobs/[slug]/page.tsx`**
  Implement tab state management with `useSearchParams` (`tab` defaults to `report` if evaluated, or `details` if unevaluated).
- [ ] **Step 4: Run test to verify it passes**
  Run: `cd apps/web && npx vitest run tests/components/jobs/job-report-header.test.tsx`
  Expected: PASS

---

### Task 6: Tab 1 - Job Details Component

**Files:**
- Create: `apps/web/components/jobs/job-detail-tab.tsx`
- Test: `apps/web/tests/components/jobs/job-detail-tab.test.tsx`

**Interfaces:**
- Consumes: `job: Job`
- Produces: Formatted role description, company details, posted date, remote/visa attributes, and direct apply external link.

- [ ] **Step 1: Write failing test for `JobDetailTab`**
  Assert job description whitespace/paragraphs, role metadata, and external apply URL link with `target="_blank" rel="noopener noreferrer"`.
- [ ] **Step 2: Run test to verify it fails**
  Run: `cd apps/web && npx vitest run tests/components/jobs/job-detail-tab.test.tsx`
  Expected: FAIL.
- [ ] **Step 3: Implement `job-detail-tab.tsx`**
  Add clean typography, key role pill tags, and fallback for empty descriptions.
- [ ] **Step 4: Run test to verify it passes**
  Run: `cd apps/web && npx vitest run tests/components/jobs/job-detail-tab.test.tsx`
  Expected: PASS

---

### Task 7: Tab 2 - Match Report View (Gauges, Categories, Evidence, Gaps)

**Files:**
- Create: `apps/web/components/matching/match-score-gauge.tsx`
- Create: `apps/web/components/matching/category-accordion.tsx`
- Create: `apps/web/components/matching/evidence-list.tsx`
- Create: `apps/web/components/matching/strengths-gaps.tsx`
- Create: `apps/web/components/matching/match-report-tab.tsx`
- Test: `apps/web/tests/components/matching/match-report-tab.test.tsx`

**Interfaces:**
- Consumes: `evalId?: string`, `slug: string`, `getMatchReport(evalId)`.
- Produces: Comprehensive AI compatibility report:
  - Overall score gauge (circular or bold progress bar with percentage).
  - Eligibility badge (`Eligible` / `Ineligible` / `Strong Match`).
  - Mandatory requirement met/not met indicators.
  - Multi-category breakdown (Score, strengths, gaps).
  - Evidence citations mapping candidate proof to job requirements (Direct vs. Transferable tags).

- [ ] **Step 1: Write failing test for `MatchReportTab`**
  Assert rendering of score gauge (e.g. 88%), category score list, evidence citations, strengths/gaps, and handling of unevaluated jobs ("Not yet evaluated" + "Analyze Compatibility" button).
- [ ] **Step 2: Run test to verify it fails**
  Run: `cd apps/web && npx vitest run tests/components/matching/match-report-tab.test.tsx`
  Expected: FAIL.
- [ ] **Step 3: Implement score gauge, category accordion, evidence list, and report tab**
  Implement responsive visual widgets with high-contrast text and clear status colors (green/emerald for met/direct, amber/rose for gaps/transferable).
- [ ] **Step 4: Run test to verify it passes**
  Run: `cd apps/web && npx vitest run tests/components/matching/match-report-tab.test.tsx`
  Expected: PASS

---

### Task 8: Tab 3 - Tailored Resume View & PDF Download

**Files:**
- Create: `apps/web/components/resume/resume-tab.tsx`
- Create: `apps/web/components/resume/download-resume-button.tsx`
- Test: `apps/web/tests/components/resume/resume-tab.test.tsx`

**Interfaces:**
- Consumes: `evalId?: string`, `resumeStatus?: string`, `reportData?: { cv_improvements?: string[] }`, `getResumeDownloadUrl(evalId)`.
- Produces: Generation status badge, AI tailored CV recommendations & targeted bullet points, and a prominent download button triggering presigned URL retrieval from `/v1/matching-evaluations/{id}/resume/download`.

- [ ] **Step 1: Write failing test for `ResumeTab`**
  Assert generation status badge (`ready`, `generating`, `failed`), CV improvement suggestions list, and download button calling presigned URL handler.
- [ ] **Step 2: Run test to verify it fails**
  Run: `cd apps/web && npx vitest run tests/components/resume/resume-tab.test.tsx`
  Expected: FAIL.
- [ ] **Step 3: Implement `download-resume-button.tsx` and `resume-tab.tsx`**
  Handle loading states, download error alerts, and clean presentation of tailored CV recommendations.
- [ ] **Step 4: Run test to verify it passes**
  Run: `cd apps/web && npx vitest run tests/components/resume/resume-tab.test.tsx`
  Expected: PASS

---

### Task 9: End-to-End Build & Lint Verification

**Files:**
- Modify/Verify: All `apps/web` files

**Interfaces:**
- Produces: Clean, production-ready build passing all tests and TypeScript compiler checks.

- [ ] **Step 1: Run complete test suite**
  Run: `cd apps/web && npm test`
  Expected: ALL PASS.
- [ ] **Step 2: Run TypeScript typechecker**
  Run: `cd apps/web && npx tsc --noEmit`
  Expected: Exit code 0, no type errors.
- [ ] **Step 3: Run production Next.js build**
  Run: `cd apps/web && npm run build`
  Expected: Build succeeds with static/server routes compiled.
- [ ] **Step 4: Verify Mock and Live environment variables**
  Verify `.env.example` documents `NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY`, `CLERK_SECRET_KEY`, `NEXT_PUBLIC_API_URL`, and `NEXT_PUBLIC_ENABLE_MOCK`.
