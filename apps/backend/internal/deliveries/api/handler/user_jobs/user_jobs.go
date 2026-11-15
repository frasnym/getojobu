package user_jobs

import (
	"net/http"
	"strconv"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucUserJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/user_jobs"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
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
}

// ListUserJobs returns the authenticated user's jobs (jobs linked to the user
// in user_jobs), ordered by created_at descending.
// @Summary List user jobs
// @Description List jobs owned by the authenticated user, ordered by created_at descending. Optional limit (default 20, max 100).
// @Tags user-jobs
// @Produce json
// @Param limit query integer false "Maximum number of jobs to return (default 20, max 100)"
// @Success 200 {array} models.Job
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/user-jobs [get]
func (h *handler) ListUserJobs(w http.ResponseWriter, r *http.Request) error {
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
		if n > 100 {
			n = 100
		}
		limit = n
	}

	jobs, err := h.ucJobs.ListByUser(r.Context(), p.ID, limit)
	if err != nil {
		logging.Error(r.Context(), "list user jobs failed", err,
			"user_id", p.ID, "limit", limit)
		return err
	}

	if jobs == nil {
		jobs = []*models.Job{}
	}

	return httpx.RespondJSON(w, r, http.StatusOK, jobs)
}
