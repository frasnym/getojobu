package handler

import (
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/contract/common"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	httpSwagger "github.com/swaggo/http-swagger"
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

	// Swagger UI at /swagger/ (serves docs/swagger.json spec).
	// Trailing slash acts as a multi-segment wildcard in Go 1.22+ ServeMux,
	// matching /swagger/, /swagger/index.html, /swagger/doc.json, etc.
	rt.GET("/swagger/", httpx.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		httpSwagger.Handler().ServeHTTP(w, r)
		return nil
	}))
}
