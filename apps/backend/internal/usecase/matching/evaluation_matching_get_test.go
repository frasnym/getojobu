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

func TestGetEvaluation_IncludesCoverLetterStatus(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)

	evalRepo := repoMocks.NewMockMatchingEvaluations(ctrl)
	usersRepo := repoMocks.NewMockUsers(ctrl)
	userJobsRepo := repoMocks.NewMockUserJobs(ctrl)
	scoringConfigsRepo := repoMocks.NewMockScoringConfigs(ctrl)
	jobsRepo := repoMocks.NewMockJobs(ctrl)

	service := matching.NewService(&repository.Contract{
		MatchingEvaluationsRepository: evalRepo,
		UsersRepository:               usersRepo,
		UserJobsRepository:            userJobsRepo,
		ScoringConfigsRepository:      scoringConfigsRepo,
		JobsRepository:                jobsRepo,
	})

	evalID := uuid.New()
	configID := uuid.New()
	userJobID := uuid.New()
	userID := uuid.New()
	jobID := int64(123)
	coverLetterStatus := "queued_retry"

	evalRepo.EXPECT().GetByID(ctx, evalID).Return(&models.MatchingEvaluation{
		ID:                evalID,
		UserJobID:         userJobID,
		ScoringConfigID:   configID,
		EvaluationStatus:  "success",
		EvaluationData:    json.RawMessage(`{"summary":"Strong match"}`),
		CoverLetterStatus: &coverLetterStatus,
	}, nil)
	scoringConfigsRepo.EXPECT().GetByID(ctx, configID).Return(&models.ScoringConfig{}, nil)
	userJobsRepo.EXPECT().GetByID(ctx, userJobID).Return(&models.UserJob{
		ID: userJobID, UserID: userID, JobID: jobID,
	}, nil)
	usersRepo.EXPECT().FindByID(ctx, userID.String()).Return(&models.User{
		ID:          userID.String(),
		ProfileData: json.RawMessage(`{}`),
		FactsData:   json.RawMessage(`{}`),
	}, nil)
	jobsRepo.EXPECT().GetByID(ctx, jobID).Return(&models.Job{
		ID:            jobID,
		Slug:          "job-1",
		Title:         "Engineer",
		Company:       "Acme",
		Location:      "Remote",
		AnalyzeResult: json.RawMessage(`{}`),
	}, nil)

	resp, err := service.GetEvaluation(ctx, evalID)
	require.NoError(t, err)
	require.NotNil(t, resp)

	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, "queued_retry", body["cover_letter_status"])
}
