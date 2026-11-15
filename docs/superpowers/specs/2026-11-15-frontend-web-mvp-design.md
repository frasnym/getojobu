# Design Document: Job Match Report & Resume Web App (go-get-job)

**Date:** 2026-11-15  
**Topic:** Frontend Web Application for Job Search, AI Match Evaluation Reports, and Tailored Resumes  
**Status:** Ready for Approval  
**Target Directory:** `apps/web`

---

## 1. Intent & Context

`go-get-job` discovers job listings, evaluates compatibility against candidate profiles using AI, generates tailored application materials (resumes and reports), and tracks applications.

This project delivers the frontend web application in `apps/web`, built on Next.js + TypeScript, authenticated via Clerk, adhering to the repository architecture defined in `docs/architecture/structure-2.md` and repository guidelines in `AGENTS.md`.

This application is purpose-built as a **Job Match Report & Resume Tool**. It intentionally excludes generic dashboard bloat (no vanity charts, widgets, or overview metric screens), focusing 100% on reviewing matched jobs, reading rich AI evaluation reports, and downloading tailored resumes.

---

## 2. Goals, Non-Goals, and Anti-Goals

### Goals
- **Authentication**: Clerk sign-in / sign-up protecting authenticated routes.
- **Jobs List (`/`)**: High-contrast, clean, instant client-side searchable list of user-owned jobs with status pills (`All`, `Analyzed`, `Pending`, `Failed`) and match score badges (`88% Eligible`).
- **Dedicated Job Match Report Page (`/jobs/[slug]`)**:
  - **Persistent Header**: Company, Title, Location, Remote badge, Match Badge (`88% Strong Match`), primary actions (`Download PDF`, `Re-evaluate`), and `[← Prev / Next →]` sequential job navigation.
  - **Tab 1: Job Details**: Full role description, company details, requirements, and direct external application link.
  - **Tab 2: Match Report**: Radial/metric score gauge, eligibility verdict, multi-category evaluation breakdown (Core Skills, Relevant Experience, Domain Knowledge), side-by-side evidence citations (Candidate Proof vs. Job Requirements), and strengths & gaps.
  - **Tab 3: Tailored Resume**: Generation status badge, AI tailored CV recommendations & improvements, and a prominent presigned PDF download button.
- **API Client & Mock Adapter Layer**: Typed client communicating with the Go API over HTTPS, injecting Clerk Bearer tokens into request headers, backed by a local mock adapter (`lib/api/mock-adapter.ts`) for seamless frontend development and testing while backend enhancements are implemented.
- **Simple, Modern Design**: Clean typography, high contrast, zinc neutral dark/light styling, and fast responsive layout using Tailwind CSS and Radix/shadcn primitives.

### Non-Goals
- **Generic Dashboard Widgets**: No vanity statistics counters, global analytics charts, or general dashboard overviews.
- **Unattended Job Submission**: In accordance with `AGENTS.md`, external job submission requires explicit user approval and is not handled automatically.
- **In-Browser Raw JSON Resume Template Editor**: Tab 3 displays generation status, tailored improvements summary, and direct PDF download; interactive visual editing of raw JSON resume templates is deferred.
- **Public Marketing Pages**: MVP focuses strictly on the authenticated candidate workflow (`(auth)` and `(app)`).

### Anti-Goals
- **No Direct Neon DB Connections**: The frontend must never connect directly to PostgreSQL/Neon. All data access flows through the Go API.
- **No Client Secrets**: Clerk publishable keys and public API URLs only.
- **No UI Slop**: No distracting animations or bloated dependencies; focused, fast, and legible.

---

## 3. Constraints & Technical Baseline

- **Framework**: Next.js 15+ (App Router), React 19 / TypeScript.
- **Styling**: Tailwind CSS, Lucide icons, Radix UI / shadcn component patterns.
- **Auth**: `@clerk/nextjs` with middleware session gating.
- **API Baseline**: Go backend at `/v1/*` documented in `apps/backend/docs/swagger.json`.
- **Structure**: Aligned with `docs/architecture/structure-2.md` (scoped to the job match report tool).

