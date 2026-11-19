# Python Worker Workflow Payloads

The Python worker registers eight workflows on three task queues (see `workers/python/worker/main.py`). Every workflow takes one `input` dict. "Required" means a missing or empty key makes the run fail; keys with a fallback or workflow-applied default are marked no. "Migrate to API" marks big dict payloads to replace with an ID fetched by the activity at runtime; "Remove" is the checklist for that key.

Source of truth is `workers/python/workflows/*.py` plus the activity validators in `workers/python/deliveries/consumer/activities/`.

## Payload keys

| Workflow | Task queue | Payload key | Required | Migrate to API | Remove | Description |
|---|---|---|---|---|---|---|
| JobAnalyzeWorkflow | job-evaluation | `job_id` | no | No | — | Internal job id sent by the Go caller; the Python worker does not read it. |
| JobAnalyzeWorkflow | job-evaluation | `slug` | yes | No | — | Job slug for the Go API lookup and status transitions. |
| MatchJobWorkflow | job-evaluation | `evaluation_id` | yes | No | — | Evaluation row updated with status, result, and metadata. |
| MatchJobWorkflow | job-evaluation | `slug` | no | No | — | Declared in the Go and Python input types but not set by the Go trigger and unused by the worker. |
| MatchJobWorkflow | job-evaluation | `candidate_profile` | yes | Yes | [ ] | Candidate data passed to the LLM match activity. |
| MatchJobWorkflow | job-evaluation | `job_profile` | yes | Yes | [ ] | Job data passed to the LLM match activity. |
| MatchJobWorkflow | job-evaluation | `evaluation_config` | yes | Yes | [ ] | Scoring and prompt config passed to the LLM match activity. |
| MatchJobWorkflow | job-evaluation | `provider` | no | No | — | Read by the Python workflow but never set by the Go trigger, so always absent. |
| MatchJobWorkflow | job-evaluation | `model` | no | No | — | Read by the Python workflow but never set by the Go trigger, so always absent. |
| GenerateReportWorkflow | job-evaluation | `evaluation_id` | yes | No | — | Report row marked pending, success, or failed. |
| GenerateReportWorkflow | job-evaluation | `candidate_profile` | yes | Yes | [ ] | Input to the report LLM. |
| GenerateReportWorkflow | job-evaluation | `job_profile` | yes | Yes | [ ] | Input to the report LLM. |
| GenerateReportWorkflow | job-evaluation | `evaluation_results` | yes | Yes | [ ] | Match findings summarized in the report. |
| GenerateReportWorkflow | job-evaluation | `overall_score` | no | No | — | Defaults to `0.0` in the workflow. |
| GenerateReportWorkflow | job-evaluation | `eligibility_status` | no | No | — | Defaults to `eligible`. |
| GenerateReportWorkflow | job-evaluation | `recommendation_label` | no | No | — | Defaults to `worth_applying`. |
| GenerateReportWorkflow | job-evaluation | `report_status` | no | No | — | Queue status sent by the Go caller; the Python worker does not read it. |
| GenerateResumeWorkflow | job-evaluation | `evaluation_id` | yes | No | — | Required by the generate and status activities. |
| GenerateResumeWorkflow | job-evaluation | `candidate_profile` | yes | Yes | [ ] | CV input to the resume LLM. |
| GenerateResumeWorkflow | job-evaluation | `job_profile` | yes | Yes | [ ] | CV input to the resume LLM. |
| GenerateResumeWorkflow | job-evaluation | `evaluation_results` | yes | Yes | [ ] | Match findings used for tailoring. |
| GenerateResumeWorkflow | job-evaluation | `facts` | yes | Yes | [ ] | Structured facts used for tailoring. |
| GenerateResumeWorkflow | job-evaluation | `template_name` | no | No | — | Read by the Python activity but never set by the Go caller, so always absent. |
| GenerateResumeWorkflow | job-evaluation | `overall_score` | no | No | — | Optional, forwarded to the LLM. |
| GenerateResumeWorkflow | job-evaluation | `eligibility_status` | no | No | — | Optional, forwarded to the LLM. |
| GenerateResumeWorkflow | job-evaluation | `recommendation_label` | no | No | — | Optional, forwarded to the LLM. |
| GenerateResumeWorkflow | job-evaluation | `job_id` | no | No | — | Optional R2 object-key prefix for the uploaded PDF and JSON. |
| GenerateResumeWorkflow | job-evaluation | `user_id` | no | No | — | Optional R2 object-key prefix for the uploaded PDF and JSON. |
| GenerateCoverLetterWorkflow | job-evaluation | `evaluation_id` | yes | No | — | Required by the generate and status activities. |
| GenerateCoverLetterWorkflow | job-evaluation | `job_id` | no | No | — | Sent by the Go caller; the Python worker does not read it. |
| GenerateCoverLetterWorkflow | job-evaluation | `user_id` | no | No | — | Sent by the Go caller; the Python worker does not read it. |
| GenerateCoverLetterWorkflow | job-evaluation | `candidate_profile` | yes | Yes | [ ] | Letter input to the LLM. |
| GenerateCoverLetterWorkflow | job-evaluation | `job_profile` | yes | Yes | [ ] | Letter input to the LLM. |
| GenerateCoverLetterWorkflow | job-evaluation | `evaluation_results` | yes | Yes | [ ] | Match findings referenced in the letter. |
| GenerateCoverLetterWorkflow | job-evaluation | `facts` | yes | Yes | [ ] | Structured facts referenced in the letter. |
| GenerateCoverLetterWorkflow | job-evaluation | `variant` | yes | No | — | Letter variant, for example `short`. |
| GenerateCoverLetterWorkflow | job-evaluation | `overall_score` | no | No | — | Optional, forwarded to the LLM. |
| GenerateCoverLetterWorkflow | job-evaluation | `eligibility_status` | no | No | — | Optional, forwarded to the LLM. |
| GenerateCoverLetterWorkflow | job-evaluation | `recommendation_label` | no | No | — | Optional, forwarded to the LLM. |
| JobScrapeWorkflow | job-scrape | `board` | yes | No | — | Source board name; must be non-empty. |
| JobScrapeWorkflow | job-scrape | `keyword` | no | No | — | Search term for the board query. |
| JobScrapeWorkflow | job-scrape | `location` | no | No | — | Search location for the board query. |
| JobScrapeWorkflow | job-scrape | `user_ids` | no | No | — | Internal user ids to attribute matches to. |
| JobScrapeWorkflow | job-scrape | `search_id` | no | No | — | Deterministic scan id used for idempotency. |
| JobScrapeWorkflow | job-scrape | `searches` | no | No | — | Legacy field; not read by the workflow. |
| JobScrapeWorkflow | job-scrape | `workflow_id` | no | No | — | Echoed in the result only. |
| UserJobScrapeWorkflow | job-scrape | `job_id` | no | No | — | Internal job id sent by Go; the Python worker does not read it. |
| UserJobScrapeWorkflow | job-scrape | `slug` | yes | No | — | Skeleton job slug used for status updates. |
| UserJobScrapeWorkflow | job-scrape | `source_url` | yes | No | — | URL fetched and parsed by the extraction activity. |
| UserJobScrapeWorkflow | job-scrape | `user_id` | no | No | — | User id sent by Go; the Python worker does not read it. |
| UserJobScrapeWorkflow | job-scrape | `owner_id` | yes | No | — | Owner of the user-supplied job; required on update. |
| ConcurrencyLimiter | job-limit | `state` (start) | no | No | — | Optional start arg carrying `queue` and `failed_items` after Continue-As-New. |
| ConcurrencyLimiter | job-limit | `params` (enqueue signal) | no | No | — | Child workflow input dict. |
| ConcurrencyLimiter | job-limit | `item_id` (enqueue signal) | yes | No | — | Dedup key; empty ids are rejected. |
| ConcurrencyLimiter | job-limit | `job_type` (enqueue signal) | no | No | — | Routes to a queue and workflow; defaults to `scrape`. |

