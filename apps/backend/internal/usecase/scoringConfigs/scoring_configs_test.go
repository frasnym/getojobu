package scoringConfigs_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	repoMocks "github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	ucScoringConfigs "github.com/frasnym/getojobu/apps/backend/internal/usecase/scoringConfigs"
)

func TestScoringConfigService_Create_EmptyFamily(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockScoringConfigs(ctrl)
	svc := ucScoringConfigs.NewScoringConfigService(mockRepo)

	params := models.CreateScoringConfigParams{
		Family:  "   ",
		Version: 1,
		Categories: []models.CategoryConfig{
			{Name: "skills", Weight: 1.0, Description: "skills"},
		},
	}

	_, err := svc.Create(context.Background(), params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "family cannot be empty")
}

func TestScoringConfigService_Create_InvalidWeights(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockScoringConfigs(ctrl)
	svc := ucScoringConfigs.NewScoringConfigService(mockRepo)

	params := models.CreateScoringConfigParams{
		Family:  "technology",
		Version: 1,
		Categories: []models.CategoryConfig{
			{Name: "skills", Weight: 0.5, Description: "skills"},
		},
	}

	_, err := svc.Create(context.Background(), params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "weights must sum to 1.0")
}

func TestScoringConfigService_Create_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockScoringConfigs(ctrl)
	svc := ucScoringConfigs.NewScoringConfigService(mockRepo)

	params := models.CreateScoringConfigParams{
		Family:  "technology",
		Version: 1,
		Categories: []models.CategoryConfig{
			{Name: "skills", Weight: 0.7, Description: "skills"},
			{Name: "experience", Weight: 0.3, Description: "experience"},
		},
	}

	expected := &models.ScoringConfig{
		ID:         uuid.New(),
		Family:     "technology",
		Version:    1,
		Categories: params.Categories,
		IsActive:   true,
	}

	mockRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(expected, nil)

	created, err := svc.Create(context.Background(), params)
	assert.NoError(t, err)
	assert.Equal(t, expected.ID, created.ID)
}

func TestScoringConfigService_GetActiveByFamily(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockScoringConfigs(ctrl)
	svc := ucScoringConfigs.NewScoringConfigService(mockRepo)

	// Empty family
	_, err := svc.GetActiveByFamily(context.Background(), "  ")
	assert.Error(t, err)

	// Valid family
	expected := &models.ScoringConfig{ID: uuid.New(), Family: "technology", IsActive: true}
	mockRepo.EXPECT().GetActiveByFamily(gomock.Any(), "technology").Return(expected, nil)

	got, err := svc.GetActiveByFamily(context.Background(), "technology")
	assert.NoError(t, err)
	assert.Equal(t, expected.ID, got.ID)
}

func TestScoringConfigService_Update_InvalidCategories(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockScoringConfigs(ctrl)
	svc := ucScoringConfigs.NewScoringConfigService(mockRepo)

	params := models.UpdateScoringConfigParams{
		Categories: []models.CategoryConfig{
			{Name: "skills", Weight: 0.5, Description: "skills"},
		},
	}

	_, err := svc.Update(context.Background(), uuid.New(), params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "weights must sum to 1.0")
}

func TestScoringConfigService_Update_ActiveConfigCategoriesRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repoMocks.NewMockScoringConfigs(ctrl)
	svc := ucScoringConfigs.NewScoringConfigService(mockRepo)

	id := uuid.New()
	existingActive := &models.ScoringConfig{
		ID:       id,
		Family:   "technology",
		Version:  1,
		IsActive: true,
	}

	mockRepo.EXPECT().GetByID(gomock.Any(), id).Return(existingActive, nil)

	params := models.UpdateScoringConfigParams{
		Categories: []models.CategoryConfig{
			{Name: "skills", Weight: 1.0, Description: "skills"},
		},
	}

	_, err := svc.Update(context.Background(), id, params)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot update categories of an active configuration")
}