---

## 4. Evaluated Approaches & Selection

### Approach 1: Dedicated Focused Pages (Selected)
- Root `/` is the Jobs List; `/jobs/[slug]` is the dedicated Job Match Report Page with segmented tabs (`Job Details`, `Match Report`, `Tailored Resume`) and quick `← Prev / Next →` navigation in the header.
- **Why Selected**: Rich evaluation reports contain dense structured data (score gauges, multi-category progress bars, evidence citation tables). A dedicated full-width page provides optimal reading space on laptops and mobile devices without the cramped horizontal squishing inherent to two-pane split views.

### Approach 2: Persistent Split-Pane Master-Detail (Rejected)
- Fixed left list and right report pane on a single screen.
- **Reason for Rejection**: Squeezes dense match evidence and resume summaries on standard 13–14" laptops, forcing awkward horizontal scrolling.

---

## 5. Information Architecture & Component Hierarchy

```text
apps/web/
├── app/
│   ├── (auth)/
│   │   ├── sign-in/[[...sign-in]]/page.tsx   # Clerk <SignIn />
│   │   └── sign-up/[[...sign-up]]/page.tsx   # Clerk <SignUp />
│   ├── (app)/
│   │   ├── layout.tsx                        # Minimal top navbar (Logo, Jobs link, Clerk <UserButton />)
│   │   ├── page.tsx                          # Jobs List (Search, status pills, job cards)
│   │   └── jobs/
│   │       └── [slug]/
│   │           └── page.tsx                  # Dedicated Job Match Report Page (Header + 3 Tabs)
│   ├── layout.tsx                            # ClerkProvider, Root theme, fonts
│   └── globals.css
├── components/
│   ├── ui/                                   # Button, Card, Badge, Tabs, Progress, Skeleton
│   ├── layout/                               # Navbar, UserMenu
│   ├── jobs/                                 # JobCard, JobFilters, JobDetailView, JobNavControls
│   ├── matching/                             # MatchScoreGauge, CategoryList, EvidenceList, StrengthsGaps
│   └── resume/                               # ResumeOverview, DownloadButton
├── hooks/
│   ├── use-jobs.ts                           # Client-side search & filtering over jobs
│   ├── use-job-detail.ts                     # Fetch job by slug with latest evaluation
│   └── use-matching-evaluation.ts            # Fetch evaluation report & resume status
└── lib/
    ├── api/
    │   ├── client.ts                         # Fetch wrapper with Clerk Bearer token
    │   ├── endpoints.ts                      # Typed API methods
    │   └── mock-adapter.ts                   # Local fallback mock data for seamless dev/testing
    └── utils.ts                              # Styling & date formatting helpers
```

---

## 6. Detailed UX & User Flow

### 1. Auth Flow
- Unauthenticated requests to `/(app)/*` redirect to `/sign-in`.
- Clerk handles login (Email, Google, GitHub).
- Post-login lands directly on `/` (Jobs List).

### 2. Jobs List (`/`)
- Instant client-side search bar (by title, company, location).
- Status filter pills: `All`, `Analyzed`, `Pending`, `Failed`.
- Job Cards displaying:
  - Role Title, Company, Location, Remote tag, Posted date.
  - Match Score badge (e.g. `88% Match`, `Eligible`) when evaluation exists.
  - Clicking any card opens `/jobs/[slug]`.

### 3. Dedicated Job Match Report Page (`/jobs/[slug]`)
- **Persistent Header**:
  - Breadcrumb (`← Back to Jobs`).
  - Company & Title, Location, Remote tag.
  - Overall Compatibility Badge (`88% Match • Strong Recommendation`).
  - Navigation controls: `[← Prev]` and `[Next →]` to switch jobs sequentially.
  - Primary Action Button: "Analyze Compatibility" (if not evaluated) or "Re-evaluate" (force).
  - Secondary Action: "Download Tailored PDF".
