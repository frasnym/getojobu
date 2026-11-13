package jobs

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs"
	ucMatching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

// handler handles job CRUD operations.
type handler struct {
	cfg        *config.Config
	userUC     ucUsers.User
	ucJobs     ucJobs.Job
	matchingUC ucMatching.Matching
}

// NewHandler constructs a job handler from the full contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:        c.Cfg,
		userUC:     c.Usecase.Users,
		ucJobs:     c.Usecase.Jobs,
		matchingUC: c.Usecase.Matching,
	}
}

// Route registers the job routes under the given group.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.userUC)
	apiKey := middleware.NewAuthMiddleware(h.cfg, nil)

	g.GET("", h.ListJobs, auth.ValidClerkOrAPIKey)
	g.POST("", h.CreateJob, auth.ValidClerkOrAPIKey)
	g.POST("/bulk", h.BulkCreateJob, auth.ValidClerkOrAPIKey)
	g.POST("/bulk-queue-analyze", h.BulkQueueAnalyzeJob, auth.ValidClerkOrAPIKey)
	g.POST("/scrape", h.ScrapeJobs, apiKey.ValidAPIKey)
	g.GET("/{slug}", h.GetJob, auth.ValidClerkOrAPIKey)
	g.PATCH("/{slug}", h.UpdateJob, auth.ValidClerkOrAPIKey)
	g.POST("/{slug}/analyze-compatibility", h.AnalyzeCompatibility, auth.ValidClerkOrAPIKey)
}

// CreateJob creates a single job listing. Returns 201 Created when inserted,
// or 200 OK with Created=false when the slug already exists (first-source-wins).
// @Summary Create job
// @Description Create a single job listing. On slug conflict the existing row is kept and returned with Created=false.
// @Tags jobs
// @Accept json
// @Produce json
// @Param job body createJobRequest true "Job data"
// @Success 201 {object} models.CreateJobResult
// @Success 200 {object} models.CreateJobResult
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs [post]
// For Clerk callers, user_id is resolved from the token and any user_id in the
// body is rejected. For API key callers, user_id is required in the request body.
func (h *handler) CreateJob(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req createJobRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	// Resolve the owning user IDs. Clerk → from token principal (reject body
	// user_ids). API key → from request body (required).
	userIDs, err := resolveOwnerUserIDs(p, req.UserIDs)
	if err != nil {
		return err
	}

	postedDate, err := req.Validate()
	if err != nil {
		return err
	}

	actorType, actorID := p.AuditActor()
	params := models.CreateJobParams{
		Slug:           models.BuildJobSlug(req.Title, req.Company, req.Location),
		Source:         req.Source,
		ExternalID:     req.ExternalID,
		SourceURL:      req.SourceURL,
		ApplyURL:       req.ApplyURL,
		Title:          req.Title,
		Company:        req.Company,
		Location:       req.Location,
		Description:    req.Description,
		PostedDate:     postedDate,
		IsRemote:       req.IsRemote,
		VisaRelocation: req.VisaRelocation,
		UpdatedByType:  actorType,
		UpdatedByID:    actorID,
		UserIDs:        userIDs,
	}

	result, err := h.ucJobs.Create(ctx, params)
	if err != nil {
		logging.Error(ctx, "create job failed", err,
			"slug", params.Slug, "actor_type", p.ActorType, "actor_id", p.ActorID)
		return err
	}

	if result.Created {
		return httpx.RespondJSON(w, r, http.StatusCreated, result)
	}
	return httpx.RespondJSON(w, r, http.StatusOK, result)
}

// @Tags jobs
// @Summary Bulk create jobs
// @Description Create many job listings at once (max 100). Conflicting slugs are skipped.
// @Accept json
// @Produce json
// @Param jobs body bulkCreateJobRequest true "Job listings"
// @Success 200 {array} models.BulkCreateJobResult
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs/bulk [post]
// BulkCreateJob creates many job listings in one request. Each row may belong
// to a different user (each row carries its own user_id). For Clerk callers,
// any user_id in a row body is rejected. For API key callers, each row must
// carry its own user_id.
func (h *handler) BulkCreateJob(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req bulkCreateJobRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if len(req.Jobs) == 0 {
		return httpx.Envelope(httpx.CodeInvalidRequest, "at least one job is required", "")
	}

	actorType, actorID := p.AuditActor()
	params := models.BulkCreateJobParams{
		Jobs: make([]models.CreateJobParams, len(req.Jobs)),
	}
	for i, j := range req.Jobs {
		userIDs, err := resolveOwnerUserIDs(p, j.UserIDs)
		if err != nil {
			return err
		}

		postedDate, err := j.Validate()
		if err != nil {
			return err
		}

		params.Jobs[i] = models.CreateJobParams{
			Slug:           models.BuildJobSlug(j.Title, j.Company, j.Location),
			Source:         j.Source,
			ExternalID:     j.ExternalID,
			SourceURL:      j.SourceURL,
			ApplyURL:       j.ApplyURL,
			Title:          j.Title,
			Company:        j.Company,
			Location:       j.Location,
			Description:    j.Description,
			PostedDate:     postedDate,
			IsRemote:       j.IsRemote,
			VisaRelocation: j.VisaRelocation,
			UpdatedByType:  actorType,
			UpdatedByID:    actorID,
			UserIDs:        userIDs,
		}
	}

	results, err := h.ucJobs.CreateBulk(ctx, params)
	if err != nil {
		logging.Error(ctx, "bulk create jobs failed", err,
			"job_count", len(params.Jobs), "actor_type", p.ActorType, "actor_id", p.ActorID)
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusOK, results)
}

