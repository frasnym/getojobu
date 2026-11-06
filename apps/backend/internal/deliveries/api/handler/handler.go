package handler

import (
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/common"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// Handler holds shared dependencies for every HTTP handler. Route methods
// have the signature func(http.ResponseWriter, *http.Request) error, so a
// handler can return an error instead of writing a response inline.
type Handler struct {
	common *common.Contract
}

// New wires the handler set from the contract.
func New(c *contract.Contract) *Handler {
	return &Handler{common: c.Common}
}

// Route registers every HTTP route for this handler set.
func (h *Handler) Route(rt *httpx.Router) {
	rt.GET("/health", h.Health)
	rt.GET("/ready", h.Ready)
}