- **Tab 1: Job Details**:
  - Full job description rendered cleanly with markdown/whitespace preservation.
  - Role highlights, original posting link (`Apply on Company Site`).
- **Tab 2: Match Report**:
  - **Overall Score Summary**: Radial score gauge / metric card with eligibility status (`eligible` / `ineligible`).
  - **Category Evaluations**: Interactive accordion / progress bars for each category (e.g., Core Skills, Relevant Experience, Domain Knowledge).
  - **Evidence Citations**: Side-by-side mapping candidate proof vs. job requirements (Direct vs. Transferable).
  - **Strengths & Gaps**: Visual callouts with actionable insights.
- **Tab 3: Tailored Resume**:
  - Generation status badge (`ready`, `generating`, `failed`).
  - Tailored recommendations & CV improvements summary from report output.
  - Prominent download button triggering presigned URL retrieval from `/v1/matching-evaluations/{id}/resume/download`.

---

## 7. Backend API Specification (For Backend Team / User)

The backend team will implement the following additions in `apps/backend`. During frontend development, `lib/api/mock-adapter.ts` provides fallback responses matching this exact contract:

### 1. Include `latest_evaluation` in `GET /v1/jobs/{slug}`
On `models.Job`, add an optional field `latest_evaluation`:
```json
{
  "id": 42,
  "slug": "senior-go-engineer-google-remote",
  "title": "Senior Go Engineer",
  "company": "Google",
  "location": "Remote",
  "status": "analyzed",
  "description": "...",
  "latest_evaluation": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "evaluation_status": "success",
    "overall_score": 88.5,
    "eligibility_status": "eligible",
    "recommendation_label": "strong_match",
    "report_status": "ready",
    "resume_status": "ready",
    "created_at": "2026-11-15T08:00:00Z",
    "updated_at": "2026-11-15T08:05:00Z"
  }
}
```
*(If no evaluation exists yet for the authenticated user and job, `latest_evaluation` is `null`).*

### 2. Include `evaluation_data` in `GET /v1/matching-evaluations/{id}/report`
Extend `models.MatchingEvaluationReportResponse` to include the category breakdown and evidence citations (`evaluation_data`):
```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "user_job_id": "...",
  "evaluation_status": "success",
  "report_status": "ready",
  "overall_score": 88.5,
  "eligibility_status": "eligible",
  "recommendation_label": "strong_match",
  "report_data": {
    "summary": "...",
    "strengths": ["Strong Go background", "Distributed systems experience"],
    "gaps": ["Lacks Kafka experience"],
    "cv_improvements": ["Highlight gRPC and concurrency in Stripe role"]
  },
  "evaluation_data": {
    "evaluations": [
      {
        "category": "Core Skills",
        "score": 90,
        "reason": "Exceeds required Go experience",
        "strengths": ["Go concurrency patterns"],
        "gaps": [],
        "evidence": [
          {
            "candidate_evidence": "5 years building high-throughput Go APIs",
            "job_requirement": "3+ years production Go experience",
            "relationship": "direct"
          }
        ]
      }
    ],
    "mandatory_requirements": [
      {
        "requirement": "Authorized to work in US / Remote",
        "status": "met",
        "reason": "Candidate is US remote eligible"
      }
    ]
  }
}
```

### 3. Idempotent `POST /v1/jobs/bulk-analyze-compatibility`
When called with `force: false` on an already evaluated job, return the existing evaluation summary (`id`, `evaluation_status`, `created_at`) with status 200/202 instead of returning an error string.

---

## 8. What is Left to the Builder

- Exact theme tokens and palette (zinc dark/light modern neutral palette).
- Loading skeleton designs and error toast handling.
- Implementation of client-side cache strategy (SWR or React Query).
