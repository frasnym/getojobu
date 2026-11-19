package jobs

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/dto"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs"
	ucMatching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	ucUserJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/user_jobs"
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
	userJobs   ucUserJobs.UserJobs
	matchingUC ucMatching.Matching
}

// NewHandler constructs a job handler from the full contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:        c.Cfg,
		userUC:     c.Usecase.Users,
		ucJobs:     c.Usecase.Jobs,
		userJobs:   c.Usecase.UserJobs,
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
	g.POST("/manual", h.CreateManualJob, auth.ValidClerkSession)
	g.POST("/scrape-url", h.ScrapeURL, auth.ValidClerkSession)
	g.POST("/bulk-queue-analyze", h.BulkQueueAnalyzeJob, auth.ValidClerkOrAPIKey)
	g.POST("/bulk-analyze-compatibility", h.BulkAnalyzeCompatibility, auth.ValidClerkOrAPIKey)
	g.POST("/scrape", h.ScrapeJobs, apiKey.ValidAPIKey)
	g.GET("/{slug}", h.GetJob, auth.ValidClerkOrAPIKey)
	g.PATCH("/{slug}", h.UpdateJob, auth.ValidClerkOrAPIKey)
}

// CreateJob creates a single job listing. Returns 201 Created when inserted,
// or 200 OK with Created=false when the slug already exists (first-source-wins).
// @Summary Create job
// @Description Create a single job listing. On slug conflict the existing row is kept and returned with Created=false.
// @Tags jobs
// @Accept json
// @Produce json
// @Param job body createJobRequest true "Job data"
// @Success 201 {object} dto.CreateJobResponse
// @Success 200 {object} dto.CreateJobResponse
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

	resp := dto.NewCreateJobResponse(result)
	if result.Created {
		return httpx.RespondJSON(w, r, http.StatusCreated, resp)
	}
	return httpx.RespondJSON(w, r, http.StatusOK, resp)
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
// @Description Queue job description analysis workflows in Temporal for multiple jobs. Jobs must have status 'new', 'queued', or 'failed' (with force=true). Per-item failures are returned in "failed" and do not stop the remaining jobs.
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

	if err := req.Validate(); err != nil {
		return err
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

	result, err := h.ucJobs.QueueAnalysisBulk(ctx, items, p)
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

	failed := make([]bulkQueueAnalyzeFailure, len(result.Failed))
	for i, f := range result.Failed {
		failed[i] = bulkQueueAnalyzeFailure{Slug: f.Slug, Error: f.Error}
	}

	return httpx.RespondJSON(w, r, http.StatusAccepted, bulkQueueAnalyzeResponse{
		WorkflowIDs: result.WorkflowIDs,
		Failed:      failed,
	})
}

// @Tags jobs
// @Summary List jobs
// @Description List jobs with optional pagination. Any authenticated user sees all jobs; each job carries an isOwned flag when the caller owns it.
// @Accept json
// @Produce json
// @Param page query integer false "Page number (default 1)"
// @Param limit query integer false "Items per page (default 10, max 50)"
// @Success 200 {array} dto.JobResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs [get]
// ListJobs returns all jobs. Clerk callers get an isOwned flag per job; API key
// callers see isOwned=false on every job.
func (h *handler) ListJobs(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	page := 1
	if v := r.URL.Query().Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return httpx.Envelope(httpx.CodeInvalidRequest, "page must be a positive integer", "")
		}
		page = n
	}

	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return httpx.Envelope(httpx.CodeInvalidRequest, "limit must be a positive integer", "")
		}
		limit = n
	}

	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	offset := (page - 1) * limit

	userID := ""
	if p.ActorType == "" {
		userID = p.ID
	}
	jobsWithOwnership, totalItems, err := h.ucJobs.List(ctx, limit, offset, userID)
	if err != nil {
		logging.Error(ctx, "list jobs failed", err,
			"page", page, "limit", limit, "offset", offset, "actor_type", p.ActorType, "actor_id", p.ActorID)
		return err
	}

	items := make([]dto.JobResponse, len(jobsWithOwnership))
	for i, j := range jobsWithOwnership {
		var isOwned *bool
		if p.ActorType == "" {
			owned := j.IsOwned
			isOwned = &owned
		}
		items[i] = dto.NewJobResponse(j.Job, isOwned)
	}

	return httpx.RespondJSON(w, r, http.StatusOK, items, httpx.WithPaginationParams(page, limit, totalItems))
}

