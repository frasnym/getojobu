package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// clerkWebhookEvent is the payload of a Clerk webhook event.
type clerkWebhookEvent struct {
	Type string `json:"type"`
	Data struct {
		ID                    string `json:"id"`
		PrimaryEmailAddressID string `json:"primary_email_address_id"`
		DeletedAt             string `json:"deleted_at"`
		EmailAddresses        []struct {
			ID           string `json:"id"`
			EmailAddress string `json:"email_address"`
		} `json:"email_addresses"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	} `json:"data"`
}

// StatusResponse is the body returned by webhook endpoints.
type StatusResponse struct {
	Status string `json:"status"`
}

// Handler handles Clerk webhook events.
type Handler struct {
	userService ucUsers.User
}

// NewHandler constructs a Clerk webhook handler.
func NewHandler(userService ucUsers.User) *Handler {
	return &Handler{userService: userService}
}

// ServeWebhook dispatches the Clerk webhook event. Svix signature verification
// is done by middleware before this handler runs.
func (h *Handler) ServeWebhook(w http.ResponseWriter, r *http.Request) error {
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

func (h *Handler) handleUserCreated(w http.ResponseWriter, ctx context.Context, event clerkWebhookEvent) error {
	email, fullName := extractEmailAndName(event)

	if _, err := h.userService.Create(ctx, event.Data.ID, email, fullName, models.RoleUser); err != nil {
		return httpx.Envelope(httpx.CodeInternal, "failed to create user", "")
	}

	return httpx.RespondJSON(w, http.StatusOK, StatusResponse{Status: "created"})
}

func (h *Handler) handleUserUpdated(w http.ResponseWriter, ctx context.Context, event clerkWebhookEvent) error {
	email, fullName := extractEmailAndName(event)

	if _, err := h.userService.Update(ctx, event.Data.ID, email, fullName); err != nil {
		return httpx.Envelope(httpx.CodeInternal, "failed to update user", "")
	}

	return httpx.RespondJSON(w, http.StatusOK, StatusResponse{Status: "updated"})
}

func (h *Handler) handleUserDeleted(w http.ResponseWriter, ctx context.Context, event clerkWebhookEvent) error {
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
