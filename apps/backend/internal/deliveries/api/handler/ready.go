package handler

import (
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/common"
)

// Ready handles GET /ready and returns 200 when the database is reachable,
// 503 otherwise.
func Ready(common *common.Contract) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := common.Health(r.Context()); err != nil {
			(&Base{}).RespondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
			return
		}
		(&Base{}).RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
