package scoringConfigs

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucScoringConfigs "github.com/frasnym/getojobu/apps/backend/internal/usecase/scoringConfigs"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// handler handles scoring configuration CRU operations.
type handler struct {
	cfg             *config.Config
	userUC          ucUsers.User
	ucScoringConfig ucScoringConfigs.ScoringConfig
}

// NewHandler constructs a scoring configuration handler from the full contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:             c.Cfg,
		userUC:          c.Usecase.Users,
		ucScoringConfig: c.Usecase.ScoringConfigs,
	}
}

// Route registers the scoring-config routes under the given group.
// All endpoints are strictly protected by ValidAPIKey (Clerk users cannot access).
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.userUC)

	g.POST("", h.CreateScoringConfig, auth.ValidAPIKey)
	g.GET("", h.ListScoringConfigs, auth.ValidAPIKey)
	g.GET("/active", h.GetActiveScoringConfig, auth.ValidAPIKey)
	g.GET("/{id}", h.GetScoringConfigByID, auth.ValidAPIKey)
	g.PATCH("/{id}", h.UpdateScoringConfig, auth.ValidAPIKey)
}

// CreateScoringConfig creates a new version of a scoring configuration for a job family.
// @Summary Create scoring configuration
// @Description Create a new version of a scoring configuration for a job family. Accessible only via machine API key.
// @Tags scoring_configs
// @Accept json
// @Produce json
// @Param config body models.CreateScoringConfigParams true "Scoring config payload"
// @Success 201 {object} models.ScoringConfig
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 409 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/scoring-configs [post]
func (h *handler) CreateScoringConfig(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req models.CreateScoringConfigParams
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	created, err := h.ucScoringConfig.Create(ctx, req)
	if err != nil {
		if errors.Is(err, constants.ErrDuplicateScoringConfigVersion) {
			return httpx.Envelope(httpx.CodeConflict, "scoring configuration version already exists for family", "")
		}
		return httpx.Envelope(httpx.CodeInvalidRequest, err.Error(), "")
	}

	return httpx.RespondJSON(w, r, http.StatusCreated, created)
}

// ListScoringConfigs lists scoring configurations filtered by family and active status.
// @Summary List scoring configurations
// @Description List scoring configurations filtered by family and active status. Accessible only via machine API key.
// @Tags scoring_configs
// @Produce json
// @Param family query string false "Filter by job family"
// @Param is_active query bool false "Filter by active state"
// @Success 200 {array} models.ScoringConfig
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/scoring-configs [get]
func (h *handler) ListScoringConfigs(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	query := r.URL.Query()
	var params models.ListScoringConfigsParams

	if family := strings.TrimSpace(query.Get("family")); family != "" {
		params.Family = &family
	}

	if isActiveStr := strings.TrimSpace(query.Get("is_active")); isActiveStr != "" {
		val, err := strconv.ParseBool(isActiveStr)
		if err != nil {
			return httpx.Envelope(httpx.CodeInvalidRequest, "invalid is_active query parameter, must be true or false", "")
		}
		params.IsActive = &val
	}

	list, err := h.ucScoringConfig.List(ctx, params)
	if err != nil {
		return httpx.Envelope(httpx.CodeInternal, "failed to list scoring configurations", "")
	}

	return httpx.RespondJSON(w, r, http.StatusOK, list)
}

// GetActiveScoringConfig retrieves the active scoring configuration for a job family.
// @Summary Get active scoring configuration
// @Description Retrieve the active scoring configuration for a job family. Accessible only via machine API key.
// @Tags scoring_configs
// @Produce json
// @Param family query string true "Job family key (e.g. technology, transportation)"
// @Success 200 {object} models.ScoringConfig
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/scoring-configs/active [get]
func (h *handler) GetActiveScoringConfig(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	family := strings.TrimSpace(r.URL.Query().Get("family"))
	if family == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "family query parameter is required", "")
	}

	cfg, err := h.ucScoringConfig.GetActiveByFamily(ctx, family)
	if err != nil {
		if errors.Is(err, constants.ErrScoringConfigNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "no active scoring configuration found for family", "")
		}
		return httpx.Envelope(httpx.CodeInternal, "failed to retrieve active scoring configuration", "")
	}

	return httpx.RespondJSON(w, r, http.StatusOK, cfg)
}

// GetScoringConfigByID retrieves a scoring configuration by its UUID.
// @Summary Get scoring configuration by ID
// @Description Retrieve a scoring configuration by its UUID. Accessible only via machine API key.
// @Tags scoring_configs
// @Produce json
// @Param id path string true "Scoring config UUID"
// @Success 200 {object} models.ScoringConfig
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/scoring-configs/{id} [get]
func (h *handler) GetScoringConfigByID(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid scoring config UUID", "")
	}

	cfg, err := h.ucScoringConfig.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, constants.ErrScoringConfigNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "scoring configuration not found", "")
		}
		return httpx.Envelope(httpx.CodeInternal, "failed to retrieve scoring configuration", "")
	}

	return httpx.RespondJSON(w, r, http.StatusOK, cfg)
}

// UpdateScoringConfig updates an existing scoring configuration (e.g. activating it or modifying categories).
// @Summary Update scoring configuration
// @Description Update an existing scoring configuration. Accessible only via machine API key.
// @Tags scoring_configs
// @Accept json
// @Produce json
// @Param id path string true "Scoring config UUID"
// @Param config body models.UpdateScoringConfigParams true "Update payload"
// @Success 200 {object} models.ScoringConfig
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/scoring-configs/{id} [patch]
func (h *handler) UpdateScoringConfig(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	p := ctxval.PrincipalFromContext(ctx)
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return httpx.Envelope(httpx.CodeInvalidRequest, "invalid scoring config UUID", "")
	}

	var req models.UpdateScoringConfigParams
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	updated, err := h.ucScoringConfig.Update(ctx, id, req)
	if err != nil {
		if errors.Is(err, constants.ErrScoringConfigNotFound) {
			return httpx.Envelope(httpx.CodeNotFound, "scoring configuration not found", "")
		}
		return httpx.Envelope(httpx.CodeInvalidRequest, err.Error(), "")
	}

	return httpx.RespondJSON(w, r, http.StatusOK, updated)
}
