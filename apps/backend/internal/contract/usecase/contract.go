package usecase

import (
	"errors"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	ucExperiences "github.com/frasnym/getojobu/apps/backend/internal/usecase/experiences"
	ucProjects "github.com/frasnym/getojobu/apps/backend/internal/usecase/projects"
	ucProjectSkills "github.com/frasnym/getojobu/apps/backend/internal/usecase/projectskills"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
)

// Contract is the usecase layer contract. Usecase implementations attach
// their business-logic methods here as the domain model grows.
type Contract struct {
	Users         ucUsers.User
	Experiences   ucExperiences.Experience
	Projects      ucProjects.Project
	ProjectSkills ucProjectSkills.ProjectSkill
}

// NewUsecase creates the usecase layer contract from the repository contract.
// It instantiates concrete usecase types backed by the shared repository contract.
func NewUsecase(r *repository.Contract) (*Contract, error) {
	if r == nil || r.UsersRepository == nil {
		return nil, errors.New("repository contract with UsersRepository is required")
	}

	c := &Contract{
		Users:         ucUsers.NewUserService(r.UsersRepository),
		Experiences:   ucExperiences.NewExperienceService(r.ExperiencesRepository),
		Projects:      ucProjects.NewProjectService(r.ProjectsRepository),
		ProjectSkills: ucProjectSkills.NewProjectSkillService(r.ProjectSkillsRepository),
	}
	return c, nil
}
