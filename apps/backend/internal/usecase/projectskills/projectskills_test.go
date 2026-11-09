package projectskills

import (
	"context"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newServiceWithMock(repo *mocks.MockProjectSkills) *projectSkillService {
	return &projectSkillService{repo: repo}
}

func TestLinkSkills_passes_through(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjectSkills(ctrl)
	s := newServiceWithMock(repo)

	params := models.LinkProjectSkillsParams{
		UserID:    "user_1",
		ProjectID: "proj_1",
		SkillIDs:  []string{"skill_a", "skill_b"},
	}

	repo.EXPECT().
		LinkSkills(gomock.Any(), params).
		Return(nil)

	// Act.
	err := s.LinkSkills(context.Background(), params)

	// Assert.
	require.NoError(t, err)
}

func TestLinkSkills_passes_error_through(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjectSkills(ctrl)
	s := newServiceWithMock(repo)

	params := models.LinkProjectSkillsParams{
		UserID:    "user_1",
		ProjectID: "proj_1",
		SkillIDs:  []string{"skill_a"},
	}

	repo.EXPECT().
		LinkSkills(gomock.Any(), params).
		Return(assert.AnError)

	// Act.
	err := s.LinkSkills(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}

func TestListSkillsForProject_passes_through(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjectSkills(ctrl)
	s := newServiceWithMock(repo)

	params := models.ListProjectSkillsParams{UserID: "user_1", ProjectID: "proj_1"}
	expected := []*models.UserSkill{{ID: "skill_1", UserID: "user_1", SkillName: "Go"}}

	repo.EXPECT().
		ListSkillsForProject(gomock.Any(), params).
		Return(expected, nil)

	// Act.
	got, err := s.ListSkillsForProject(context.Background(), params)

	// Assert.
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "skill_1", got[0].ID)
	assert.Equal(t, "Go", got[0].SkillName)
}

func TestListSkillsForProject_passes_error_through(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjectSkills(ctrl)
	s := newServiceWithMock(repo)

	params := models.ListProjectSkillsParams{UserID: "user_1", ProjectID: "proj_1"}

	repo.EXPECT().
		ListSkillsForProject(gomock.Any(), params).
		Return(nil, assert.AnError)

	// Act.
	_, err := s.ListSkillsForProject(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}
