package handler

import (
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// @Summary Health check
// @Description Returns service status
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} StatusResponse
// @Router /health [get]
func (h *handler) Health(w http.ResponseWriter, r *http.Request) error {
	return httpx.RespondJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}
