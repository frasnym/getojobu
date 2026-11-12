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
)

func TestStartJobScrape_valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWF := mocks.NewMockWorkflows(ctrl)
	mockPrefs := mocks.NewMockUserPreferences(ctrl)
	s := &jobService{workflows: mockWF, userPreferences: mockPrefs}

	mockPrefs.EXPECT().
		GetActivePreferences(gomock.Any(), []string{"userA"}).
		Return([]models.UserPreference{
			{
				UserID:    "userA",
				Keywords:  []string{"golang"},
				Locations: []string{"indonesia"},
				Boards:    []string{"glassdoor"},
				IsActive:  true,
			},
		}, nil)

	expectedInput := models.JobScrapeWorkflowInput{
		Board:      "glassdoor",
		Keyword:    "golang",
		Location:   "indonesia",
		UserIDs:    []string{"userA"},
		SearchID:   "glassdoor-golang-indonesia",
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
	s := &jobService{workflows: mockWF, userPreferences: mockPrefs}

	// When user-ids is not provided (nil or empty),
	// GetActivePreferences is invoked with nil/empty, which selects all active user preferences.
	mockPrefs.EXPECT().
		GetActivePreferences(gomock.Any(), gomock.Nil()).
		Return([]models.UserPreference{
			{
				UserID:    "userA",
				Keywords:  []string{"golang"},
				Locations: []string{"indonesia"},
				Boards:    []string{"glassdoor"},
				IsActive:  true,
			},
			{
				UserID:    "userB",
				Keywords:  []string{"golang"},
				Locations: []string{"indonesia"},
				Boards:    []string{"glassdoor"},
				IsActive:  true,
			},
		}, nil)

	// UserA and UserB are normalized and grouped into a single target for glassdoor-golang-indonesia
	expectedInput := models.JobScrapeWorkflowInput{
		Board:      "glassdoor",
		Keyword:    "golang",
		Location:   "indonesia",
		UserIDs:    []string{"userA", "userB"},
		SearchID:   "glassdoor-golang-indonesia",
		WorkflowID: "job-scrape-glassdoor-golang-indonesia",
	}

	mockWF.EXPECT().
		StartLimiterWorkItem(gomock.Any(), "job-scrape-glassdoor-golang-indonesia", models.LimiterJobTypeScrape, expectedInput).
		Return("job-scrape-glassdoor-golang-indonesia", nil)

	// Call without providing any user_ids (empty params)
	wfIDs, err := s.StartJobScrape(context.Background(), models.ScrapeJobsParams{})
	require.NoError(t, err)
	assert.Equal(t, []string{"job-scrape-glassdoor-golang-indonesia"}, wfIDs)
}

func TestStartJobScrape_empty_preferences_returns_empty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPrefs := mocks.NewMockUserPreferences(ctrl)
	s := &jobService{userPreferences: mockPrefs}

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
	s := &jobService{userPreferences: mockPrefs}

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
	s := &jobService{workflows: mockWF, userPreferences: mockPrefs}

	mockPrefs.EXPECT().
		GetActivePreferences(gomock.Any(), []string{"userA"}).
		Return([]models.UserPreference{
			{
				UserID:    "userA",
				Keywords:  []string{"golang"},
				Locations: []string{"ireland"},
				Boards:    []string{"glassdoor"},
				IsActive:  true,
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

func TestNormalizePreferences(t *testing.T) {
	t.Run("normalizes user preferences exactly per specification", func(t *testing.T) {
		// userA,[golang,php],[indonesia],[glassdoor]
		// userB,[golang],[malaysia,indonesia],[linkedin,glassdoor]
		prefs := []models.UserPreference{
			{
				UserID:    "userA",
				Keywords:  []string{"golang", "php"},
				Locations: []string{"indonesia"},
				Boards:    []string{"glassdoor"},
				IsActive:  true,
			},
			{
				UserID:    "userB",
				Keywords:  []string{"golang"},
				Locations: []string{"malaysia", "indonesia"},
				Boards:    []string{"linkedin", "glassdoor"},
				IsActive:  true,
			},
		}

		targets := normalizePreferences(prefs)

		expected := []models.NormalizedScrapeTarget{
			{
				Board:    "glassdoor",
				Keyword:  "golang",
				Location: "indonesia",
				UserIDs:  []string{"userA", "userB"},
			},
			{
				Board:    "glassdoor",
				Keyword:  "golang",
				Location: "malaysia",
				UserIDs:  []string{"userB"},
			},
			{
				Board:    "glassdoor",
				Keyword:  "php",
				Location: "indonesia",
				UserIDs:  []string{"userA"},
			},
			{
				Board:    "linkedin",
				Keyword:  "golang",
				Location: "indonesia",
				UserIDs:  []string{"userB"},
			},
			{
				Board:    "linkedin",
				Keyword:  "golang",
				Location: "malaysia",
				UserIDs:  []string{"userB"},
			},
		}

		assert.Equal(t, expected, targets)
	})

	t.Run("skips inactive preferences or empty keywords/locations", func(t *testing.T) {
		prefs := []models.UserPreference{
			{
				UserID:    "userA",
				Keywords:  []string{"golang"},
				Locations: []string{"indonesia"},
				Boards:    []string{"glassdoor"},
				IsActive:  false, // inactive
			},
			{
				UserID:    "userB",
				Keywords:  []string{""},
				Locations: []string{"indonesia"},
				Boards:    []string{"glassdoor"},
				IsActive:  true,
			},
		}

		targets := normalizePreferences(prefs)
		assert.Empty(t, targets)
	})

	t.Run("skips preferences when boards empty", func(t *testing.T) {
		prefs := []models.UserPreference{
			{
				UserID:    "userA",
				Keywords:  []string{"GoLang "},
				Locations: []string{" INDONESIA "},
				Boards:    nil,
				IsActive:  true,
			},
			{
				UserID:    "userB",
				Keywords:  []string{"Python"},
				Locations: []string{"Singapore"},
				Boards:    []string{"", "  "},
				IsActive:  true,
			},
		}

		targets := normalizePreferences(prefs)
		assert.Empty(t, targets)
	})

	t.Run("normalizes whitespace and case in board, keyword, and location", func(t *testing.T) {
		prefs := []models.UserPreference{
			{
				UserID:    "userA",
				Keywords:  []string{" GoLang "},
				Locations: []string{" INDONESIA "},
				Boards:    []string{" GlassDoor "},
				IsActive:  true,
			},
		}

		targets := normalizePreferences(prefs)
		expected := []models.NormalizedScrapeTarget{
			{
				Board:    "glassdoor",
				Keyword:  "golang",
				Location: "indonesia",
				UserIDs:  []string{"userA"},
			},
		}
		assert.Equal(t, expected, targets)
	})
}
