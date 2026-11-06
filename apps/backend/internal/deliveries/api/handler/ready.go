package handler

import (
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// Ready handles GET /ready and returns 200 when the database is reachable,
// 503 otherwise.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) error {
	if err := h.common.Health(r.Context()); err != nil {
		return httpx.RespondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
	}
	return httpx.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
