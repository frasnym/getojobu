package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newServiceWithMock(repo *mocks.MockProjects) *projectService {
	return &projectService{repo: repo}
}

func TestCreateProject_valid(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjects(ctrl)
	s := newServiceWithMock(repo)

	params := models.CreateProjectParams{
		UserID:           "user_1",
		Name:             "Website Redesign",
		Description:      "Redesign the company website",
		Responsibilities: "UI design and frontend",
		Achievements:     "Improved load time by 40%",
		UserExperienceID: utils.PtrString("exp_1"),
		StartedAt:        utils.PtrString("2024-01-01"),
		EndedAt:          utils.PtrString("2024-06-30"),
		IsCurrent:        false,
	}

	repo.EXPECT().
		Create(gomock.Any(), params).
		Return(&models.Project{ID: "proj_new"}, nil)

	// Act.
	got, err := s.Create(context.Background(), params)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, "proj_new", got.ID)
}

func TestCreateProject_ended_before_started(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjects(ctrl)
	s := newServiceWithMock(repo)

	params := models.CreateProjectParams{
		UserID:    "user_1",
		Name:      "Project X",
		StartedAt: utils.PtrString("2024-12-31"),
		EndedAt:   utils.PtrString("2024-01-01"),
		IsCurrent: false,
	}

	// Act.
	_, err := s.Create(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ended_at must be >= started_at")
}

func TestCreateProject_current_with_ended_at(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjects(ctrl)
	s := newServiceWithMock(repo)

	params := models.CreateProjectParams{
		UserID:    "user_1",
		Name:      "Project X",
		EndedAt:   utils.PtrString("2024-12-31"),
		IsCurrent: true,
	}

	// Act.
	_, err := s.Create(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a current project must not have an ended_at")
}

func TestGetByID_not_found(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjects(ctrl)
	s := newServiceWithMock(repo)

	repo.EXPECT().
		GetByID(gomock.Any(), models.GetProjectParams{UserID: "user_1", ID: "proj_1"}).
		Return(nil, constants.ErrNotFound)

	// Act.
	_, err := s.GetByID(context.Background(), models.GetProjectParams{UserID: "user_1", ID: "proj_1"})

	// Assert.
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrNotFound), "expected ErrNotFound, got %v", err)
}

func TestGetByID_other_error(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjects(ctrl)
	s := newServiceWithMock(repo)

	repo.EXPECT().
		GetByID(gomock.Any(), models.GetProjectParams{UserID: "user_1", ID: "proj_1"}).
		Return(nil, errors.New("db connection lost"))

	// Act.
	_, err := s.GetByID(context.Background(), models.GetProjectParams{UserID: "user_1", ID: "proj_1"})

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get project")
	assert.Contains(t, err.Error(), "db connection lost")
}

func TestGetByID_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjects(ctrl)
	s := newServiceWithMock(repo)

	expected := &models.Project{ID: "proj_1", UserID: "user_1"}
	repo.EXPECT().
		GetByID(gomock.Any(), models.GetProjectParams{UserID: "user_1", ID: "proj_1"}).
		Return(expected, nil)

	// Act.
	got, err := s.GetByID(context.Background(), models.GetProjectParams{UserID: "user_1", ID: "proj_1"})

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestListByUser_passes_through(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjects(ctrl)
	s := newServiceWithMock(repo)

	expected := []*models.Project{{ID: "proj_1", UserID: "user_1"}}
	repo.EXPECT().
		ListByUser(gomock.Any(), "user_1").
		Return(expected, nil)

	// Act.
	got, err := s.ListByUser(context.Background(), "user_1")

	// Assert.
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "proj_1", got[0].ID)
}

func TestUpdateProject_valid(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjects(ctrl)
	s := newServiceWithMock(repo)

	params := models.UpdateProjectParams{
		UserID:           "user_1",
		ID:               "proj_1",
		Name:             "Updated Project",
		Description:      "Updated description",
		Responsibilities: "New responsibilities",
		Achievements:     "New achievements",
		UserExperienceID: utils.PtrString("exp_2"),
	}

	repo.EXPECT().
		Update(gomock.Any(), params).
		Return(&models.Project{ID: "proj_1"}, nil)

	// Act.
	got, err := s.Update(context.Background(), params)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, "proj_1", got.ID)
}

func TestUpdateProject_ended_before_started(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjects(ctrl)
	s := newServiceWithMock(repo)

	params := models.UpdateProjectParams{
		UserID:    "user_1",
		ID:        "proj_1",
		StartedAt: utils.PtrString("2024-12-31"),
		EndedAt:   utils.PtrString("2024-01-01"),
		IsCurrent: false,
	}

	// Act.
	_, err := s.Update(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ended_at must be >= started_at")
}

func TestUpdateProject_current_with_ended_at(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjects(ctrl)
	s := newServiceWithMock(repo)

	params := models.UpdateProjectParams{
		UserID:    "user_1",
		ID:        "proj_1",
		EndedAt:   utils.PtrString("2024-12-31"),
		IsCurrent: true,
	}

	// Act.
	_, err := s.Update(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a current project must not have an ended_at")
}

func TestUpdateProject_not_found(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockProjects(ctrl)
	s := newServiceWithMock(repo)

	repo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil, constants.ErrNotFound)

	// Act. Use a valid project so validation passes and we reach the repo.
	_, err := s.Update(context.Background(), models.UpdateProjectParams{
		UserID: "user_1",
		ID:     "proj_1",
		Name:   "Project X",
	})

	// Assert.
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrNotFound), "expected ErrNotFound, got %v", err)
}
