package matching

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/dto"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucMatching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

type bulkGenerateCoverLetterItemRequest struct {
	ID      string `json:"id"`
	Variant string `json:"variant"`
	Force   bool   `json:"force"`
}

type bulkGenerateCoverLetterRequest []bulkGenerateCoverLetterItemRequest

func (r bulkGenerateCoverLetterRequest) Validate() error {
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
		if !constants.IsValidCoverLetterVariant(strings.TrimSpace(item.Variant)) {
			return httpx.Envelope(httpx.CodeInvalidRequest, fmt.Sprintf("items[%d].variant must be short or full", i), "")
		}
	}
	return nil
}

// BulkQueueGenerateCoverLetter queues cover letter generation for multiple evaluations.
// @Summary Bulk queue cover letter generation
// @Description Queues short or full cover letter generation for multiple evaluations. Clerk users may only enqueue evaluations they own; worker API key callers may enqueue any evaluation.
// @Tags matching
// @Accept json
// @Produce json
// @Param items body bulkGenerateCoverLetterRequest true "Evaluations to generate cover letters for"
// @Success 202 {array} models.MatchingEvaluationSummary
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/matching-evaluations/bulk-generate-cover-letter [post]
func (h *handler) BulkQueueGenerateCoverLetter(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	principal := ctxval.PrincipalFromContext(ctx)
	if principal == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthorized", "")
	}

	var req bulkGenerateCoverLetterRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}
	if err := req.Validate(); err != nil {
		return err
	}

	items := make([]models.BulkGenerateCoverLetterItem, len(req))
	for i, item := range req {
		id, _ := uuid.Parse(item.ID)
		items[i] = models.BulkGenerateCoverLetterItem{
			ID:      id,
			Variant: strings.TrimSpace(item.Variant),
			Force:   item.Force,
		}
	}

	summaries, err := h.matchingUC.BulkQueueCoverLetter(ctx, items, principal)
	if err != nil {
		if errors.Is(err, constants.ErrUnauthorized) {
			return httpx.Envelope(httpx.CodeUnauthorized, "unauthorized", "")
		}
		return err
	}
	return httpx.RespondJSON(w, r, http.StatusAccepted, summaries)
}

type updateCoverLetterPayload struct {
	CoverLetterStatus         string          `json:"cover_letter_status"`
	CoverLetterContent        json.RawMessage `json:"cover_letter_content,omitempty" swaggertype:"object"`
	CoverLetterFailureMessage *string         `json:"cover_letter_failure_message,omitempty"`
}

// UpdateCoverLetter records cover letter generation status and content from the worker.
// @Summary Update cover letter generation status
// @Description Records cover letter status, generated content, or a failure. Failed attempts append a safe, fixed message. Accessible only via machine API key.
// @Tags matching
// @Accept json
// @Produce json
// @Param id path string true "Matching evaluation ID (UUID)"
// @Param payload body updateCoverLetterPayload true "Cover letter status update payload"
// @Success 200 {object} dto.MatchingEvaluationResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/matching-evaluations/{id}/cover-letter [patch]
func (h *handler) UpdateCoverLetter(w http.ResponseWriter, r *http.Request) error {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid evaluation id", "")
	}

	var payload updateCoverLetterPayload
	if err := httpx.ParseJSON(r, &payload); err != nil {
		return err
	}
	status := strings.TrimSpace(payload.CoverLetterStatus)
	if status == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "cover_letter_status is required", "")
	}
	if !constants.IsValidCoverLetterStatus(status) {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid cover_letter_status", "")
	}

	// A success without content would otherwise keep stale content from a
	// previous attempt while flipping the status to success.
	if status == string(constants.CoverLetterStatusSuccess) && len(payload.CoverLetterContent) == 0 {
		return httpx.Envelope(httpx.CodeInvalidRequest, "cover_letter_content is required for success", "")
	}

	var content *models.CoverLetterContent
	if len(payload.CoverLetterContent) > 0 {
		var parsed models.CoverLetterContent
		if err := json.Unmarshal(payload.CoverLetterContent, &parsed); err != nil {
			return httpx.Envelope(httpx.CodeInvalidRequest, "invalid cover_letter_content", "")
		}
		if status == string(constants.CoverLetterStatusSuccess) && strings.TrimSpace(parsed.Text) == "" {
			return httpx.Envelope(httpx.CodeInvalidRequest, "invalid cover_letter_content: text is required", "")
		}
		content = &parsed
	}

	eval, err := h.matchingUC.UpdateEvaluationCoverLetter(r.Context(), id, ucMatching.UpdateEvaluationCoverLetterRequest{
		CoverLetterStatus:         status,
		CoverLetterContent:        content,
		CoverLetterFailureMessage: payload.CoverLetterFailureMessage,
	})
	if err != nil {
		switch {
		case errors.Is(err, constants.ErrNotFound):
			return httpx.Envelope(httpx.CodeNotFound, "evaluation not found", "")
		case errors.Is(err, constants.ErrInvalidRequest):
			return httpx.Envelope(httpx.CodeInvalidRequest, err.Error(), "")
		default:
			return err
		}
	}

	return httpx.RespondJSON(w, r, http.StatusOK, dto.NewMatchingEvaluationResponse(eval))
}
