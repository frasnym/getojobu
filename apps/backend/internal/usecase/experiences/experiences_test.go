package experiences

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

func newServiceWithMock(repo *mocks.MockExperiences) *experienceService {
	return &experienceService{repo: repo}
}

func TestCreateExperience_valid(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	params := models.CreateExperienceParams{
		UserID:         "user_1",
		ExperienceType: models.ExperienceTypeProfessional,
		Organization:   "Acme Corp",
		RoleTitle:      "Engineer",
		StartedAt:      utils.PtrString("2024-01-01"),
		EndedAt:        utils.PtrString("2024-12-31"),
		IsCurrent:      false,
	}

	repo.EXPECT().
		Create(gomock.Any(), params).
		Return(&models.Experience{ID: "exp_new"}, nil)

	// Act.
	got, err := s.Create(context.Background(), params)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, "exp_new", got.ID)
}

func TestCreateExperience_invalid_type(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	params := models.CreateExperienceParams{
		UserID:         "user_1",
		ExperienceType: models.ExperienceType("bogus"),
	}

	// Act.
	_, err := s.Create(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid experience_type")
}

func TestCreateExperience_ended_before_started(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	params := models.CreateExperienceParams{
		UserID:         "user_1",
		ExperienceType: models.ExperienceTypeProfessional,
		StartedAt:      utils.PtrString("2024-12-31"),
		EndedAt:        utils.PtrString("2024-01-01"),
	}

	// Act.
	_, err := s.Create(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ended_at must be >= started_at")
}

func TestCreateExperience_current_with_ended_at(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	params := models.CreateExperienceParams{
		UserID:         "user_1",
		ExperienceType: models.ExperienceTypeProfessional,
		EndedAt:        utils.PtrString("2024-12-31"),
		IsCurrent:      true,
	}

	// Act.
	_, err := s.Create(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a current experience must not have an ended_at")
}

func TestGetByID_not_found(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	repo.EXPECT().
		GetByID(gomock.Any(), models.GetExperienceParams{UserID: "user_1", ID: "exp_1"}).
		Return(nil, constants.ErrNotFound)

	// Act.
	_, err := s.GetByID(context.Background(), models.GetExperienceParams{UserID: "user_1", ID: "exp_1"})

	// Assert.
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrNotFound), "expected ErrNotFound, got %v", err)
}

func TestGetByID_other_error(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	repo.EXPECT().
		GetByID(gomock.Any(), models.GetExperienceParams{UserID: "user_1", ID: "exp_1"}).
		Return(nil, errors.New("db connection lost"))

	// Act.
	_, err := s.GetByID(context.Background(), models.GetExperienceParams{UserID: "user_1", ID: "exp_1"})

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get experience")
	assert.Contains(t, err.Error(), "db connection lost")
}

func TestGetByID_success(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	expected := &models.Experience{ID: "exp_1", UserID: "user_1"}
	repo.EXPECT().
		GetByID(gomock.Any(), models.GetExperienceParams{UserID: "user_1", ID: "exp_1"}).
		Return(expected, nil)

	// Act.
	got, err := s.GetByID(context.Background(), models.GetExperienceParams{UserID: "user_1", ID: "exp_1"})

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestListByUser_passes_through(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	expected := []*models.Experience{{ID: "exp_1", UserID: "user_1"}}
	repo.EXPECT().
		ListByUser(gomock.Any(), "user_1").
		Return(expected, nil)

	// Act.
	got, err := s.ListByUser(context.Background(), "user_1")

	// Assert.
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "exp_1", got[0].ID)
}

func TestUpdateExperience_valid(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	params := models.UpdateExperienceParams{
		UserID:         "user_1",
		ID:             "exp_1",
		ExperienceType: models.ExperienceTypeFreelance,
		Organization:   "LLC",
		RoleTitle:      "Consultant",
	}

	repo.EXPECT().
		Update(gomock.Any(), params).
		Return(&models.Experience{ID: "exp_1"}, nil)

	// Act.
	got, err := s.Update(context.Background(), params)

	// Assert.
	require.NoError(t, err)
	assert.Equal(t, "exp_1", got.ID)
}

func TestUpdateExperience_invalid_type(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	params := models.UpdateExperienceParams{
		UserID:         "user_1",
		ID:             "exp_1",
		ExperienceType: models.ExperienceType("nope"),
	}

	// Act.
	_, err := s.Update(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid experience_type")
}

func TestUpdateExperience_ended_before_started(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	params := models.UpdateExperienceParams{
		UserID:         "user_1",
		ID:             "exp_1",
		ExperienceType: models.ExperienceTypeProfessional,
		StartedAt:      utils.PtrString("2024-12-31"),
		EndedAt:        utils.PtrString("2024-01-01"),
	}

	// Act.
	_, err := s.Update(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ended_at must be >= started_at")
}

func TestUpdateExperience_current_with_ended_at(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	params := models.UpdateExperienceParams{
		UserID:         "user_1",
		ID:             "exp_1",
		ExperienceType: models.ExperienceTypeProfessional,
		EndedAt:        utils.PtrString("2024-12-31"),
		IsCurrent:      true,
	}

	// Act.
	_, err := s.Update(context.Background(), params)

	// Assert.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a current experience must not have an ended_at")
}

func TestUpdateExperience_not_found(t *testing.T) {
	// Arrange.
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockExperiences(ctrl)
	s := newServiceWithMock(repo)

	repo.EXPECT().
		Update(gomock.Any(), gomock.Any()).
		Return(nil, constants.ErrNotFound)

	// Act. Use a valid ExperienceType so validation passes and we reach the repo.
	_, err := s.Update(context.Background(), models.UpdateExperienceParams{
		UserID:         "user_1",
		ID:             "exp_1",
		ExperienceType: models.ExperienceTypeProfessional,
	})

	// Assert.
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrNotFound), "expected ErrNotFound, got %v", err)
}
