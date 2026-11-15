# Web Application Design System (getojobu)

> **Specification & UI/UX Guidelines for `apps/web`**  
> *Last updated: 2026-11-15*

---

## 1. Design Philosophy & Ethos

`getojobu` is a job matching and application-assistance platform. The web application (`apps/web`) is purpose-built as an **authoritative, high-density candidate workspace**.

### Core Tenets

1. **Anti-Dashboard Bloat**: No vanity metric cards, global analytics graphs, or overview screens that delay the candidate from evaluating jobs. Every screen directly delivers actionable insight.
2. **Dense Yet Legible**: Job evaluations produce rich, multi-dimensional AI outputs (scores, multi-category breakdowns, side-by-side evidence citations, resume diffs). The UI organizes dense information into clear visual layers without horizontal squishing.
3. **High Signal Contrast**: Modern zinc-neutral monochromatic base paired with high-contrast semantic status accents (Emerald for high compatibility, Amber for moderate gaps, Rose for mismatches).
4. **Fluid Navigation**: Seamless sequential workflow (`[← Prev / Next →]`) allowing candidates to triage 20+ matched positions rapidly without round-tripping to the feed.

---

## 2. Design Tokens & Theme Variables

The design system is built on **CSS Variables** defined in `app/globals.css` and integrated via `tailwind.config.ts`.

### 2.1 Color Tokens (HSL)

#### Light Mode (`:root`)
| Token | HSL Value | Hex Approx | Semantic Purpose |
|---|---|---|---|
| `--background` | `0 0% 100%` | `#FFFFFF` | Application canvas |
| `--foreground` | `240 10% 3.9%` | `#09090B` | Primary text |
| `--card` | `0 0% 100%` | `#FFFFFF` | Surfaces & containers |
| `--card-foreground` | `240 10% 3.9%` | `#09090B` | Text on card surfaces |
| `--primary` | `240 5.9% 10%` | `#18181B` | Primary CTAs, active states |
| `--primary-foreground` | `0 0% 98%` | `#FAFAFA` | High-contrast text on primary |
| `--secondary` | `240 4.8% 95.9%` | `#F4F4F5` | Subtle buttons, secondary pills |
| `--muted` | `240 4.8% 95.9%` | `#F4F4F5` | Neutral borders, tracks |
| `--muted-foreground` | `240 3.8% 46.1%` | `#71717A` | Captions, metadata, inactive icons |
| `--border` | `240 5.9% 90%` | `#E4E4E7` | Component outlines, dividers |
| `--destructive` | `0 84.2% 60.2%` | `#EF4444` | Dangerous actions & critical errors |
| `--ring` | `240 5.9% 10%` | `#18181B` | Focus rings (accessibility) |

#### Dark Mode (`.dark`)
| Token | HSL Value | Hex Approx | Semantic Purpose |
|---|---|---|---|
| `--background` | `240 10% 3.9%` | `#09090B` | Deep zinc background |
| `--foreground` | `0 0% 98%` | `#FAFAFA` | Crisp primary text |
| `--card` | `240 10% 3.9%` | `#09090B` | Elevated card surfaces |
| `--primary` | `0 0% 98%` | `#FAFAFA` | Inverted primary actions |
| `--primary-foreground` | `240 5.9% 10%` | `#18181B` | Text on dark-mode primary |
| `--secondary` | `240 3.7% 15.9%` | `#27272A` | Secondary chip background |
| `--border` | `240 3.7% 15.9%` | `#27272A` | Subtle dark borders |

### 2.2 Semantic Status & Match Tier Colors

Used across badges, progress gauges, and compatibility pills:

