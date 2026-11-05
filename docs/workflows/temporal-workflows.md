# Temporal Workflow Design

## 1. Responsibilities

Temporal orchestrates long-running and retryable job-search operations. The Go API starts workflows and sends signals. Workers poll Temporal task queues and execute activities. Workers do not call each other directly; Temporal is the coordination layer.

## 2. Initial workflow families

| Workflow | Purpose | Likely worker |
|---|---|---|
| `JobDiscoveryWorkflow` | Discover and normalize listings from configured sources | Python |
| `JobEvaluationWorkflow` | Evaluate jobs against one user's profile/skills | Python |
| `JobReviewWorkflow` | Coordinate review state and wait for user decisions where needed | Python/API signal |
| `DocumentGenerationWorkflow` | Generate CV/cover-letter variants | Python |
| `JobApplicationWorkflow` | Execute an approved application attempt and record outcome | Browser + Python |
| `ProfileSyncWorkflow` | Reconcile profile/skills into normalized records | Python |
| `JobRepairWorkflow` | Reconcile recoverable inconsistencies | Python/admin |

Names are proposed logical boundaries. Keep workflows cohesive and avoid creating a workflow for every trivial database operation.

## 3. Task queues

Suggested queues:

- `job-discovery`
- `job-evaluation`
- `job-documents`
- `job-browser`
- `job-maintenance`

Python and browser workers register only the queues and activities they own. Configure separate worker concurrency/resource limits, especially for browser sessions.

## 4. Determinism and activity rules

- Workflow code must be deterministic. Put network, database, filesystem, LLM, and browser I/O in activities.
- Activities must be idempotent or use explicit idempotency keys.
- Set start-to-close and schedule-to-close timeouts intentionally.
- Use retries only for errors that can plausibly recover.
- Use heartbeats for long-running browser or scraping activities.
- Make cancellation cooperative and clean up browser contexts and temporary files.
- Version workflows when changing behavior that may affect in-flight executions.
- Do not store secrets or unnecessary personal data in workflow input/history.

## 5. Human approval

The system must require an explicit approval before external application submission. Approval should be recorded as a durable application transition/activity, then signal or start the relevant workflow.

Do not infer approval from opening a page, generating a document, clicking a review card, or a timeout. The approval action must be attributable to the authenticated user.

## 6. Application outcomes and recovery

Use explicit outcomes:
- `applied`: external submission is confirmed.
- `failed`: submission is confirmed not to have succeeded and can be retried under policy.
- `submitted_unknown`: the browser/ATS result is ambiguous. Quarantine and require manual reconciliation.

Never blindly retry an ambiguous submit action. The workflow should first attempt a safe status check where possible, otherwise stop for user/operator review.

## 7. Browser worker isolation

The browser worker is a separate process/container with:
- independent concurrency limits,
- CPU/memory limits,
- browser version pinned in its image,
- isolated temporary directories and browser profiles,
- restricted network access where feasible,
- secrets/session material scoped to the minimum required operation,
- screenshots/downloads stored with controlled retention.

Do not expose browser debugging ports publicly.

## 8. Observability

Log workflow ID, run ID, activity type, job/application ID, and tenant ID where safe. Avoid logging CV contents, credentials, session cookies, or full prompt payloads. Emit metrics for workflow starts, completions, retries, failures, activity latency, queue backlog, and browser resource use.
