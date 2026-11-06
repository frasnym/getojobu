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
// @Success 200 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /ready [get]
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) error {
	if err := h.common.Health(r.Context()); err != nil {
		return httpx.RespondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
	}
	return httpx.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