| Tier | Score Range | Background (Light) | Text / Border (Light) | Semantic Label |
|---|---|---|---|---|
| **High Match** | $\ge 80\%$ | `bg-emerald-50 dark:bg-emerald-950/40` | `text-emerald-700 border-emerald-200 dark:text-emerald-400` | Strong Match / Eligible |
| **Moderate Match** | $60\% - 79\%$ | `bg-amber-50 dark:bg-amber-950/40` | `text-amber-700 border-amber-200 dark:text-amber-400` | Moderate / Review Required |
| **Low / Gap** | $< 60\%$ | `bg-rose-50 dark:bg-rose-950/40` | `text-rose-700 border-rose-200 dark:text-rose-400` | Low Compatibility / Missing Skills |
| **Pending / Queue** | N/A | `bg-zinc-100 dark:bg-zinc-800` | `text-zinc-600 dark:text-zinc-400` | Analysis Queued |

### 2.3 Geometry & Radii
- Base Radius (`--radius`): `0.5rem` (`8px`)
- Card & Container Radius: `rounded-lg` (`8px`) or `rounded-xl` (`12px`)
- Badges & Pills: `rounded-full`
- Form Controls (Inputs, Selects): `rounded-md` (`6px`)

---

## 3. Typography & Spacing Hierarchy

### 3.1 Type Scale

- **Display / Job Title** (`text-2xl` / `24px` to `text-3xl` / `30px`, font-bold, `tracking-tight`): Primary page headings and job titles.
- **Section Heading** (`text-lg` / `18px`, font-semibold): Card titles, tab panel section headers.
- **Body Regular** (`text-sm` / `14px`, font-normal, `leading-relaxed`): Job descriptions, AI rationales, requirements.
- **Meta / Caption** (`text-xs` / `12px`, font-medium): Tags, timestamp pills, source metadata, citation markers.
- **Monospace** (`font-mono text-xs`): Code snippets, technical keywords, resume file hashes, citation references.

### 3.2 Spacing System

- Layout Container: `max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-8`
- Stack Gaps:
  - Micro (`gap-1` to `gap-2`): Badges inside meta rows, icon-to-label spacing.
  - Form & List (`gap-4`): Card grids, form inputs, category accordion items.
  - Sectional (`gap-6` to `gap-8`): Main workspace tabs, header-to-content spacing.

---

## 4. Navigation & Page Architecture

```text
┌────────────────────────────────────────────────────────────────────────┐
│ Navbar: [Logo: getojobu]     [Jobs]                [Clerk User Button] │
└────────────────────────────────────────────────────────────────────────┘
                                    │
           ┌────────────────────────┴────────────────────────┐
           ▼                                                 ▼
   / (Jobs Feed)                                     /jobs/[slug]
┌─────────────────────────────┐    ┌─────────────────────────────────────┐
│ Search & Filters            │    │ Sticky Header:                      │
│ - Live search input         │    │ - [← Prev / Next →] Job Switcher    │
│ - Category pills            │    │ - Title, Company, Remote Tag        │
│ - Remote-only toggle        │    │ - Match Score Gauge / Badge         │
│                             │    │ - Primary Actions (Apply, Re-eval)  │
│ Job Cards Grid              │    ├─────────────────────────────────────┤
│ - Title, Company, Location  │    │ Segmented Tabs:                     │
│ - Match Badge (88% Strong)  │───►│ [1. Job Details]                    │
│ - Quick Skills Tags         │    │ [2. Match Report] (Dense AI Analysis│
│ - Direct Open CTA           │    │ [3. Tailored Resume] (PDF Download) │
└─────────────────────────────┘    └─────────────────────────────────────┘
```

---

## 5. Core Component Specifications

### 5.1 Navbar (`components/layout/navbar.tsx`)
- **Height**: `56px` (`h-14`), sticky top with `backdrop-blur-md bg-background/80 border-b border-border`.
- **Left**: Brand logo icon (`Briefcase` or `Sparkles`) + bold wordmark.
- **Center / Left Links**: Subtle hover navigation (`text-muted-foreground` to `text-foreground`).
- **Right**: User state via Clerk `<UserButton />` with custom avatar fallback.

