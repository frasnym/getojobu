import { Job, MatchReport, DownloadResumeResponse, EvaluationSummary } from "./types";

export const MOCK_JOBS: Job[] = [
  {
    id: 1,
    slug: "senior-go-engineer-stripe-remote",
    status: "analyzed",
    title: "Senior Go Engineer - Infrastructure",
    company: "Stripe",
    location: "Remote, US/EU",
    is_remote: true,
    apply_url: "https://stripe.com/jobs/senior-go-infra",
    posted_date: "2026-11-13T10:00:00Z",
    created_at: "2026-11-13T10:00:00Z",
    description: `### About Stripe
Stripe is a financial infrastructure platform for the internet. Millions of companies use Stripe to accept payments, grow their revenue, and accelerate new business opportunities.

### Responsibilities
- Architect, build, and maintain core transaction processing microservices using Go and PostgreSQL.
- Ensure 99.999% uptime across high-throughput distributed systems.
- Collaborate with platform security and infrastructure teams to scale global distributed databases.

### Requirements
- 5+ years of experience building production backend systems with Go.
- Deep expertise in distributed systems, concurrency, and high-throughput APIs.
- Production experience with PostgreSQL, message queues, and containerization.
- Strong communication and asynchronous collaboration skills.`,
    latest_evaluation: {
      id: "eval-stripe-001",
      evaluation_status: "success",
      overall_score: 88.5,
      eligibility_status: "eligible",
      recommendation_label: "strong_match",
      report_status: "ready",
      resume_status: "ready",
      created_at: "2026-11-14T08:30:00Z",
      updated_at: "2026-11-14T08:35:00Z",
    },
  },
  {
    id: 2,
    slug: "staff-backend-developer-datadog-nyc",
    status: "analyzed",
    title: "Staff Backend Engineer - Core Telemetry",
    company: "Datadog",
    location: "New York, NY",
    is_remote: false,
    apply_url: "https://datadog.com/jobs/staff-backend",
    posted_date: "2026-11-12T09:00:00Z",
    created_at: "2026-11-12T09:00:00Z",
    description: `Datadog is the monitoring and security platform for cloud applications. We are seeking a Staff Backend Engineer to drive architectural decisions across high-volume time-series telemetry ingestion pipelines.`,
    latest_evaluation: {
      id: "eval-datadog-002",
      evaluation_status: "success",
      overall_score: 76.0,
      eligibility_status: "eligible",
      recommendation_label: "good_match",
      report_status: "ready",
      resume_status: "ready",
      created_at: "2026-11-13T14:10:00Z",
      updated_at: "2026-11-13T14:15:00Z",
    },
  },
  {
    id: 3,
    slug: "lead-systems-engineer-cloudflare-remote",
    status: "pending",
    title: "Lead Systems Engineer - Edge Compute",
    company: "Cloudflare",
    location: "Remote",
    is_remote: true,
    apply_url: "https://cloudflare.com/careers/lead-systems",
    posted_date: "2026-11-14T11:00:00Z",
    created_at: "2026-11-14T11:00:00Z",
    description: `Cloudflare's mission is to help build a better Internet. We are seeking a Systems Engineer with deep Go/Rust experience to improve Workers and Edge runtime routing.`,
    latest_evaluation: null,
  },
];

export const MOCK_REPORTS: Record<string, MatchReport> = {
  "eval-stripe-001": {
    id: "eval-stripe-001",
    user_job_id: "uj-stripe-001",
    evaluation_status: "success",
    report_status: "ready",
    overall_score: 88.5,
    eligibility_status: "eligible",
    recommendation_label: "strong_match",
    report_data: {
      summary:
        "The candidate is an exceptionally strong fit for Stripe's infrastructure role. With over 6 years of production Go development, deep concurrency knowledge, and hands-on PostgreSQL optimization, the candidate fulfills all core engineering requirements.",
      strengths: [
        "6+ years production Go microservices experience exceeding the 5-year requirement",
        "Extensive experience with high-throughput transactional database systems and pgx",
        "Demonstrated track record of maintaining high-availability cloud deployments",
      ],
      gaps: [
        "Limited explicit mention of Kafka streaming in candidate projects; transferable messaging experience in RabbitMQ and Redis",
      ],
      cv_improvements: [
        "Quantify the transactions-per-second throughput in your most recent fintech backend role",
        "Highlight your experience architecting zero-downtime database migrations in PostgreSQL",
        "Emphasize distributed tracing and OpenTelemetry observability experience",
      ],
    },
    evaluation_data: {
      evaluations: [
        {
          category: "Core Skills & Go Mastery",
          score: 95,
          reason: "Extensive background building idiomatic, low-latency Go services with concurrency patterns.",
          strengths: ["Go concurrency primitives", "Benchmarking and memory profiling"],
          gaps: [],
          evidence: [
            {
              candidate_evidence: "Built and scaled distributed payment gateway handling 15,000 req/sec in Go",
              job_requirement: "5+ years experience building production backend systems with Go",
              relationship: "direct",
            },
          ],
        },
        {
          category: "Distributed Systems & Reliability",
          score: 85,
          reason: "Solid understanding of fault tolerance, idempotency, and high availability design.",
          strengths: ["Circuit breakers", "Idempotency keys implementation"],
          gaps: ["No direct Kafka stream processing cited"],
          evidence: [
            {
              candidate_evidence: "Implemented Temporal workflows for distributed multi-step transaction rollbacks",
              job_requirement: "Deep expertise in distributed systems and transaction processing",
              relationship: "direct",
            },
            {
              candidate_evidence: "Managed Redis pub/sub and RabbitMQ asynchronous queues",
              job_requirement: "Production experience with message queues and event-driven architecture",
              relationship: "transferable",
            },
          ],
        },
        {
          category: "Databases & Storage",
          score: 90,
          reason: "Deep PostgreSQL query tuning and connection pooling knowledge.",
          strengths: ["Index optimization", "pgx connection pooling"],
          gaps: [],
          evidence: [
            {
              candidate_evidence: "Reduced 99th percentile query latency by 45% using composite indexing and partitioned tables",
              job_requirement: "Production experience with PostgreSQL and relational data modeling",
              relationship: "direct",
            },
          ],
        },
      ],
      mandatory_requirements: [
        {
          requirement: "Authorized to work in US/EU remote jurisdictions",
          status: "met",
          reason: "Candidate profile confirms valid work authorization.",
        },
        {
          requirement: "5+ years Go experience",
          status: "met",
          reason: "Candidate has 6+ verified years of production Go.",
        },
      ],
    },
    created_at: "2026-11-14T08:30:00Z",
    updated_at: "2026-11-14T08:35:00Z",
  },
};

export const MOCK_RESUME_DOWNLOADS: Record<string, DownloadResumeResponse> = {
  "eval-stripe-001": {
    download_url: "https://storage.getojobu.com/resumes/signed-download-eval-stripe-001.pdf?signature=valid",
    expires_at: new Date(Date.now() + 3600 * 1000).toISOString(),
    filename: "Alex_Rivera_Stripe_Tailored_Resume.pdf",
  },
};
