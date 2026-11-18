package me

import (
	"errors"
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// handler serves the authenticated user's own account details.
type handler struct {
	cfg    *config.Config
	userUC ucUsers.User
}

// NewHandler constructs the current-user handler from the full contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:    c.Cfg,
		userUC: c.Usecase.Users,
	}
}

// Route registers the account routes on the given /me group.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.userUC)

	g.GET("", h.GetMe, auth.ValidClerkSession)
	g.PATCH("", h.UpdateMe, auth.ValidClerkSession)
	g.POST("/deactivate", h.DeactivateMe, auth.ValidClerkSession)
}

// toMeResponse projects the internal user onto the public account DTO. The
// internal id and Clerk id are deliberately not both exposed.
func toMeResponse(u *models.User) models.MeResponse {
	return models.MeResponse{
		ID:                    u.ID,
		Email:                 u.Email,
		FullName:              u.FullName,
		Headline:              u.Headline,
		Phone:                 u.Phone,
		PortfolioURL:          u.PortfolioURL,
		LinkedInURL:           u.LinkedInURL,
		GitHubURL:             u.GitHubURL,
		Role:                  u.Role,
		OnboardingCompletedAt: u.OnboardingCompletedAt,
		CreatedAt:             u.CreatedAt,
		UpdatedAt:             u.UpdatedAt,
	}
}

// GetMe returns the authenticated user's account details. The internal user id
// is taken from the verified Clerk principal in the request context.
// @Summary Get current user
// @Description Return the authenticated user's account details. The internal id comes from the verified Clerk principal, never from the request.
// @Tags me
// @Accept json
// @Produce json
// @Success 200 {object} models.MeResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/me [get]
func (h *handler) GetMe(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	u, err := h.userUC.FindByID(r.Context(), p.ID)
	if err != nil {
		if errors.Is(err, constants.ErrUserNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "user not found", "")
		}
		return httpx.Envelope(httpx.CodeInternal, "failed to load account details", "")
	}

	return httpx.RespondJSON(w, r, http.StatusOK, toMeResponse(u))
}

// UpdateMe replaces the six self-editable profile fields. Email, role,
// onboarding state, and timestamps are not accepted from the client.
// @Summary Update current user
// @Description Replace the six editable account fields. All six are required; null clears a field. Email, role, and internal fields are read-only.
// @Tags me
// @Accept json
// @Produce json
// @Param body body models.UserProfileInput true "Editable account fields"
// @Success 200 {object} models.MeResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/me [patch]
func (h *handler) UpdateMe(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	input, errResp := decodeUpdateMeRequest(r)
	if errResp != nil {
		return errResp
	}

	u, err := h.userUC.UpdateProfile(r.Context(), p.ID, input)
	if err != nil {
		if errors.Is(err, constants.ErrUserNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "user not found", "")
		}
		return httpx.Envelope(httpx.CodeInternal, "failed to update account details", "")
	}

	return httpx.RespondJSON(w, r, http.StatusOK, toMeResponse(u))
}

// DeactivateMe soft-deletes the authenticated user's own account.
// @Summary Deactivate current user
// @Description Soft-delete the authenticated user's own account by setting deleted_at.
// @Tags me
// @Accept json
// @Produce json
// @Success 200 {object} models.MeResponse
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/me/deactivate [post]
func (h *handler) DeactivateMe(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	u, err := h.userUC.SoftDelete(r.Context(), p.ID)
	if err != nil {
		if errors.Is(err, constants.ErrUserNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "user not found", "")
		}
		return httpx.Envelope(httpx.CodeInternal, "failed to deactivate account", "")
	}

	return httpx.RespondJSON(w, r, http.StatusOK, toMeResponse(u))
}
