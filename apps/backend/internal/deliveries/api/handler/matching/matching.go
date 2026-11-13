package matching

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucMatching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
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
}

type updateEvaluationPayload struct {
	Status         string                        `json:"status"`
	EvaluationData *models.MatchEvaluationOutput `json:"evaluation_data,omitempty"`
	FailureMessage *string                       `json:"failure_message,omitempty"`
	Provider       *string                       `json:"provider,omitempty"`
	Model          *string                       `json:"model,omitempty"`
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
		Status:         payload.Status,
		EvaluationData: evalDataBytes,
		FailureMessage: payload.FailureMessage,
		Provider:       payload.Provider,
		Model:          payload.Model,
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
