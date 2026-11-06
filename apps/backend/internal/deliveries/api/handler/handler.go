package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
	httpSwagger "github.com/swaggo/http-swagger"
)

// handler holds shared dependencies for every HTTP handler. Route methods
// have the signature func(http.ResponseWriter, *http.Request) error, so a
// handler can return an error instead of writing a response inline.
type handler struct {
	cfg         *config.Config
	userService ucUsers.User
}

// New wires the handler set from the contract. contract.New guarantees Cfg,
// Common, Repository, and Usecase are non-nil, so they are used directly.
func New(c *contract.Contract) *handler {
	return &handler{
		cfg:         c.Cfg,
		userService: c.Usecase.User,
	}
}

// Route registers every HTTP route for this handler set.
// Clerk auth is applied per-route via authMiddleware (Echo-style),
// not as a global middleware chain.
// Usage: rt.GET("/protected", h.ProtectedHandler, auth.ValidClerkSession)
func (h *handler) Route(rt *httpx.Router) {
	rt.GET("/health", h.Health)

	// Swagger UI at /swagger/ (serves docs/swagger.json spec).
	// Trailing slash acts as a multi-segment wildcard in Go 1.22+ ServeMux,
	// matching /swagger/, /swagger/index.html, /swagger/doc.json, etc.
	rt.GET("/swagger/", httpx.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		httpSwagger.Handler().ServeHTTP(w, r)
		return nil
	}))

	auth := middleware.NewAuthMiddleware(h.cfg, h.userService)

	// Clerk webhook endpoint (no Clerk auth — verified via Svix signature).
	rt.POST("/webhooks/clerk", h.serveWebhook, auth.ValidClerkSvixSignature)
}

// serveWebhook dispatches the Clerk webhook event. Svix signature verification
// is done by middleware before this handler runs.
func (h *handler) serveWebhook(w http.ResponseWriter, r *http.Request) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "failed to read body", "")
	}

	var event clerkWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid webhook payload", "")
	}

	switch event.Type {
	case "user.created":
		return h.handleUserCreated(w, r.Context(), event)
	case "user.updated":
		return h.handleUserUpdated(w, r.Context(), event)
	case "user.deleted":
		return h.handleUserDeleted(w, r.Context(), event)
	default:
		return httpx.RespondJSON(w, http.StatusOK, StatusResponse{Status: "unhandled"})
	}
}

func (h *handler) handleUserCreated(w http.ResponseWriter, ctx context.Context, event clerkWebhookEvent) error {
	email, fullName := extractEmailAndName(event)

	if _, err := h.userService.Create(ctx, event.Data.ID, email, fullName, models.RoleUser); err != nil {
		return httpx.Envelope(httpx.CodeInternal, "failed to create user", "")
	}

	return httpx.RespondJSON(w, http.StatusOK, StatusResponse{Status: "created"})
}

func (h *handler) handleUserUpdated(w http.ResponseWriter, ctx context.Context, event clerkWebhookEvent) error {
	email, fullName := extractEmailAndName(event)

	if _, err := h.userService.Update(ctx, event.Data.ID, email, fullName); err != nil {
		return httpx.Envelope(httpx.CodeInternal, "failed to update user", "")
	}

	return httpx.RespondJSON(w, http.StatusOK, StatusResponse{Status: "updated"})
}

func (h *handler) handleUserDeleted(w http.ResponseWriter, ctx context.Context, event clerkWebhookEvent) error {
	if _, err := h.userService.SoftDelete(ctx, event.Data.ID); err != nil {
		return httpx.Envelope(httpx.CodeInternal, "failed to delete user", "")
	}

	return httpx.RespondJSON(w, http.StatusOK, StatusResponse{Status: "deleted"})
}

func extractEmailAndName(event clerkWebhookEvent) (email, fullName string) {
	for _, ea := range event.Data.EmailAddresses {
		if ea.ID == event.Data.PrimaryEmailAddressID {
			email = ea.EmailAddress
			break
		}
	}

	parts := make([]string, 0, 2)
	if event.Data.FirstName != "" {
		parts = append(parts, event.Data.FirstName)
	}
	if event.Data.LastName != "" {
		parts = append(parts, event.Data.LastName)
	}
	fullName = strings.Join(parts, " ")

	return email, fullName
}