// @Tags jobs
// @Summary Bulk queue job analysis
// @Description Queue job description analysis workflows in Temporal for multiple jobs. Jobs must have status 'new' or 'failed' (with force=true).
// @Accept json
// @Produce json
// @Param jobs body bulkQueueAnalyzeRequest true "Jobs to queue for analysis"
// @Success 202 {object} bulkQueueAnalyzeResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @Failure 422 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs/bulk-queue-analyze [post]
func (h *handler) BulkQueueAnalyzeJob(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req bulkQueueAnalyzeRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if len(req) == 0 {
		return httpx.Envelope(httpx.CodeInvalidRequest, "at least one job is required", "")
	}
	if len(req) > 100 {
		return httpx.Envelope(httpx.CodeInvalidRequest, "maximum 100 jobs per request", "")
	}

	items := make([]models.QueueAnalysisItem, len(req))
	for i, item := range req {
		if strings.TrimSpace(item.Slug) == "" {
			return httpx.Envelope(httpx.CodeInvalidRequest, "slug is required for each job", "")
		}
		items[i] = models.QueueAnalysisItem{
			Slug:  strings.TrimSpace(item.Slug),
			Force: item.Force,
		}
	}

	workflowIDs, err := h.ucJobs.QueueAnalysisBulk(ctx, items, p)
	if err != nil {
		logging.Error(ctx, "bulk queue analyze failed", err,
			"job_count", len(items), "actor_type", p.ActorType, "actor_id", p.ActorID)
		if errors.Is(err, constants.ErrNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "job not found", "")
		}
		if errors.Is(err, constants.ErrInvalidTransition) {
			return httpx.Envelope(httpx.CodeInvalidRequest, err.Error(), "")
		}
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusAccepted, bulkQueueAnalyzeResponse{
		WorkflowIDs: workflowIDs,
	})
}

// @Tags jobs
// @Summary List jobs
// @Description List recent jobs ordered by created_at descending. Optional limit (default 20, max 100). Clerk callers see only their owned jobs; API key callers see all jobs.
// @Accept json
// @Produce json
// @Param limit query integer false "Maximum number of jobs to return (default 20, max 100)"
// @Success 200 {array} models.Job
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs [get]
// ListJobs returns jobs. Clerk callers see only their owned jobs; API key
// callers see all jobs.
func (h *handler) ListJobs(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return httpx.Envelope(httpx.CodeInvalidRequest, "limit must be a positive integer", "")
		}
		limit = clampLimit(n)
	}

	var jobs []*models.Job
	var err error
	// Clerk: return only jobs owned by this user. API key: return all jobs.
	userID := ""
	if p.ActorType == "" {
		userID = p.ID
	}
	jobs, err = h.ucJobs.List(ctx, limit, userID)
	if err != nil {
		logging.Error(ctx, "list jobs failed", err,
			"limit", limit, "actor_type", p.ActorType, "actor_id", p.ActorID)
		return err
	}

	if jobs == nil {
		jobs = []*models.Job{}
	}

	return httpx.RespondJSON(w, r, http.StatusOK, jobs)
}

