package matching

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucMatching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

type handler struct {
	cfg        *config.Config
	userUC     ucUsers.User
	matchingUC ucMatching.Matching
}

// NewHandler constructs a matching handler from the contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:        c.Cfg,
		userUC:     c.Usecase.Users,
		matchingUC: c.Usecase.Matching,
	}
}

// Route registers the matching routes under the given router.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.userUC)

	// PATCH /v1/matching-evaluations/{id} (strictly worker API key)
	g.PATCH("/{id}", h.UpdateEvaluation, auth.ValidAPIKey)

	// PATCH /v1/matching-evaluations/{id}/report (strictly worker API key)
	g.PATCH("/{id}/report", h.UpdateReport, auth.ValidAPIKey)

	// GET /v1/matching-evaluations/{id}/report (Clerk user session or worker API key)
	g.GET("/{id}/report", h.GetReport, auth.ValidClerkOrAPIKey)

	// POST /v1/matching-evaluations/bulk-generate-report (strictly worker API key)
	g.POST("/bulk-generate-report", h.BulkQueueGenerateReport, auth.ValidAPIKey)
}

type updateEvaluationPayload struct {
	EvaluationStatus string                        `json:"evaluation_status"`
	EvaluationData   *models.MatchEvaluationOutput `json:"evaluation_data,omitempty"`
	FailureMessage   *string                       `json:"failure_message,omitempty"`
}

// UpdateEvaluation updates the matching evaluation status and results.
// @Summary Update matching evaluation
// @Description Updates the matching evaluation status and results. Accessible only via machine API key.
// @Tags matching
// @Accept json
// @Produce json
// @Param id path string true "Matching evaluation ID (UUID)"
// @Param payload body updateEvaluationPayload true "Evaluation update payload"
// @Success 200 {object} models.MatchingEvaluation
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/matching-evaluations/{id} [patch]
func (h *handler) UpdateEvaluation(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid evaluation id", "")
	}

	var payload updateEvaluationPayload
	if err := httpx.ParseJSON(r, &payload); err != nil {
		return err
	}

	var evalDataBytes []byte
	if payload.EvaluationData != nil {
		evalDataBytes, err = json.Marshal(payload.EvaluationData)
		if err != nil {
			return httpx.Envelope(httpx.CodeInvalidRequest, "invalid evaluation_data", "")
		}
	}

	req := ucMatching.UpdateEvaluationRequest{
		EvaluationStatus: payload.EvaluationStatus,
		EvaluationData:   evalDataBytes,
		FailureMessage:   payload.FailureMessage,
	}

	eval, err := h.matchingUC.UpdateEvaluation(ctx, id, req)
	if err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "evaluation not found", "")
		}
		if errors.Is(err, constants.ErrInvalidRequest) || errors.Is(err, constants.ErrInvalidTransition) {
			return httpx.Envelope(httpx.CodeInvalidRequest, err.Error(), "")
		}
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusOK, eval)
}

type updateReportPayload struct {
	ReportStatus         string          `json:"report_status"`
	ReportData           json.RawMessage `json:"report_data,omitempty" swaggertype:"object"`
	ReportFailureMessage *string         `json:"report_failure_message,omitempty"`
}

// UpdateReport updates the match report data and status on an evaluation.
// @Summary Update match report
// @Description Updates the match report data and status. Accessible only via machine API key.
// @Tags matching
// @Accept json
// @Produce json
// @Param id path string true "Matching evaluation ID (UUID)"
// @Param payload body updateReportPayload true "Report update payload"
// @Success 200 {object} models.MatchingEvaluation
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/matching-evaluations/{id}/report [patch]
func (h *handler) UpdateReport(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid evaluation id", "")
	}

	var payload updateReportPayload
	if err := httpx.ParseJSON(r, &payload); err != nil {
		return err
	}

	status := constants.ReportStatus(strings.TrimSpace(payload.ReportStatus))
	if status == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "report_status is required", "")
	}
	switch status {
	case constants.ReportStatusPending, constants.ReportStatusPendingRetry, constants.ReportStatusGenerated, constants.ReportStatusFailed:
	default:
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid report_status", "")
	}
	if status == constants.ReportStatusGenerated && len(payload.ReportData) == 0 {
		return httpx.Envelope(httpx.CodeInvalidRequest, "report_data is required when status is generated", "")
	}

	req := ucMatching.UpdateEvaluationReportRequest{
		ReportStatus:         string(status),
		ReportData:           payload.ReportData,
		ReportFailureMessage: payload.ReportFailureMessage,
	}

	eval, err := h.matchingUC.UpdateEvaluationReport(ctx, id, req)
	if err != nil {
		if errors.Is(err, constants.ErrNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "evaluation not found", "")
		}
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusOK, eval)
}

