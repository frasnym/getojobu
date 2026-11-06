package handler

import (
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// Health handles GET /health and returns {"status":"ok"}.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) error {
	return httpx.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
