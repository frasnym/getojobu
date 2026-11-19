package matching_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repoMocks "github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
)

func TestRefreshProfileCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	usersRepo := repoMocks.NewMockUsers(ctrl)
	expRepo := repoMocks.NewMockExperiences(ctrl)
	projRepo := repoMocks.NewMockProjects(ctrl)
	projSkillsRepo := repoMocks.NewMockProjectSkills(ctrl)
	eduRepo := repoMocks.NewMockEducations(ctrl)

	userID := uuid.New()
	name := "Alex"
	usersRepo.EXPECT().FindByID(gomock.Any(), userID.String()).Return(&models.User{
		ID:       userID.String(),
		FullName: &name,
	}, nil)
	expRepo.EXPECT().ListByUser(gomock.Any(), userID.String()).Return([]*models.Experience{}, nil).Times(2)
	eduRepo.EXPECT().ListByUser(gomock.Any(), userID.String()).Return([]*models.UserEducation{}, nil).Times(2)
	projRepo.EXPECT().ListByUser(gomock.Any(), userID.String()).Return([]*models.Project{}, nil)
	projSkillsRepo.EXPECT().ListSkillsByUser(gomock.Any(), userID.String()).Return([]*models.UserSkill{}, nil)

	var profileJSON, factsJSON []byte
	usersRepo.EXPECT().UpdateProfileCache(gomock.Any(), userID.String(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, profile, facts []byte) error {
			profileJSON, factsJSON = profile, facts
			return nil
		})

	service := matching.NewService(&repository.Contract{
		UsersRepository:         usersRepo,
		ExperiencesRepository:   expRepo,
		ProjectsRepository:      projRepo,
		ProjectSkillsRepository: projSkillsRepo,
		EducationsRepository:    eduRepo,
	})

	results, err := service.RefreshProfileCache(context.Background(), []uuid.UUID{userID})
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Nil(t, results[0].Error)

	var facts map[string]interface{}
	require.NoError(t, json.Unmarshal(factsJSON, &facts))
	assert.Equal(t, "Alex", facts["name"])

	var profile map[string]interface{}
	require.NoError(t, json.Unmarshal(profileJSON, &profile))
	assert.Contains(t, profile, "educations")
}
