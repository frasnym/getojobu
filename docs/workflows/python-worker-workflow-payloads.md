# Python Worker Workflow Payloads

The Python worker registers eight workflows on three task queues (see `workers/python/worker/main.py`). Every workflow takes one `input` dict. "Required" means a missing or empty key makes the run fail; keys with a fallback or workflow-applied default are marked no. "Migrate to API" marks big dict payloads to replace with an ID fetched by the activity at runtime; "Remove" is the checklist for that key.

Source of truth is `workers/python/workflows/*.py` plus the activity validators in `workers/python/deliveries/consumer/activities/`.

## Payload keys

| Workflow                    | Task queue     | Payload key                 | Required | Migrate to API | Remove | Description                                                                   |
| --------------------------- | -------------- | --------------------------- | -------- | -------------- | ------ | ----------------------------------------------------------------------------- |
| JobAnalyzeWorkflow          | job-evaluation | `slug`                      | yes      | No             | —      | Job slug for the Go API lookup and status transitions.                        |
| MatchJobWorkflow            | job-evaluation | `evaluation_id`             | yes      | No             | —      | Evaluation row updated with status, result, and metadata.                     |
| GenerateReportWorkflow      | job-evaluation | `evaluation_id`             | yes      | No             | —      | Report row marked pending, success, or failed.                                |
| GenerateResumeWorkflow      | job-evaluation | `evaluation_id`             | yes      | No             | —      | Required by the generate and status activities.                               |
| GenerateCoverLetterWorkflow | job-evaluation | `evaluation_id`             | yes      | No             | —      | Required by the generate and status activities.                               |
| GenerateCoverLetterWorkflow | job-evaluation | `variant`                   | yes      | No             | —      | Letter variant, for example `short`.                                          |
| JobScrapeWorkflow           | job-scrape     | `board`                     | yes      | No             | —      | Source board name; must be non-empty.                                         |
| JobScrapeWorkflow           | job-scrape     | `keyword`                   | no       | No             | —      | Search term for the board query.                                              |
| JobScrapeWorkflow           | job-scrape     | `location`                  | no       | No             | —      | Search location for the board query.                                          |
| JobScrapeWorkflow           | job-scrape     | `user_ids`                  | no       | No             | —      | Internal user ids to attribute matches to.                                    |
| JobScrapeWorkflow           | job-scrape     | `search_id`                 | no       | No             | —      | Deterministic scan id used for idempotency.                                   |
| JobScrapeWorkflow           | job-scrape     | `searches`                  | no       | No             | —      | Legacy field; not read by the workflow.                                       |
| JobScrapeWorkflow           | job-scrape     | `workflow_id`               | no       | No             | —      | Echoed in the result only.                                                    |
| UserJobScrapeWorkflow       | job-scrape     | `job_id`                    | no       | No             | —      | Internal job id sent by Go; the Python worker does not read it.               |
| UserJobScrapeWorkflow       | job-scrape     | `slug`                      | yes      | No             | —      | Skeleton job slug used for status updates.                                    |
| UserJobScrapeWorkflow       | job-scrape     | `source_url`                | yes      | No             | —      | URL fetched and parsed by the extraction activity.                            |
| UserJobScrapeWorkflow       | job-scrape     | `user_id`                   | no       | No             | —      | User id sent by Go; the Python worker does not read it.                       |
| UserJobScrapeWorkflow       | job-scrape     | `owner_id`                  | yes      | No             | —      | Owner of the user-supplied job; required on update.                           |
| ConcurrencyLimiter          | job-limit      | `state` (start)             | no       | No             | —      | Optional start arg carrying `queue` and `failed_items` after Continue-As-New. |
| ConcurrencyLimiter          | job-limit      | `params` (enqueue signal)   | no       | No             | —      | Child workflow input dict.                                                    |
| ConcurrencyLimiter          | job-limit      | `item_id` (enqueue signal)  | yes      | No             | —      | Dedup key; empty ids are rejected.                                            |
| ConcurrencyLimiter          | job-limit      | `job_type` (enqueue signal) | no       | No             | —      | Routes to a queue and workflow; defaults to `scrape`.                         |

`job_type` values for the `enqueue` signal are `scrape`, `analyze`, `match`, `report`, `resume`, `cover_letter`, and `user_job_scrape`.

## Migration notes

