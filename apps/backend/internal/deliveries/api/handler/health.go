package handler

import (
	"net/http"
)

// Health handles GET /health and returns {"status":"ok"}.
func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		(&Base{}).RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
