package projects

import (
	"net/http"

	"github.com/frasnym/getojobu/apps/backend/internal/config"
	"github.com/frasnym/getojobu/apps/backend/internal/contract"
	"github.com/frasnym/getojobu/apps/backend/internal/deliveries/api/middleware"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	ucProjects "github.com/frasnym/getojobu/apps/backend/internal/usecase/projects"
	ucProjectSkills "github.com/frasnym/getojobu/apps/backend/internal/usecase/projectskills"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/ctxval"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/httpx"
)

// handler handles project CRU and project-skill link operations.
type handler struct {
	cfg          *config.Config
	userUC       ucUsers.User
	ucProj       ucProjects.Project
	ucProjSkills ucProjectSkills.ProjectSkill
}

// NewHandler constructs a project handler from the full contract.
func NewHandler(c *contract.Contract) *handler {
	return &handler{
		cfg:          c.Cfg,
		userUC:       c.Usecase.Users,
		ucProj:       c.Usecase.Projects,
		ucProjSkills: c.Usecase.ProjectSkills,
	}
}

// Route registers the project routes under the given group.
func (h *handler) Route(g *httpx.Router) {
	auth := middleware.NewAuthMiddleware(h.cfg, h.userUC)

	g.POST("", h.CreateProject, auth.ValidClerkSession)
	g.GET("", h.ListProjects, auth.ValidClerkSession)
	g.GET("/{id}", h.GetProject, auth.ValidClerkSession)
	g.PUT("/{id}", h.UpdateProject, auth.ValidClerkSession)

	g.POST("/{id}/skills", h.LinkProjectSkills, auth.ValidClerkSession)
	g.GET("/{id}/skills", h.ListProjectSkills, auth.ValidClerkSession)
}

type createProjectRequest struct {
	UserExperienceID string `json:"user_experience_id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Responsibilities string `json:"responsibilities"`
	Achievements     string `json:"achievements"`
	StartedAt        string `json:"started_at"`
	EndedAt          string `json:"ended_at"`
	IsCurrent        bool   `json:"is_current"`
}

type updateProjectRequest struct {
	UserExperienceID string `json:"user_experience_id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Responsibilities string `json:"responsibilities"`
	Achievements     string `json:"achievements"`
	StartedAt        string `json:"started_at"`
	EndedAt          string `json:"ended_at"`
	IsCurrent        bool   `json:"is_current"`
}

type linkProjectSkillsRequest struct {
	SkillIDs []string `json:"skill_ids"`
}

// CreateProject creates a new project for the authenticated user.
// @Summary Create project
// @Description Create a new project for the authenticated user
// @Tags projects
// @Accept json
// @Produce json
// @Param project body createProjectRequest true "Project data"
// @Success 201 {object} models.Project
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/projects [post]
func (h *handler) CreateProject(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	var req createProjectRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if req.Name == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "name is required", "")
	}

	proj, err := h.ucProj.Create(r.Context(), models.CreateProjectParams{
		UserID:           p.ID,
		Name:             req.Name,
		Description:      req.Description,
		Responsibilities: req.Responsibilities,
		Achievements:     req.Achievements,
		UserExperienceID: utils.PtrString(req.UserExperienceID),
		StartedAt:        utils.PtrString(req.StartedAt),
		EndedAt:          utils.PtrString(req.EndedAt),
		IsCurrent:        req.IsCurrent,
	})
	if err != nil {
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusCreated, proj)
}

// GetProject returns a single project by id for the authenticated user.
// @Summary Get project
// @Description Get a project by id
// @Tags projects
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {object} models.Project
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/projects/{id} [get]
func (h *handler) GetProject(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	id := httpx.ExtractPathParam(r, "id")
	if id == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "project id is required", "")
	}

	proj, err := h.ucProj.GetByID(r.Context(), models.GetProjectParams{
		UserID: p.ID,
		ID:     id,
	})
	if err != nil {
		if err == constants.ErrNotFound {
			return httpx.Envelope(httpx.CodeNotFound, "project not found", "")
		}
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusOK, proj)
}