- `candidate_profile` and `facts` are cached on `users.profile_data` and `users.facts_data`. Worker-auth `POST /v1/users/bulk-refresh-profile` recomposes and stores them for up to 100 user IDs. `GetEvaluation` composes once when either column is NULL, then uses the cache. Workflows always get the current profile, not a per-evaluation snapshot.
- `job_profile` is derived from the shared jobs row (`analyze_result` plus title/company/location) on every GET. It is not cached per user because it belongs to the job.
- `evaluation_config` is resolved from `scoring_config_id` on the evaluation row.
- `matching_evaluations.candidate_profile_data` and `job_profile_data` were dropped in migration `000020_user_profile_cache.sql`. `evaluation_data` and `scoring_config_id` remain.
- Each generation workflow calls a fetch activity first and skips unless its status is `queued` or `queued_retry`: `evaluation_status` (match), `report_status` (report), `resume_status` (resume), `cover_letter_status` (cover letter). Go flips the status before enqueueing.
- `job_description` was removed from the workflow input and from `AnalyzeJob.analyze()`, which now takes only `slug`. The usecase calls `get_job(slug)` and uses the stored job description; the Go caller still validates that a description exists before queueing.

The only generation payload keys left are `evaluation_id`, plus `variant` for cover letters. `slug` remains for job analysis; board/scan fields and limiter fields stay as-is.

## Migration checklist

| Done | Area   | Task                                                                                                                                                           |
| ---- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [x]  | Python | Remove `job_description` from the `JobAnalyzeWorkflow` payload and from `AnalyzeJob.analyze()`; the JD is fetched by slug.                                     |
| [x]  | Go     | Remove the unused `job_id` field from `JobAnalyzeWorkflowInput`; only `slug` remains.                                                                          |
| [x]  | Go     | Remove the unused `slug` field from `MatchJobWorkflowInput`.                                                                                                   |
| [x]  | API    | Add worker-auth `GET /v1/matching-evaluations/{id}` returning `evaluation_status`, `candidate_profile`, `job_profile`, and `evaluation_config`.                |
| [x]  | API    | Regenerate the OpenAPI spec for the new endpoint.                                                                                                              |
| [x]  | API    | No generated TypeScript client exists in `apps/web/lib/api`; the OpenAPI spec is the contract.                                                                 |
| [x]  | Python | Add `get_evaluation(evaluation_id)` to the `GetojobuAPI` protocol and client in `workers/python/repository/getojobu_api.py`.                                   |
| [x]  | Go     | Stop composing the profile/config dicts in `TriggerEvaluations`; only `evaluation_id` is enqueued.                                                             |
| [x]  | Go     | Flip evaluation status to queued/queued_retry before enqueueing so the worker's status check is correct.                                                       |
| [x]  | Python | Add `fetch_match_input_activity` to fetch the snapshot and gate on status in `MatchJobWorkflow`.                                                               |
| [x]  | Python | Remove dead `provider` and `model` reads in `MatchJobWorkflow`.                                                                                                |
| [x]  | API    | Extend `GetEvaluationResponse` with `report_status`, `evaluation_results`, and the score fields; regenerate Swagger.                                          |
| [x]  | Go     | Enqueue only `evaluation_id` for report generation; the stored snapshots are loaded by the worker.                                                            |
| [x]  | Python | Add `fetch_report_input_activity` and gate `GenerateReportWorkflow` on `report_status`.                                                                       |
| [x]  | API    | Add worker-auth `POST /v1/users/bulk-refresh-profile` and `RefreshProfileCache` usecase.                                                                      |
| [x]  | Go     | Add `users.profile_data` and `users.facts_data`; compose on cache miss in `GetEvaluation`.                                                                    |
| [x]  | Go     | Drop `matching_evaluations.candidate_profile_data` and `job_profile_data`; derive the job profile from the jobs row.                                          |
| [x]  | Python | Add `fetch_resume_input_activity` and `fetch_cover_letter_input_activity`; gate workflows on their status.                                                   |
| [x]  | Python | Remove dead `template_name` read from resume generation.                                                                                                      |
| [x]  | Tests  | Update Python activity and workflow tests to feed only IDs and assert the API fetch.                                                                           |
| [x]  | Tests  | Update Go usecase tests to assert the new workflow input shape and enqueue order.                                                                              |
| [ ]  | Tests  | Add an API handler test for the new GET endpoint, including authorization.                                                                                     |
| [x]  | Verify | Run `make test` in `workers/python` and the Go backend test suite, then re-check the payload table against the code.                                           |
