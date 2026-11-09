package projectskills

import (
	"context"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository"
)

// ProjectSkill resolves project-skill operations against the repository.
type ProjectSkill interface {
	LinkSkills(ctx context.Context, params models.LinkProjectSkillsParams) error
	ListSkillsForProject(ctx context.Context, params models.ListProjectSkillsParams) ([]*models.UserSkill, error)
}

// projectSkillService implements ProjectSkill backed by a repository.
type projectSkillService struct {
	repo repository.ProjectSkills
}

// NewProjectSkillService creates a project-skill service backed by a repository.
func NewProjectSkillService(repo repository.ProjectSkills) ProjectSkill {
	return &projectSkillService{repo: repo}
}

// LinkSkills associates the given skills with a project owned by userID.
func (s *projectSkillService) LinkSkills(ctx context.Context, params models.LinkProjectSkillsParams) error {
	return s.repo.LinkSkills(ctx, params)
}

// ListSkillsForProject returns the skills linked to a project owned by userID.
func (s *projectSkillService) ListSkillsForProject(ctx context.Context, params models.ListProjectSkillsParams) ([]*models.UserSkill, error) {
	return s.repo.ListSkillsForProject(ctx, params)
}
