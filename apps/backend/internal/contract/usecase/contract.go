package usecase

import (
	"errors"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	ucExperiences "github.com/frasnym/getojobu/apps/backend/internal/usecase/experiences"
	ucJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs"
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
	Jobs          ucJobs.Job
}

// NewUsecase creates the usecase layer contract from the repository contract.
// It instantiates concrete usecase types backed by the shared repository contract.
func NewUsecase(r *repository.Contract) (*Contract, error) {
	if r == nil {
		return nil, errors.New("repository contract is required")
	}

	c := &Contract{
		Users:         ucUsers.NewUserService(r.UsersRepository),
		Experiences:   ucExperiences.NewExperienceService(r.ExperiencesRepository),
		Projects:      ucProjects.NewProjectService(r.ProjectsRepository),
		ProjectSkills: ucProjectSkills.NewProjectSkillService(r.ProjectSkillsRepository),
		Jobs:          ucJobs.NewJobService(r),
	}
	return c, nil
}
