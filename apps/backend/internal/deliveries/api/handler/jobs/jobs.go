package jobs

import (
	"net/http"
	"strconv"
	"time"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// handler handles job CRUD operations.
type handler struct {
	cfg    *config.Config
	userUC ucUsers.User
	ucJobs ucJobs.Job
}

// NewHandler constructs a job handler from the full contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:    c.Cfg,
		userUC: c.Usecase.Users,
		ucJobs: c.Usecase.Jobs,
	}
}

// Route registers the job routes under the given group.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.userUC)

	g.POST("", h.CreateJob, auth.ClerkOrAPIKey)
	g.POST("/bulk", h.BulkCreateJob, auth.ClerkOrAPIKey)
	g.GET("", h.ListJobs, auth.ClerkOrAPIKey)
	g.GET("/{slug}", h.GetJob, auth.ClerkOrAPIKey)
	g.PATCH("/{slug}", h.UpdateJobDescription, auth.ClerkOrAPIKey)
}

type createJobRequest struct {
	Slug           string  `json:"slug"`
	Source         string  `json:"source"`
	ExternalID     *string `json:"external_id,omitempty"`
	SourceURL      *string `json:"source_url,omitempty"`
	ApplyURL       *string `json:"apply_url,omitempty"`
	Title          string  `json:"title"`
	Company        string  `json:"company"`
	Location       string  `json:"location"`
	Description    *string `json:"description,omitempty"`
	PostedDate     string  `json:"posted_date,omitempty"`
	IsRemote       *bool   `json:"is_remote,omitempty"`
	VisaRelocation *bool   `json:"visa_relocation,omitempty"`
}

type bulkCreateJobRequest struct {
	Jobs []createJobRequest `json:"jobs"`
}

type updateJobDescriptionRequest struct {
	Description string `json:"description"`
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
// @Router /v1/jobs [post]
func (h *handler) CreateJob(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req createJobRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if req.Slug == "" || req.Source == "" || req.Title == "" || req.Company == "" || req.Location == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "slug, source, title, company, and location are required", "")
	}

	var postedDate time.Time
	if req.PostedDate != "" {
		t, err := time.Parse("2006-01-02", req.PostedDate)
		if err != nil {
			return httpx.Envelope(httpx.CodeInvalidRequest, "posted_date must be in YYYY-MM-DD format", "")
		}
		postedDate = t
	}

	result, err := h.ucJobs.Create(r.Context(), models.CreateJobParams{
		Slug:           req.Slug,
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
		UpdatedByType:  p.ActorType,
		UpdatedByID:    p.ActorID,
	})
	if err != nil {
		return err
	}

	if result.Created {
		return httpx.RespondJSON(w, http.StatusCreated, result)
	}
	return httpx.RespondJSON(w, http.StatusOK, result)
}

// BulkCreateJob creates many job listings in one request. Each row uses
// ON CONFLICT (slug) DO NOTHING, so conflicting rows are skipped without
// rolling back the whole batch. Maximum 100 jobs per request.
// @Summary Bulk create jobs
// @Description Create many job listings at once (max 100). Conflicting slugs are skipped.
// @Tags jobs
// @Accept json
// @Produce json
// @Param jobs body bulkCreateJobRequest true "Job listings"
// @Success 200 {array} models.BulkCreateJobResult
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Router /v1/jobs/bulk [post]
func (h *handler) BulkCreateJob(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
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

	params := models.BulkCreateJobParams{
		Jobs: make([]models.CreateJobParams, len(req.Jobs)),
	}
	for i, j := range req.Jobs {
		if j.Slug == "" || j.Source == "" || j.Title == "" || j.Company == "" || j.Location == "" {
			return httpx.Envelope(httpx.CodeInvalidRequest, "slug, source, title, company, and location are required for every job", "")
		}

		var postedDate time.Time
		if j.PostedDate != "" {
			t, err := time.Parse("2006-01-02", j.PostedDate)
			if err != nil {
				return httpx.Envelope(httpx.CodeInvalidRequest, "posted_date must be in YYYY-MM-DD format", "")
			}
			postedDate = t
		}

		params.Jobs[i] = models.CreateJobParams{
			Slug:           j.Slug,
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
			UpdatedByType:  p.ActorType,
			UpdatedByID:    p.ActorID,
		}
	}

	results, err := h.ucJobs.CreateBulk(r.Context(), params)
	if err != nil {
		return err
	}

	return httpx.RespondJSON(w, http.StatusOK, results)
}

// ListJobs returns recent jobs, optionally filtered by company.
// @Summary List jobs
// @Description List recent jobs ordered by created_at descending. Optional limit (default 20, max 100) and company ILIKE filter.
// @Tags jobs
// @Produce json
// @Param limit query int false "Maximum number of jobs to return (default 20, max 100)"
// @Param company query string false "Filter by company name (case-insensitive substring)"
// @Success 200 {array} models.Job
// @Failure 401 {object} httpx.Error
// @Router /v1/jobs [get]
func (h *handler) ListJobs(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return httpx.Envelope(httpx.CodeInvalidRequest, "limit must be a positive integer", "")
		}
		limit = n
	}

	company := r.URL.Query().Get("company")

	var jobs []*models.Job
	var err error
	if company != "" {
		jobs, err = h.ucJobs.SearchByCompany(r.Context(), company, limit)
	} else {
		jobs, err = h.ucJobs.ListRecent(r.Context(), limit)
	}
	if err != nil {
		return err
	}

	if jobs == nil {
		jobs = []*models.Job{}
	}

	return httpx.RespondJSON(w, http.StatusOK, jobs)
}

// GetJob returns a single job by slug.
// @Summary Get job
// @Description Get a job listing by slug.
// @Tags jobs
// @Produce json
// @Param slug path string true "Job slug"
// @Success 200 {object} models.Job
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @Router /v1/jobs/{slug} [get]
func (h *handler) GetJob(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	slug := httpx.ExtractPathParam(r, "slug")
	if slug == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "job slug is required", "")
	}

	job, err := h.ucJobs.GetBySlug(r.Context(), slug)
	if err != nil {
		if err == constants.ErrNotFound {
			return httpx.Envelope(httpx.CodeNotFound, "job not found", "")
		}
		return err
	}

	return httpx.RespondJSON(w, http.StatusOK, job)
}

// UpdateJobDescription updates only the description of a job. Allowed only
// when the existing description IS NULL or the status is not 'analyzed'.
// @Summary Update job description
// @Description Update a job's description. Blocked with 409 when the job already has a description and status 'analyzed'.
// @Tags jobs
// @Accept json
// @Produce json
// @Param slug path string true "Job slug"
// @Param description body updateJobDescriptionRequest true "Description"
// @Success 200 {object} models.Job
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @Failure 409 {object} httpx.Error
// @Router /v1/jobs/{slug} [patch]
func (h *handler) UpdateJobDescription(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	slug := httpx.ExtractPathParam(r, "slug")
	if slug == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "job slug is required", "")
	}

	var req updateJobDescriptionRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if req.Description == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "description is required", "")
	}

	job, err := h.ucJobs.UpdateDescription(r.Context(), slug, req.Description, p.ActorType, p.ActorID)
	if err != nil {
		if err == constants.ErrNotFound {
			return httpx.Envelope(httpx.CodeNotFound, "job not found", "")
		}
		if err == ucJobs.ErrDescriptionBlocked {
			return httpx.Envelope(httpx.CodeConflict, "description update blocked: job is already analyzed", "")
		}
		return err
	}

	return httpx.RespondJSON(w, http.StatusOK, job)
}