// @Tags jobs
// @Summary Get job
// @Description Get a job listing by slug.
// @Accept json
// @Produce json
// @Param slug path string true "Job slug"
// @Success 200 {object} models.Job
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs/{slug} [get]
// GetJob returns a single job by slug. API key callers may read any job.
// Clerk callers may read only jobs they own.
func (h *handler) GetJob(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	slug := httpx.ExtractPathParam(r, "slug")
	if slug == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "job slug is required", "")
	}

	job, err := h.ucJobs.GetBySlug(ctx, slug)
	if err != nil {
		logging.Error(ctx, "get job failed", err,
			"slug", slug, "actor_type", p.ActorType, "actor_id", p.ActorID)
		if err == constants.ErrNotFound {
			return httpx.Envelope(httpx.CodeNotFound, "job not found", "")
		}
		return err
	}

	if p.ActorType == "" {
		// Clerk: ensure the caller owns this job.
		owned, err := h.ucJobs.IsOwned(ctx, p.ID, job.ID)
		if err != nil {
			logging.Error(ctx, "check job ownership failed", err,
				"slug", slug, "user_id", p.ID, "job_id", job.ID)
			return err
		}
		if !owned {
			return httpx.Envelope(httpx.CodeNotFound, "job not found", "")
		}
	}

	return httpx.RespondJSON(w, r, http.StatusOK, job)
}

// @Tags jobs
// @Summary Update job status
// @Description Transition a job's status (analyze_pending | analyzed | failed). Called by the Temporal worker on completion.
// @Accept json
// @Produce json
// @Param slug path string true "Job slug"
// @Param job body updateJobRequest true "Status update payload"
// @Success 200 {object} models.Job
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @Failure 422 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs/{slug} [patch]
// UpdateJob performs a transition-validated status update on the job identified
// by slug. The body must include a non-empty "status" field.
func (h *handler) UpdateJob(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	slug := httpx.ExtractPathParam(r, "slug")
	if slug == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "job slug is required", "")
	}

	var req updateJobRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if req.Status == nil || *req.Status == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "status is required", "")
	}

	actorType, actorID := p.AuditActor()

	job, err := h.ucJobs.UpdateStatus(ctx, slug, *req.Status, req.AnalyzeResult, req.FailureMessage, actorType, actorID)
	if err != nil {
		logging.Error(ctx, "update job status failed", err,
			"slug", slug, "status", *req.Status, "actor_type", p.ActorType, "actor_id", p.ActorID)
		if errors.Is(err, constants.ErrNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "job not found", "")
		}
		if errors.Is(err, constants.ErrInvalidTransition) {
			return httpx.Envelope(httpx.CodeInvalidRequest, err.Error(), "")
		}
		return err
	}
	return httpx.RespondJSON(w, r, http.StatusOK, job)
}

// resolveOwnerUserIDs returns the internal user ids that own the job being created.
// For Clerk callers, user_ids is resolved from the principal token and any user_ids
// sent in the body is rejected (400). For API key callers, user_ids is required in the body.
func resolveOwnerUserIDs(p *models.Principal, bodyUserIDs []string) ([]string, error) {
	if p.ActorType == "" {
		// Clerk caller: resolve user_id from the principal (internal user id).
		if len(bodyUserIDs) > 0 {
			return nil, httpx.Envelope(httpx.CodeInvalidRequest, "user_ids not allowed for Clerk authentication", "")
		}
		if p.ID == "" {
			return nil, httpx.Envelope(httpx.CodeUnauthorized, "internal user id not available", "")
		}
		return []string{p.ID}, nil
	}

	// API key caller: user_ids is required in the body.
	var uids []string
	for _, uid := range bodyUserIDs {
		trimmed := strings.TrimSpace(uid)
		if trimmed != "" && !slices.Contains(uids, trimmed) {
			uids = append(uids, trimmed)
		}
	}

	if len(uids) == 0 {
		return nil, httpx.Envelope(httpx.CodeInvalidRequest, "user_ids is required for API key authentication", "")
	}
	return uids, nil
}

// clampLimit clamps limit to the inclusive range [1, 100], defaulting to 20
// when non-positive.
func clampLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

// AnalyzeCompatibility triggers candidate-job compatibility evaluation.
// @Summary Trigger compatibility evaluation
// @Description Trigger compatibility evaluation for a specific job slug. Clerk users evaluate their own profile; machine API key callers evaluate all users associated with the job in user_jobs.
// @Tags jobs
// @Accept json
// @Produce json
// @Param slug path string true "Job slug"
// @Success 202 {array} models.MatchingEvaluationSummary
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs/{slug}/analyze-compatibility [post]
func (h *handler) AnalyzeCompatibility(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	slug := r.PathValue("slug")
	if slug == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "slug is required", "")
	}

	evals, err := h.matchingUC.TriggerCompatibility(ctx, slug, p)
	if err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "job or user link not found", "")
		}
		if errors.Is(err, constants.ErrInvalidTransition) {
			return httpx.Envelope(httpx.CodeInvalidRequest, err.Error(), "")
		}
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusAccepted, evals)
}
