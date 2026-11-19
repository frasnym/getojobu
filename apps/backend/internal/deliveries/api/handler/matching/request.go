package matching

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// bulkGenerateEvaluationItemRequest is one item in the POST
// /v1/matching-evaluations/bulk-generate-evaluation request.
type bulkGenerateEvaluationItemRequest struct {
	ID     string `json:"id"`
	UserID string `json:"user_id,omitempty"`
}

// bulkGenerateEvaluationRequest is the array payload for POST
// /v1/matching-evaluations/bulk-generate-evaluation.
type bulkGenerateEvaluationRequest []bulkGenerateEvaluationItemRequest

// Validate checks array bounds and that every item carries a valid evaluation ID.
func (r bulkGenerateEvaluationRequest) Validate() error {
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
