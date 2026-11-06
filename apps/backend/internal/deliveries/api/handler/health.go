package handler

import (
	"net/http"

	httperf "github.com/frasnym/getojobu/apps/backend/internal/utils/http"
)

// Health handles GET /health and returns {"status":"ok"}.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) error {
	return httperf.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