// @Tags jobs
// @Summary Get job
// @Description Get a job listing by slug.
// @Accept json
// @Produce json
// @Param slug path string true "Job slug"
// @Success 200 {object} dto.JobResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs/{slug} [get]
// GetJob returns a single job by slug. Any authenticated user may read any job.
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

	userID := ""
	if p.ActorType == "" {
		userID = p.ID
	}
	jobWithOwnership, err := h.ucJobs.GetBySlug(ctx, slug, userID)
	if err != nil {
		logging.Error(ctx, "get job failed", err,
			"slug", slug, "actor_type", p.ActorType, "actor_id", p.ActorID)
		if err == constants.ErrNotFound {
			return httpx.Envelope(httpx.CodeNotFound, "job not found", "")
		}
		return err
	}

	var isOwned *bool
	if p.ActorType == "" {
		owned := jobWithOwnership.IsOwned
		isOwned = &owned
	}

	resp := dto.NewJobDetailResponse(jobWithOwnership.Job, isOwned, jobWithOwnership.UserJobID)
	return httpx.RespondJSON(w, r, http.StatusOK, resp)
}

// @Tags jobs
// @Summary Update job status
// @Description Transition a job's status (queued | queued_retry | analyze_pending | analyzed | failed). Called by the Temporal worker on completion.
// @Accept json
// @Produce json
// @Param slug path string true "Job slug"
// @Param job body updateJobRequest true "Status update payload"
// @Success 200 {object} dto.JobResponse
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

	// Clerk callers may only update jobs they can see. Machine callers keep
	// full access so the Python worker can update private jobs.
	if p.ActorType == "" {
		if _, err := h.ucJobs.GetBySlug(ctx, slug, p.ID); err != nil {
			if errors.Is(err, constants.ErrNotFound) {
				return httpx.Envelope(httpx.CodeNotFound, "job not found", "")
			}
			return err
		}
	}

	actorType, actorID := p.AuditActor()

	var postedDate *time.Time
	if req.PostedDate != nil && *req.PostedDate != "" {
		parsed, err := time.Parse("2006-01-02", *req.PostedDate)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339Nano, *req.PostedDate)
			if err != nil {
				return httpx.Envelope(httpx.CodeInvalidRequest, "posted_date must be in YYYY-MM-DD or RFC3339 format", "")
			}
		}
		postedDate = &parsed
	}

	job, err := h.ucJobs.UpdateStatus(ctx, models.UpdateJobParams{
		Slug:           slug,
		Status:         *req.Status,
		AnalyzeResult:  req.AnalyzeResult,
		FailureMessage: req.FailureMessage,
		NewSlug:        req.Slug,
		Title:          req.Title,
		Company:        req.Company,
		Location:       req.Location,
		Description:    req.Description,
		SourceURL:      req.SourceURL,
		ApplyURL:       req.ApplyURL,
		PostedDate:     postedDate,
		IsRemote:       req.IsRemote,
		VisaRelocation: req.VisaRelocation,
		UpdatedByType:  actorType,
		UpdatedByID:    actorID,
	})
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
	return httpx.RespondJSON(w, r, http.StatusOK, dto.NewJobResponse(job, nil))
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

// clampLimit clamps limit to the inclusive range [1, 50], defaulting to 10
// when non-positive.
func clampLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 50 {
		return 50
	}
	return limit
}

// BulkAnalyzeCompatibility triggers candidate-job compatibility evaluation in bulk.
// @Summary Trigger bulk compatibility evaluation
// @Description Trigger compatibility evaluation for multiple jobs in bulk with partial results. Clerk users evaluate their own profile; machine API key callers evaluate all users associated with the job in user_jobs.
// @Tags jobs
// @Accept json
// @Produce json
// @Param jobs body bulkAnalyzeCompatibilityRequest true "Jobs to evaluate compatibility"
// @Success 202 {array} models.MatchingEvaluationSummary
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/jobs/bulk-analyze-compatibility [post]
func (h *handler) BulkAnalyzeCompatibility(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req bulkAnalyzeCompatibilityRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if err := req.Validate(); err != nil {
		return err
	}

	items := make([]models.BulkAnalyzeCompatibilityItem, len(req))
	for i, item := range req {
		items[i] = models.BulkAnalyzeCompatibilityItem{
			Slug:  strings.TrimSpace(item.Slug),
			Force: item.Force,
		}
	}

	evals, err := h.matchingUC.TriggerCompatibilityBulk(ctx, items, p)
	if err != nil {
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusAccepted, evals)
}
