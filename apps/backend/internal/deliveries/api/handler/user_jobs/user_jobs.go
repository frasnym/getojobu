package user_jobs

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/dto"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucMatching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	ucUserJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/user_jobs"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

// handler handles user-jobs list operations.
type handler struct {
	cfg        *config.Config
	userUC     ucUsers.User
	ucJobs     ucUserJobs.UserJobs
	matchingUC ucMatching.Matching
}

// addUserJobRequest is the POST /v1/user-jobs body.
type addUserJobRequest struct {
	JobID int64 `json:"job_id"`
}

// addUserJobResponse is the outcome of adding a job to the user's list.
type addUserJobResponse struct {
	JobID   int64 `json:"job_id"`
	Created bool  `json:"created"`
}

// setHideStatusRequest is the PATCH /v1/user-jobs/bulk body.
type setHideStatusRequest struct {
	UserJobIDs []string `json:"user_job_ids"`
	IsHidden   bool     `json:"is_hidden"`
}

// setHideStatusResponse is the outcome of toggling hide on user jobs.
type setHideStatusResponse struct {
	Updated int `json:"updated"`
}

// NewHandler constructs a user-jobs handler from the full contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:        c.Cfg,
		userUC:     c.Usecase.Users,
		ucJobs:     c.Usecase.UserJobs,
		matchingUC: c.Usecase.Matching,
	}
}

// Route registers the user-jobs routes under the given group.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.userUC)

	g.GET("", h.ListUserJobs, auth.ValidClerkSession)
	g.POST("", h.AddUserJob, auth.ValidClerkSession)
	g.PATCH("/bulk", h.SetBulkHideStatus, auth.ValidClerkSession)
	g.GET("/{user_job_id}/evaluation", h.GetEvaluation, auth.ValidClerkSession)
}

// ListUserJobs returns the authenticated user's jobs (jobs linked to the user
// in user_jobs), ordered by is_hidden ascending (visible jobs first) and then
// created_at descending.
// @Summary List user jobs
// @Description List jobs owned by the authenticated user, ordered by is_hidden ascending (visible first) then created_at descending. Optional pagination (default page 1, limit 10, max limit 50).
// @Tags user-jobs
// @Produce json
// @Param page query integer false "Page number (default 1)"
// @Param limit query integer false "Items per page (default 10, max 50)"
// @Param status query string false "Filter by job status"
// @Param remote query boolean false "Filter by remote jobs"
// @Param hide query string false "Filter by hide status (visible, hidden, or all)"
// @Param evaluation_status query string false "Filter by matching evaluation status"
// @Param eligibility_status query string false "Filter by eligibility status"
// @Param recommendation_label query string false "Filter by recommendation label"
// @Param report_status query string false "Filter by match report status"
// @Param resume_status query string false "Filter by resume status"
// @Success 200 {array} dto.UserJobResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/user-jobs [get]
func (h *handler) ListUserJobs(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
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

	filter := models.UserJobFilter{
		Status:              r.URL.Query().Get("status"),
		RemoteOnly:          r.URL.Query().Get("remote") == "true",
		HideFilter:          r.URL.Query().Get("hide"),
		EvaluationStatus:    r.URL.Query().Get("evaluation_status"),
		EligibilityStatus:   r.URL.Query().Get("eligibility_status"),
		RecommendationLabel: r.URL.Query().Get("recommendation_label"),
		ReportStatus:        r.URL.Query().Get("report_status"),
		ResumeStatus:        r.URL.Query().Get("resume_status"),
	}

	jobs, totalItems, err := h.ucJobs.ListByUser(r.Context(), p.ID, filter, limit, offset)
	if err != nil {
		logging.Error(r.Context(), "list user jobs failed", err,
			"user_id", p.ID, "page", page, "limit", limit, "offset", offset)
		return err
	}

	dtoRows := make([]dto.UserJobResponse, len(jobs))
	for i, row := range jobs {
		dtoRows[i] = dto.NewUserJobResponse(row)
	}

	return httpx.RespondJSON(w, r, http.StatusOK, dtoRows, httpx.WithPaginationParams(page, limit, totalItems))
}