// ListProjects returns all projects for the authenticated user.
// @Summary List projects
// @Description List all projects for the authenticated user
// @Tags projects
// @Produce json
// @Success 200 {array} models.Project
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/projects [get]
func (h *handler) ListProjects(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	projects, err := h.ucProj.ListByUser(r.Context(), p.ID)
	if err != nil {
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusOK, projects)
}

// UpdateProject modifies an existing project for the authenticated user.
// @Summary Update project
// @Description Update a project by id
// @Tags projects
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param project body updateProjectRequest true "Project data"
// @Success 200 {object} models.Project
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @Failure 404 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/projects/{id} [put]
func (h *handler) UpdateProject(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	id := httpx.ExtractPathParam(r, "id")
	if id == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "project id is required", "")
	}

	var req updateProjectRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if req.Name == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "name is required", "")
	}

	proj, err := h.ucProj.Update(r.Context(), models.UpdateProjectParams{
		UserID:           p.ID,
		ID:               id,
		Name:             req.Name,
		Description:      req.Description,
		Responsibilities: req.Responsibilities,
		Achievements:     req.Achievements,
		UserExperienceID: utils.PtrString(req.UserExperienceID),
		StartedAt:        utils.PtrString(req.StartedAt),
		EndedAt:          utils.PtrString(req.EndedAt),
		IsCurrent:        req.IsCurrent,
	})
	if err != nil {
		if err == constants.ErrNotFound {
			return httpx.Envelope(httpx.CodeNotFound, "project not found", "")
		}
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusOK, proj)
}

// LinkProjectSkills associates skills with a project owned by the authenticated user.
// @Summary Link project skills
// @Description Associate skills with a project
// @Tags project-skills
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param skills body linkProjectSkillsRequest true "Skill IDs"
// @Success 200 {object} map[string]string
// @Failure 400 {object} httpx.Error
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/projects/{id}/skills [post]
func (h *handler) LinkProjectSkills(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	id := httpx.ExtractPathParam(r, "id")
	if id == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "project id is required", "")
	}

	var req linkProjectSkillsRequest
	if err := httpx.ParseJSON(r, &req); err != nil {
		return err
	}

	if len(req.SkillIDs) == 0 {
		return httpx.Envelope(httpx.CodeInvalidRequest, "skill_ids is required", "")
	}

	if err := h.ucProjSkills.LinkSkills(r.Context(), models.LinkProjectSkillsParams{
		UserID:    p.ID,
		ProjectID: id,
		SkillIDs:  req.SkillIDs,
	}); err != nil {
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusOK, map[string]string{"status": "linked"})
}

// ListProjectSkills returns the skills linked to a project owned by the authenticated user.
// @Summary List project skills
// @Description List skills linked to a project
// @Tags project-skills
// @Produce json
// @Param id path string true "Project ID"
// @Success 200 {array} models.UserSkill
// @Failure 401 {object} httpx.Error
// @security ApiKeyAuth
// @Router /v1/projects/{id}/skills [get]
func (h *handler) ListProjectSkills(w http.ResponseWriter, r *http.Request) error {
	p := ctxval.PrincipalFromContext(r.Context())
	if p == nil {
		return httpx.Envelope(httpx.CodeUnauthorized, "unauthenticated", "")
	}

	id := httpx.ExtractPathParam(r, "id")
	if id == "" {
		return httpx.Envelope(httpx.CodeInvalidRequest, "project id is required", "")
	}

	skills, err := h.ucProjSkills.ListSkillsForProject(r.Context(), models.ListProjectSkillsParams{
		UserID:    p.ID,
		ProjectID: id,
	})
	if err != nil {
		return err
	}

	return httpx.RespondJSON(w, r, http.StatusOK, skills)
}
