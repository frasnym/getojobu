package handler

import (
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/webhook"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// @Summary Health check
// @Description Returns service status
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} webhook.StatusResponse
// @Router /v1/health [get]
func (h *handler) Health(w http.ResponseWriter, r *http.Request) error {
	return httpx.RespondJSON(w, http.StatusOK, webhook.StatusResponse{Status: "ok"})
}
