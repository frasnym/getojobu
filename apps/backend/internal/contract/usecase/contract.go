package usecase

import (
	"errors"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	ucExperiences "github.com/frasnym/getojobu/apps/backend/internal/usecase/experiences"
	ucJobs "github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs"
	ucMatching "github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
	ucProjects "github.com/frasnym/getojobu/apps/backend/internal/usecase/projects"
	ucProjectSkills "github.com/frasnym/getojobu/apps/backend/internal/usecase/projectskills"
	ucScoringConfigs "github.com/frasnym/getojobu/apps/backend/internal/usecase/scoringConfigs"
	ucUsers "github.com/frasnym/getojobu/apps/backend/internal/usecase/users"
)

// Contract is the usecase layer contract. Usecase implementations attach
// their business-logic methods here as the domain model grows.
type Contract struct {
	Users          ucUsers.User
	Experiences    ucExperiences.Experience
	Projects       ucProjects.Project
	ProjectSkills  ucProjectSkills.ProjectSkill
	Jobs           ucJobs.Job
	ScoringConfigs ucScoringConfigs.ScoringConfig
	Matching       ucMatching.Matching
}

// NewUsecase creates the usecase layer contract from the repository contract.
func NewUsecase(r *repository.Contract) (*Contract, error) {
	if r == nil {
		return nil, errors.New("repository contract is required")
	}

	c := &Contract{
		Users:          ucUsers.NewUserService(r.UsersRepository),
		Experiences:    ucExperiences.NewExperienceService(r.ExperiencesRepository),
		Projects:       ucProjects.NewProjectService(r.ProjectsRepository),
		ProjectSkills:  ucProjectSkills.NewProjectSkillService(r.ProjectSkillsRepository),
		Jobs:           ucJobs.NewJobService(r),
		ScoringConfigs: ucScoringConfigs.NewScoringConfigService(r.ScoringConfigsRepository),
		Matching:       ucMatching.NewService(r),
	}
	return c, nil
}