### 5.2 Job Card (`components/jobs/job-card.tsx`)
- **Container**: `border border-border rounded-xl p-5 bg-card hover:border-zinc-400 dark:hover:border-zinc-600 transition-all shadow-sm hover:shadow-md`.
- **Header**: Flex layout with Title (`text-base font-semibold`) and Match Score Badge (`rounded-full px-2.5 py-0.5 text-xs font-semibold`).
- **Company & Meta**: Location pin icon, briefcase icon, salary range (if present).
- **Skills Pills**: Up to 4 requirement pills (`bg-secondary text-secondary-foreground text-xs px-2 py-0.5 rounded-md`).

### 5.3 Job Report Header (`components/jobs/job-report-header.tsx`)
- **Persistent Context**: Displays job title, company, location, and remote status.
- **Sequential Pager**:
  - `[← Prev]` and `[Next →]` buttons enabling one-click browsing across the filtered jobs queue.
  - Disables gracefully at bounds (`index === 0` or `index === total - 1`).
- **Action Strip**:
  - Direct external application link (`Apply Externally` with external-link icon).
  - `Re-evaluate` button (triggers async compatibility re-analysis).

### 5.4 Match Score Gauge (`components/matching/match-score-gauge.tsx`)
- **Visual**: Custom SVG circular progress ring (radius `40px`, stroke `6px`, smooth `stroke-dashoffset` transition).
- **Center**: Prominent percentage readout (`text-2xl font-bold`) and descriptive verdict (`text-xs font-medium`).
- **Color Coding**: Dynamic SVG stroke based on $\ge 80\%$ (emerald), $60-79\%$ (amber), $<60\%$ (rose).

### 5.5 Category Breakdown (`components/matching/category-accordion.tsx`)
- **Categories**: Core Skills, Experience Match, Domain Knowledge, Education/Certifications.
- **Progress Bar**: Horizontal track (`h-2 bg-muted rounded-full overflow-hidden`) with filled score indicator.
- **Expansion Panel**: Collapsible accordion revealing detailed AI reasoning and specific missing competencies.

### 5.6 Evidence List (`components/matching/evidence-list.tsx`)
- **Side-by-Side Verification**: 2-column comparison card:
  - **Left**: *Job Requirement* (exact extracted sentence or keyword from listing).
  - **Right**: *Candidate Proof* (citation extracted from user's career profile or past projects).
- **Confidence Rating**: Verified badge (`bg-emerald-100 text-emerald-800 text-[10px] font-mono`).

### 5.7 Tailored Resume Tab (`components/resume/resume-tab.tsx`)
- **Status Indicator**:
  - Ready: Green check with generated timestamp.
  - In Progress: Animated spinner badge (`Generating tailored resume...`).
  - Pending: Call-to-action button to trigger AI document generation.
- **Key Enhancements**: AI bullet points summarizing changes made to the base CV to highlight job-specific keywords.
- **Download CTA (`components/resume/download-resume-button.tsx`)**:
  - Prominent primary button with PDF download icon.
  - Safe presigned URL handler opening direct browser download without popup blocker disruption.

---

## 6. Accessibility & Keyboard Guidelines

- **Focus Rings**: All interactive buttons, tabs, and form controls have high-contrast focus rings (`focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2`).
- **Tab Navigation**: Segmented tabs implement standard WAI-ARIA tab pattern (`role="tablist"`, `aria-selected`, arrow key navigation).
- **Color Independence**: Status badges never rely on color alone; every indicator pairs color with text labels (e.g. `88% Strong Match`, `52% Missing Skills`).
- **Screen Reader Support**: Meaningful `aria-label` attributes on icon-only navigation buttons (`Previous job`, `Next job`, `Download resume PDF`).

---

## 7. Developer & Implementation Notes

- **Offline / Mock Mode**: Toggleable via `NEXT_PUBLIC_ENABLE_MOCK=true` in `.env.local` for instantaneous offline development and UI verification without requiring live VPS API services.
- **OpenAPI Synchronization**: All API interfaces in `lib/api/types.ts` strictly adhere to the Go backend OpenAPI contract in `apps/backend/docs/swagger.json`.
- **Component Colocation**: Feature-specific components are organized under `components/<feature>/` to maintain modularity and prevent bloated shared utility folders.
