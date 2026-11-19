package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCreateManual(t *testing.T) {
	userID := "abcdef12-3456-7890-abcd-ef1234567890"
	params := models.CreateManualJobParams{
		Title:    "Senior Engineer",
		Company:  "Acme",
		Location: "Berlin",
	}

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockJobs(ctrl)
		userJobs := mocks.NewMockUserJobs(ctrl)
		s := newServiceWithMock(repo, userJobs)

		repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, got models.CreateJobParams) (*models.CreateJobResult, error) {
				assert.Equal(t, "manual", got.Source)
				require.NotNil(t, got.OwnerID)
				assert.Equal(t, userID, got.OwnerID.String())
				assert.Equal(t, "m-abcdef12-senior-engineer-acme-berlin", got.Slug)
				assert.Equal(t, []string{userID}, got.UserIDs)
				assert.Equal(t, "user", got.UpdatedByType)
				assert.Equal(t, userID, got.UpdatedByID)
				return &models.CreateJobResult{Job: &models.Job{ID: 1, Slug: got.Slug}, Created: true}, nil
			},
		)
		userJobs.EXPECT().Create(gomock.Any(), userID, int64(1)).Return(true, nil)

		got, err := s.CreateManual(context.Background(), userID, params)
		require.NoError(t, err)
		assert.True(t, got.Created)
		assert.True(t, got.AddedToUserList)
	})

	t.Run("duplicate slug", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockJobs(ctrl)
		userJobs := mocks.NewMockUserJobs(ctrl)
		s := newServiceWithMock(repo, userJobs)

		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(
			&models.CreateJobResult{Job: &models.Job{ID: 1}, Created: false}, nil,
		)
		userJobs.EXPECT().Create(gomock.Any(), userID, int64(1)).Return(false, nil)

		got, err := s.CreateManual(context.Background(), userID, params)
		require.NoError(t, err)
		assert.False(t, got.Created)
	})

	t.Run("missing title", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		s := newServiceWithMock(mocks.NewMockJobs(ctrl), mocks.NewMockUserJobs(ctrl))

		_, err := s.CreateManual(context.Background(), userID, models.CreateManualJobParams{
			Company: "Acme", Location: "Berlin",
		})
		require.Error(t, err)
	})
}

func TestScrapeURL(t *testing.T) {
	userID := "abcdef12-3456-7890-abcd-ef1234567890"
	sourceURL := "https://example.com/jobs/1"

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockJobs(ctrl)
		userJobs := mocks.NewMockUserJobs(ctrl)
		workflows := mocks.NewMockWorkflows(ctrl)
		s := newServiceWithMock(repo, userJobs)
		s.workflows = workflows

		repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, got models.CreateJobParams) (*models.CreateJobResult, error) {
				assert.Equal(t, "user_scrape", got.Source)
				assert.Equal(t, "scrape_new", got.Status)
				require.NotNil(t, got.OwnerID)
				assert.Equal(t, userID, got.OwnerID.String())
				assert.Equal(t, sourceURL, *got.SourceURL)
				assert.Equal(t, []string{userID}, got.UserIDs)
				assert.True(t, strings.HasPrefix(got.Slug, "m-abcdef12-scrape-"))
				return &models.CreateJobResult{Job: &models.Job{ID: 42, Slug: got.Slug}, Created: true}, nil
			},
		)
		userJobs.EXPECT().Create(gomock.Any(), userID, int64(42)).Return(true, nil)
		workflows.EXPECT().StartUserJobScrape(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, payload models.UserJobScrapeWorkflowInput) (string, error) {
				assert.Equal(t, int64(42), payload.JobID)
				assert.True(t, strings.HasPrefix(payload.Slug, "m-abcdef12-scrape-"))
				assert.Equal(t, sourceURL, payload.SourceURL)
				assert.Equal(t, userID, payload.UserID)
				assert.Equal(t, userID, payload.OwnerID)
				return "user-job-scrape-" + payload.Slug, nil
			},
		)

		got, err := s.ScrapeURL(context.Background(), userID, models.ScrapeURLParams{SourceURL: sourceURL})
		require.NoError(t, err)
		assert.True(t, got.Created)
	})

	t.Run("empty URL", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		s := newServiceWithMock(mocks.NewMockJobs(ctrl), mocks.NewMockUserJobs(ctrl))

		_, err := s.ScrapeURL(context.Background(), userID, models.ScrapeURLParams{})
		require.Error(t, err)
	})

	t.Run("workflow start failure marks scrape failed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		repo := mocks.NewMockJobs(ctrl)
		userJobs := mocks.NewMockUserJobs(ctrl)
		workflows := mocks.NewMockWorkflows(ctrl)
		s := newServiceWithMock(repo, userJobs)
		s.workflows = workflows

		repo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(
			&models.CreateJobResult{Job: &models.Job{ID: 7, Slug: "m-abcdef12-scrape-1"}, Created: true}, nil,
		)
		userJobs.EXPECT().Create(gomock.Any(), userID, int64(7)).Return(true, nil)
		workflows.EXPECT().StartUserJobScrape(gomock.Any(), gomock.Any()).
			Return("", errors.New("temporal unavailable"))
		repo.EXPECT().GetBySlug(gomock.Any(), "m-abcdef12-scrape-1", "").Return(
			&models.JobWithOwnership{Job: &models.Job{ID: 7, Slug: "m-abcdef12-scrape-1", Status: "scrape_new"}}, nil,
		)
		repo.EXPECT().UpdateStatus(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, p models.UpdateJobParams) (*models.Job, error) {
				assert.Equal(t, "scrape_failed", p.Status)
				require.NotNil(t, p.FailureMessage)
				assert.Contains(t, *p.FailureMessage, "temporal unavailable")
				return &models.Job{ID: 7, Slug: "m-abcdef12-scrape-1", Status: "scrape_failed"}, nil
			},
		)

		got, err := s.ScrapeURL(context.Background(), userID, models.ScrapeURLParams{SourceURL: sourceURL})
		require.NoError(t, err)
		assert.True(t, got.Created)
	})
}
