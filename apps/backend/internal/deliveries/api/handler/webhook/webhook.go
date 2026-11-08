package webhook

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
)

// handler handles Clerk webhook events.
type handler struct {
	cfg         *config.Config
	userService ucUsers.User
}

// NewHandler constructs a Clerk webhook handler from the full contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:         c.Cfg,
		userService: c.Usecase.User,
	}
}

// Route registers the webhook routes under the given group.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.userService)
	g.POST("/clerk", h.serveWebhook, auth.ValidClerkSvixSignature)
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