`job_type` values for the `enqueue` signal are `scrape`, `analyze`, `match`, `report`, `resume`, `cover_letter`, and `user_job_scrape`.

## Migration notes

- `candidate_profile`, `job_profile`, and `evaluation_results` are stored as JSONB columns `candidate_profile_data`, `job_profile_data`, and `evaluation_data` on the matching evaluation row. A worker-auth GET endpoint keyed by `evaluation_id` can return them.
- `evaluation_config` is the scoring config referenced by `scoring_config_id` on the evaluation row. The worker already has `get_active_scoring_config(family)`; the GET response should include the config ID so the exact version used at queue time can be fetched.
- `facts` is not stored as one column. The Go service composes it from the user profile (name, contact, headline, education, roles) while building the workflow input, so the new GET endpoint must recompose it from `evaluation_id`.
- `job_description` was removed from the workflow input and from `AnalyzeJob.analyze()`, which now takes only `slug`. The usecase calls `get_job(slug)` and uses the stored job description; the Go caller still validates that a description exists before queueing.
- The Python API client needs a matching `get_evaluation(evaluation_id)` method before activities can replace these payload keys.

`job_id` and `user_id` are already IDs, so keep them in the payload. The small scalars and routing keys (`overall_score`, `eligibility_status`, `recommendation_label`, `variant`, `template_name`, `provider`, `model`, board/scan fields, limiter fields) are not big payloads and stay as-is.

## Migration checklist

| Done | Area | Task |
|---|---|---|
| [x] | Python | Remove `job_description` from the `JobAnalyzeWorkflow` payload and from `AnalyzeJob.analyze()`; the JD is fetched by slug. |
| [ ] | API | Add worker-auth `GET /v1/matching-evaluations/{id}` returning `candidate_profile_data`, `job_profile_data`, `evaluation_data`, `scoring_config_id`, and recomposed `facts`. |
| [ ] | API | Regenerate the OpenAPI spec and TypeScript client after adding the endpoint. |
| [ ] | Python | Add `get_evaluation(evaluation_id)` to the `GetojobuAPI` protocol and client in `workers/python/repository/getojobu_api.py`. |
| [ ] | Go | Stop composing the removed dicts in `apps/backend/internal/usecase/matching/` once the GET endpoint supplies them. |
| [ ] | Python | Fetch each removed key inside its activity from `evaluation_id` instead of reading `input`. |
| [ ] | Python | Remove dead `provider` and `model` reads in `MatchJobWorkflow`; the Go trigger never sets them. |
| [ ] | Python | Set evaluation statuses (`pending`, `success`, `failed`) from the fetched row, not from payload fields. |
| [ ] | Tests | Update Python activity and workflow tests to feed only IDs and assert the API fetch. |
| [ ] | Tests | Update Go usecase tests to assert the new workflow input shape. |
| [ ] | Tests | Add an API test for the new GET endpoint, including authorization and cross-tenant isolation. |
| [ ] | Verify | Run `make test` in `workers/python` and the Go backend test suite, then re-check the payload table against the code. |
