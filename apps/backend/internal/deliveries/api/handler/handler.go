package handler

import (
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/handler/webhook"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	httpSwagger "github.com/swaggo/http-swagger"
)

// handler holds shared dependencies for every HTTP handler. Route methods
// have the signature func(http.ResponseWriter, *http.Request) error, so a
// handler can return an error instead of writing a response inline.
type handler struct {
	contract *contract.Contract
}

// New wires the handler set from the contract. contract.New guarantees Cfg,
// Common, Repository, and Usecase are non-nil, so they are used directly.
func New(c *contract.Contract) *handler {
	return &handler{contract: c}
}

// Route registers every HTTP route for this handler set.
// Clerk auth is applied per-route via authMiddleware (Echo-style),
// not as a global middleware chain.
// Usage: rt.GET("/protected", h.ProtectedHandler, auth.ValidClerkSession)
func (h *handler) Route(rt *httpx.Router) {
	// All API routes are grouped under /v1/.
	v1 := rt.Group("/v1")

	v1.GET("/health", h.Health)

	// Swagger UI at /v1/swagger/ (serves docs/swagger.json spec).
	// Trailing slash acts as a multi-segment wildcard in Go 1.22+ ServeMux,
	// matching /v1/swagger/, /v1/swagger/index.html, /v1/swagger/doc.json, etc.
	v1.GET("/swagger/", httpx.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		httpSwagger.Handler().ServeHTTP(w, r)
		return nil
	}))

	// Clerk webhook under /v1/webhooks group.
	webhook.NewHandler(h.contract).Route(v1.Group("/webhooks"))
}
