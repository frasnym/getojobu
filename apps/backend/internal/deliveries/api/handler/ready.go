package handler

import (
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// @Summary Readiness check
// @Description Returns 200 when dependencies (database) are reachable, 503 otherwise
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} StatusResponse
// @Failure 503 {object} StatusResponse
// @Router /ready [get]
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) error {
	if err := h.common.Health(r.Context()); err != nil {
		return httpx.RespondJSON(w, http.StatusServiceUnavailable, StatusResponse{Status: "not ready"})
	}
	return httpx.RespondJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}
