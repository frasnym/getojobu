package experiences

import (
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucExperiences "github.com/frasnym/getojobu/apps/backend/internal/usecase/experiences"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// handler handles experience CRU operations.
type handler struct {
	cfg    *config.Config
	userUC ucUsers.User
	ucExp  ucExperiences.Experience
}

// NewHandler constructs an experience handler from the full contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:    c.Cfg,
		userUC: c.Usecase.Users,
		ucExp:  c.Usecase.Experiences,
	}
}

// Route registers the experience routes under the given group.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.userUC)

	g.POST("", h.CreateExperience, auth.ValidClerkSession)
	g.GET("", h.ListExperiences, auth.ValidClerkSession)
	g.GET("/{id}", h.GetExperience, auth.ValidClerkSession)
	g.PUT("/{id}", h.UpdateExperience, auth.ValidClerkSession)
}

type createExperienceRequest struct {
	ExperienceType string `json:"experience_type"`
	Organization   string `json:"organization"`
	RoleTitle      string `json:"role_title"`
	Location       string `json:"location"`
	Description    string `json:"description"`
	StartedAt      string `json:"started_at"`
	EndedAt        string `json:"ended_at"`
	IsCurrent      bool   `json:"is_current"`
}

type updateExperienceRequest struct {
	ExperienceType string `json:"experience_type"`
	Organization   string `json:"organization"`
	RoleTitle      string `json:"role_title"`
	Location       string `json:"location"`
	Description    string `json:"description"`
	StartedAt      string `json:"started_at"`
	EndedAt        string `json:"ended_at"`
	IsCurrent      bool   `json:"is_current"`
}

// CreateExperience creates a new experience for the authenticated user.
// @Summary Create experience
// @Description Create a new career experience for the authenticated user
// @Tags experiences
// @Accept json
// @Produce json
// @Param experience body createExperienceRequest true "Experience data"
// @Success 201 {object} models.Experience
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/experiences [post]
func (h *handler) CreateExperience(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req createExperienceRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if req.ExperienceType == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "experience_type is required", "")
	}
	expType := models.ExperienceType(req.ExperienceType)
	if !expType.IsValid() {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid experience_type", "")
	}

	e, err := h.ucExp.Create(r.Context(), models.CreateExperienceParams{
		UserID:         p.ID,
		ExperienceType: expType,
		Organization:   req.Organization,
		RoleTitle:      req.RoleTitle,
		Location:       req.Location,
		Description:    req.Description,
		StartedAt:      utils.PtrString(req.StartedAt),
		EndedAt:        utils.PtrString(req.EndedAt),
		IsCurrent:      req.IsCurrent,
	})
	if err != nil {
		return err
	}

	return httpx.RespondJSON(w, http.StatusCreated, e)
}

// GetExperience returns a single experience by id for the authenticated user.
// @Summary Get experience
// @Description Get a career experience by id
// @Tags experiences
// @Produce json
// @Param id path string true "Experience ID"
// @Success 200 {object} models.Experience
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/experiences/{id} [get]
func (h *handler) GetExperience(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	id := httpx.ExtractPathParam(r, "id")
	if id == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "experience id is required", "")
	}

	e, err := h.ucExp.GetByID(r.Context(), models.GetExperienceParams{
		UserID: p.ID,
		ID:     id,
	})
	if err != nil {
		if err == constants.ErrNotFound {
			return httpx.Envelope(httpx.CodeNotFound, "experience not found", "")
		}
		return err
	}

	return httpx.RespondJSON(w, http.StatusOK, e)
}

// ListExperiences returns all experiences for the authenticated user.
// @Summary List experiences
// @Description List all career experiences for the authenticated user
// @Tags experiences
// @Produce json
// @Success 200 {array} models.Experience
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/experiences [get]
func (h *handler) ListExperiences(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	experiences, err := h.ucExp.ListByUser(r.Context(), p.ID)
	if err != nil {
		return err
	}

	return httpx.RespondJSON(w, http.StatusOK, experiences)
}

// UpdateExperience modifies an existing experience for the authenticated user.
// @Summary Update experience
// @Description Update a career experience by id
// @Tags experiences
// @Accept json
// @Produce json
// @Param id path string true "Experience ID"
// @Param experience body updateExperienceRequest true "Experience data"
// @Success 200 {object} models.Experience
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/experiences/{id} [put]
func (h *handler) UpdateExperience(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	id := httpx.ExtractPathParam(r, "id")
	if id == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "experience id is required", "")
	}

	var req updateExperienceRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if req.ExperienceType == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "experience_type is required", "")
	}
	expType := models.ExperienceType(req.ExperienceType)
	if !expType.IsValid() {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid experience_type", "")
	}

	e, err := h.ucExp.Update(r.Context(), models.UpdateExperienceParams{
		UserID:         p.ID,
		ID:             id,
		ExperienceType: expType,
		Organization:   req.Organization,
		RoleTitle:      req.RoleTitle,
		Location:       req.Location,
		Description:    req.Description,
		StartedAt:      utils.PtrString(req.StartedAt),
		EndedAt:        utils.PtrString(req.EndedAt),
		IsCurrent:      req.IsCurrent,
	})
	if err != nil {
		if err == constants.ErrNotFound {
			return httpx.Envelope(httpx.CodeNotFound, "experience not found", "")
		}
		return err
	}

	return httpx.RespondJSON(w, http.StatusOK, e)
}
