package repository

import (
	"errors"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/common"
	repo "github.com/frasnym/getojobu/apps/backend/internal/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// Contract is the repository layer contract. Repository implementations
// attach their data-access methods here as the domain model grows.
type Contract struct {
	DB                            database.DB
	Transactor                    database.Transactor
	UsersRepository               repo.Users
	ExperiencesRepository         repo.Experiences
	ProjectsRepository            repo.Projects
	ProjectSkillsRepository       repo.ProjectSkills
	JobsRepository                repo.Jobs
	UserJobsRepository            repo.UserJobs
	WorkflowsRepository           repo.Workflows
	ScoringConfigsRepository      repo.ScoringConfigs
	UserPreferencesRepository     repo.UserPreferences
	MatchingEvaluationsRepository repo.MatchingEvaluations
	EducationsRepository          repo.Educations
	StorageRepository             repo.Storage
}

// NewRepository creates the repository layer contract from the common
// infrastructure contract. It instantiates concrete repository types backed
// by the shared database connection.
func NewRepository(comm *common.Contract) (*Contract, error) {
	if comm == nil || comm.DB == nil {
		return nil, errors.New("common database is required")
	}

	pg := database.NewPG(comm.DB)

	c := &Contract{
		DB:                            comm.DB,
		Transactor:                    pg,
		UsersRepository:               repo.NewUsersRepository(comm.DB),
		ExperiencesRepository:         repo.NewExperiencesRepository(comm.DB),
		ProjectsRepository:            repo.NewProjectsRepository(comm.DB),
		ProjectSkillsRepository:       repo.NewProjectSkillsRepository(comm.DB),
		JobsRepository:                repo.NewJobsRepository(pg),
		UserJobsRepository:            repo.NewUserJobsRepository(pg),
		WorkflowsRepository:           repo.NewWorkflowsRepository(comm.Temporal, comm.TemporalTaskQueue),
		ScoringConfigsRepository:      repo.NewScoringConfigsRepository(pg),
		UserPreferencesRepository:     repo.NewUserPreferencesRepository(pg),
		MatchingEvaluationsRepository: repo.NewMatchingEvaluationsRepository(pg),
		EducationsRepository:          repo.NewEducationsRepository(comm.DB),
		StorageRepository:             comm.Storage,
	}
	return c, nil
}
