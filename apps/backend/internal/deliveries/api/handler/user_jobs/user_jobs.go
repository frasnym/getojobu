package user_jobs

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucUserJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/user_jobs"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/logging"
)

// handler handles user-jobs list operations.
type handler struct {
	cfg    *config.Config
	userUC ucUsers.User
	ucJobs ucUserJobs.UserJobs
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

// setHideStatusRequest is the PATCH /v1/user-jobs/{job_id}/hide body.
type setHideStatusRequest struct {
	IsHidden bool `json:"is_hidden"`
}

// setHideStatusResponse is the outcome of toggling hide on a user job.
type setHideStatusResponse struct {
	UserJobID string `json:"user_job_id"`
	IsHidden  bool   `json:"is_hidden"`
}

// NewHandler constructs a user-jobs handler from the full contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:    c.Cfg,
		userUC: c.Usecase.Users,
		ucJobs: c.Usecase.UserJobs,
	}
}

// Route registers the user-jobs routes under the given group.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.userUC)

	g.GET("", h.ListUserJobs, auth.ValidClerkSession)
	g.POST("", h.AddUserJob, auth.ValidClerkSession)
	g.PATCH("/{user_job_id}/hide", h.SetHideStatus, auth.ValidClerkSession)
}

// ListUserJobs returns the authenticated user's jobs (jobs linked to the user
// in user_jobs), ordered by created_at descending.
// @Summary List user jobs
// @Description List jobs owned by the authenticated user, ordered by created_at descending. Optional pagination (default page 1, limit 10, max limit 50).
// @Tags user-jobs
// @Produce json
// @Param page query integer false "Page number (default 1)"
// @Param limit query integer false "Items per page (default 10, max 50)"
// @Success 200 {array} models.Job
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

	jobs, totalItems, err := h.ucJobs.ListByUser(r.Context(), p.ID, limit, offset)
	if err != nil {
		logging.Error(r.Context(), "list user jobs failed", err,
			"user_id", p.ID, "page", page, "limit", limit, "offset", offset)
		return err
	}

	if jobs == nil {
		jobs = []*models.Job{}
	}

	return httpx.RespondJSON(w, r, http.StatusOK, jobs, httpx.WithPaginationParams(page, limit, totalItems))
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

// SetHideStatus toggles the is_hidden flag on a user↔job link.
// The link must belong to the authenticated user (404 otherwise).
// @Summary Toggle hide status on a user job
// @Description Hide or unhide a user↔job link owned by the authenticated user. Idempotent.
// @Tags user-jobs
// @Accept json
// @Produce json
// @Param user_job_id path string true "User job link ID"
// @Param hide body setHideStatusRequest true "Hide status"
// @Success 200 {object} setHideStatusResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/user-jobs/{user_job_id}/hide [patch]
func (h *handler) SetHideStatus(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	userJobID := httpx.ExtractPathParam(r, "user_job_id")
	if strings.TrimSpace(userJobID) == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "user_job_id is required", "")
	}
	if _, err := uuid.Parse(userJobID); err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "user_job_id must be a valid UUID", "")
	}

	var req setHideStatusRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if err := h.ucJobs.SetHideStatus(r.Context(), p.ID, userJobID, req.IsHidden); err != nil {
		logging.Error(r.Context(), "set user job hide status failed", err,
			"user_id", p.ID, "user_job_id", userJobID, "is_hidden", req.IsHidden)
		if errors.Is(err, constants.ErrNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "user job not found", "")
		}
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusOK, setHideStatusResponse{
		UserJobID: userJobID,
		IsHidden:  req.IsHidden,
	})
}