// AddUserJob links a job listing to the authenticated user's saved jobs
// (user_jobs). Idempotent: linking an already-linked job returns 200 OK with
// created=false; a new link returns 201 Created.
// @Summary Add job to user jobs
// @Description Link a job listing to the authenticated user's saved jobs. Idempotent: linking an already-linked job returns 200 OK with created=false; a new link returns 201 Created.
// @Tags user-jobs
// @Accept json
// @Produce json
// @Param job body addUserJobRequest true "Job to add"
// @Success 201 {object} addUserJobResponse
// @Success 200 {object} addUserJobResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/user-jobs [post]
func (h *handler) AddUserJob(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req addUserJobRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if req.JobID <= 0 {
		return httpx.Envelope(httpx.CodeInvalidRequest, "job_id must be a positive integer", "")
	}

	created, err := h.ucJobs.Add(r.Context(), p.ID, req.JobID)
	if err != nil {
		logging.Error(r.Context(), "add user job failed", err,
			"user_id", p.ID, "job_id", req.JobID)
		return err
	}

	resp := addUserJobResponse{JobID: req.JobID, Created: created}
	if created {
		return httpx.RespondJSON(w, r, http.StatusCreated, resp)
	}
	return httpx.RespondJSON(w, r, http.StatusOK, resp)
}

// SetBulkHideStatus sets is_hidden on multiple user↔job links owned by the
// authenticated user in one request. Links that don't exist or aren't owned by
// the user are skipped. Idempotent.
// @Summary Bulk toggle hide status on user jobs
// @Description Hide or unhide multiple user↔job links owned by the authenticated user. Idempotent.
// @Tags user-jobs
// @Accept json
// @Produce json
// @Param hide body setHideStatusRequest true "Hide status"
// @Success 200 {object} setHideStatusResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/user-jobs/bulk [patch]
func (h *handler) SetBulkHideStatus(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req setHideStatusRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if len(req.UserJobIDs) == 0 {
		return httpx.Envelope(httpx.CodeInvalidRequest, "user_job_ids must not be empty", "")
	}
	for _, id := range req.UserJobIDs {
		if strings.TrimSpace(id) == "" {
			return httpx.Envelope(httpx.CodeInvalidRequest, "user_job_ids must not contain empty values", "")
		}
		if _, err := uuid.Parse(id); err != nil {
			return httpx.Envelope(httpx.CodeInvalidRequest, "user_job_ids must be valid UUIDs", "")
		}
	}

	updated, err := h.ucJobs.SetBulkHideStatus(r.Context(), p.ID, req.UserJobIDs, req.IsHidden)
	if err != nil {
		logging.Error(r.Context(), "set bulk user job hide status failed", err,
			"user_id", p.ID, "user_job_ids", req.UserJobIDs, "is_hidden", req.IsHidden)
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusOK, setHideStatusResponse{
		Updated: updated,
	})
}

// GetEvaluation returns the latest matching evaluation for a user-job link.
// The link must belong to the authenticated user (404 otherwise).
// @Summary Get matching evaluation for a user job
// @Description Returns the latest matching evaluation for the given user_job_id, owned by the authenticated user.
// @Tags user-jobs
// @Produce json
// @Param user_job_id path string true "User job link ID (UUID)"
// @Success 200 {object} dto.EvaluationResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/user-jobs/{user_job_id}/evaluation [get]
func (h *handler) GetEvaluation(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	uid, err := uuid.Parse(p.ID)
	if err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid user id", "")
	}

	userJobID := httpx.ExtractPathParam(r, "user_job_id")
	if strings.TrimSpace(userJobID) == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "user_job_id is required", "")
	}
	ujid, err := uuid.Parse(userJobID)
	if err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "user_job_id must be a valid UUID", "")
	}

	evalWithJob, err := h.matchingUC.GetEvaluationByUserJobID(r.Context(), uid, ujid)
	if err != nil {
		logging.Error(r.Context(), "get user job evaluation failed", err,
			"user_id", p.ID, "user_job_id", userJobID)
		if errors.Is(err, constants.ErrNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "evaluation not found", "")
		}
		return err
	}

	// The principal already carries the user's full name, so the cover-letter
	// download filename is computed here without a second user lookup.
	jobTitle := ""
	if evalWithJob.Job != nil {
		jobTitle = evalWithJob.Job.Title
	}
	evalWithJob.Evaluation.CoverLetterFileName = ucMatching.CoverLetterFileName(p.FullName, jobTitle)

	// Strip service metadata (model, prompt_version) from evaluation_data
	// and report_data before exposing them to clients. Internal Go usecases
	// still read the raw fields from the DB.
	for _, field := range []*json.RawMessage{&evalWithJob.Evaluation.EvaluationData, &evalWithJob.Evaluation.ReportData} {
		*field = utils.StripJSONKeys(*field, "metadata")
	}

	return httpx.RespondJSON(w, r, http.StatusOK, dto.NewEvaluationResponsePublic(evalWithJob.Evaluation, evalWithJob.Job))
}
