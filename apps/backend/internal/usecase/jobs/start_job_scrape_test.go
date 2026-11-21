package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/usecase/jobs/scrape"
)

func testRegistry() *scrape.JobScrapeRegistry {
	return scrape.NewJobScrapeRegistry()
}

func TestStartJobScrape_valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWF := mocks.NewMockWorkflows(ctrl)
	mockPrefs := mocks.NewMockUserPreferences(ctrl)
	s := &jobService{workflows: mockWF, userPreferences: mockPrefs, registry: testRegistry()}

	mockPrefs.EXPECT().
		GetActivePreferences(gomock.Any(), []string{"userA"}).
		Return([]models.UserPreference{
			{
				UserID:    "userA",
				Keywords:  []string{"golang"},
				Locations: []string{"indonesia"},
				Boards:    []string{"glassdoor"},
			},
		}, nil)

	expectedInput := models.JobScrapeWorkflowInput{
		Board:      "glassdoor",
		Keyword:    "golang",
		Location:   "indonesia",
		UserIDs:    []string{"userA"},
		SearchID:   "golang-indonesia",
		WorkflowID: "job-scrape-glassdoor-golang-indonesia",
	}

	mockWF.EXPECT().
		StartLimiterWorkItem(gomock.Any(), "job-scrape-glassdoor-golang-indonesia", models.LimiterJobTypeScrape, expectedInput).
		Return("job-scrape-glassdoor-golang-indonesia", nil)

	wfIDs, err := s.StartJobScrape(context.Background(), models.ScrapeJobsParams{
		UserIDs: []string{"userA"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"job-scrape-glassdoor-golang-indonesia"}, wfIDs)
}

func TestStartJobScrape_all_users_when_user_ids_not_provided(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWF := mocks.NewMockWorkflows(ctrl)
	mockPrefs := mocks.NewMockUserPreferences(ctrl)
	s := &jobService{workflows: mockWF, userPreferences: mockPrefs, registry: testRegistry()}

	mockPrefs.EXPECT().
		GetActivePreferences(gomock.Any(), gomock.Nil()).
		Return([]models.UserPreference{
			{
				UserID:    "userA",
				Keywords:  []string{"golang"},
				Locations: []string{"indonesia"},
				Boards:    []string{"glassdoor"},
			},
			{
				UserID:    "userB",
				Keywords:  []string{"golang"},
				Locations: []string{"indonesia"},
				Boards:    []string{"glassdoor"},
			},
		}, nil)

	expectedInput := models.JobScrapeWorkflowInput{
		Board:      "glassdoor",
		Keyword:    "golang",
		Location:   "indonesia",
		UserIDs:    []string{"userA", "userB"},
		SearchID:   "golang-indonesia",
		WorkflowID: "job-scrape-glassdoor-golang-indonesia",
	}

	mockWF.EXPECT().
		StartLimiterWorkItem(gomock.Any(), "job-scrape-glassdoor-golang-indonesia", models.LimiterJobTypeScrape, expectedInput).
		Return("job-scrape-glassdoor-golang-indonesia", nil)

	wfIDs, err := s.StartJobScrape(context.Background(), models.ScrapeJobsParams{})
	require.NoError(t, err)
	assert.Equal(t, []string{"job-scrape-glassdoor-golang-indonesia"}, wfIDs)
}

func TestStartJobScrape_empty_preferences_returns_empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPrefs := mocks.NewMockUserPreferences(ctrl)
	s := &jobService{userPreferences: mockPrefs, registry: testRegistry()}

	mockPrefs.EXPECT().
		GetActivePreferences(gomock.Any(), gomock.Any()).
		Return([]models.UserPreference{}, nil)

	wfIDs, err := s.StartJobScrape(context.Background(), models.ScrapeJobsParams{})
	require.NoError(t, err)
	assert.Empty(t, wfIDs)
}

func TestStartJobScrape_preferences_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPrefs := mocks.NewMockUserPreferences(ctrl)
	s := &jobService{userPreferences: mockPrefs, registry: testRegistry()}

	mockPrefs.EXPECT().
		GetActivePreferences(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("db error"))

	_, err := s.StartJobScrape(context.Background(), models.ScrapeJobsParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get active preferences")
}

func TestStartJobScrape_workflow_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWF := mocks.NewMockWorkflows(ctrl)
	mockPrefs := mocks.NewMockUserPreferences(ctrl)
	s := &jobService{workflows: mockWF, userPreferences: mockPrefs, registry: testRegistry()}

	mockPrefs.EXPECT().
		GetActivePreferences(gomock.Any(), []string{"userA"}).
		Return([]models.UserPreference{
			{
				UserID:    "userA",
				Keywords:  []string{"golang"},
				Locations: []string{"ireland"},
				Boards:    []string{"glassdoor"},
			},
		}, nil)

	mockWF.EXPECT().
		StartLimiterWorkItem(gomock.Any(), gomock.Any(), models.LimiterJobTypeScrape, gomock.Any()).
		Return("", errors.New("temporal unavailable"))

	_, err := s.StartJobScrape(context.Background(), models.ScrapeJobsParams{
		UserIDs: []string{"userA"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start scrape workflow")
}
