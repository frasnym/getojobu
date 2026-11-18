package retry

import (
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	ucRetry "github.com/frasnym/getojobu/apps/backend/internal/usecase/retry"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

type handler struct {
	cfg     *config.Config
	retryUC ucRetry.Retry
}

// NewHandler constructs the retry handler from the full contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{cfg: c.Cfg, retryUC: c.Usecase.Retry}
}

// Route registers the retry routes. Only machine API keys may call them.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, nil)
	g.POST("/retry-failed-jobs", h.RetryFailedJobs, auth.ValidAPIKey)
	g.POST("/retry-failed-matching-evaluations", h.RetryFailedMatchingEvaluations, auth.ValidAPIKey)
}

// RetryFailedJobs re-queues failed job analysis workflows.
// @Summary Retry failed jobs
// @Description Re-queues up to 20 failed job analysis workflows with force=true. Machine API key only.
// @Tags maintenance
// @Produce json
// @Success 200 {object} models.RetryJobsResult
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/maintenance/retry-failed-jobs [post]
func (h *handler) RetryFailedJobs(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	principal := ctxval.PrincipalFromContext(ctx)
	if principal == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthorized", "")
	}

	result, err := h.retryUC.FailedJobs(ctx, principal)
	if err != nil {
		return err
	}
	return httpx.RespondJSON(w, r, http.StatusOK, result)
}

// RetryFailedMatchingEvaluations re-queues failed evaluation, report, resume and cover-letter workflows.
// @Summary Retry failed matching evaluations
// @Description Re-queues up to 50 evaluations with any failed workflow status. Cover letters use variant short. Machine API key only.
// @Tags maintenance
// @Produce json
// @Success 200 {object} models.RetryMatchingEvaluationsResult
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/maintenance/retry-failed-matching-evaluations [post]
func (h *handler) RetryFailedMatchingEvaluations(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	principal := ctxval.PrincipalFromContext(ctx)
	if principal == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthorized", "")
	}

	result, err := h.retryUC.FailedMatchingEvaluations(ctx, principal)
	if err != nil {
		return err
	}
	return httpx.RespondJSON(w, r, http.StatusOK, result)
}