// GetReport returns the candidate-facing match report for an evaluation.
// @Summary Get match report
// @Description Returns the candidate-facing match report. If report generation is still in progress, returns pending status.
// @Tags matching
// @Produce json
// @Param id path string true "Matching evaluation ID (UUID)"
// @Success 200 {object} models.MatchingEvaluationReportResponse
// @Failure 401 {object} httpx.Error
// @Failure 403 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/matching-evaluations/{id}/report [get]
func (h *handler) GetReport(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthorized", "")
	}

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid evaluation id", "")
	}

	report, err := h.matchingUC.GetReport(ctx, id, p)
	if err != nil {
		if errors.Is(err, constants.ErrUnauthorized) {
			return httpx.Envelope(httpx.CodeUnauthorized, "unauthorized", "")
		}
		if errors.Is(err, constants.ErrNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "evaluation not found", "")
		}
		if errors.Is(err, constants.ErrForbidden) {
			return httpx.Envelope(httpx.CodeForbidden, "access denied", "")
		}
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusOK, report)
}

type bulkGenerateReportItemRequest struct {
	ID    string `json:"id"`
	Force bool   `json:"force"`
}

type bulkGenerateReportRequest []bulkGenerateReportItemRequest

func (r bulkGenerateReportRequest) Validate() error {
	if len(r) == 0 {
		return httpx.Envelope(httpx.CodeInvalidRequest, "request body must be a non-empty array", "")
	}
	if len(r) > 100 {
		return httpx.Envelope(httpx.CodeInvalidRequest, "batch size must not exceed 100 items", "")
	}
	for i, item := range r {
		if strings.TrimSpace(item.ID) == "" {
			return httpx.Envelope(httpx.CodeInvalidRequest, fmt.Sprintf("items[%d].id is required", i), "")
		}
		if _, err := uuid.Parse(item.ID); err != nil {
			return httpx.Envelope(httpx.CodeInvalidRequest, fmt.Sprintf("items[%d].id must be a valid UUID", i), "")
		}
	}
	return nil
}

// BulkQueueGenerateReport queues match report generation for multiple evaluations.
// @Summary Bulk queue match report generation
// @Description Queues match report generation for multiple evaluations in bulk. Accessible only via machine API key.
// @Tags matching
// @Accept json
// @Produce json
// @Param items body bulkGenerateReportRequest true "Evaluations to generate reports for"
// @Success 202 {array} models.MatchingEvaluationSummary
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/matching-evaluations/bulk-generate-report [post]
func (h *handler) BulkQueueGenerateReport(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthorized", "")
	}

	var req bulkGenerateReportRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if err := req.Validate(); err != nil {
		return err
	}

	items := make([]models.BulkGenerateReportItem, len(req))
	for i, item := range req {
		u, _ := uuid.Parse(item.ID)
		items[i] = models.BulkGenerateReportItem{
			ID:    u,
			Force: item.Force,
		}
	}

	summaries, err := h.matchingUC.BulkQueueGenerateReport(ctx, items, p)
	if err != nil {
		if errors.Is(err, constants.ErrUnauthorized) {
			return httpx.Envelope(httpx.CodeUnauthorized, "unauthorized", "")
		}
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusAccepted, summaries)
}
