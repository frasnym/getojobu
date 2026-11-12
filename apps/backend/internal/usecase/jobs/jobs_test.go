package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// dummyTransactor executes the operation closure directly without a real database.
type dummyTransactor struct{}

func (dummyTransactor) Transact(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func newServiceWithMock(repo *mocks.MockJobs, userJobs *mocks.MockUserJobs) *jobService {
	return &jobService{tx: dummyTransactor{}, repo: repo, userJobs: userJobs}
}

func TestCreate_passes_through(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	params := models.CreateJobParams{
		Slug:        "senior-backend-engineer-acme-jakarta",
		Source:      "linkedin",
		Title:       "Senior Backend Engineer",
		Company:     "Acme",
		Location:    "Jakarta",
		Description: strPtr("We build backend systems"),
	}
	expected := &models.CreateJobResult{Job: &models.Job{Slug: params.Slug, ID: 1}, Created: true}

	repo.EXPECT().
		Create(gomock.Any(), params).
		Return(expected, nil)
	userJobs.EXPECT().
		Create(gomock.Any(), params.UserID, int64(1)).
		Return(true, nil)

	got, err := s.Create(context.Background(), params)

	require.NoError(t, err)
	assert.True(t, got.Created)
	assert.True(t, got.AddedToUserList)
	assert.Equal(t, params.Slug, got.Job.Slug)
}

func TestCreate_conflict_returns_existing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	params := models.CreateJobParams{
		Slug:     "senior-backend-engineer-acme-jakarta",
		Source:   "linkedin",
		Title:    "Senior Backend Engineer",
		Company:  "Acme",
		Location: "Jakarta",
		UserID:   "user_1",
	}
	existing := &models.Job{Slug: params.Slug, ID: 7}
	expected := &models.CreateJobResult{Job: existing, Created: false}

	repo.EXPECT().
		Create(gomock.Any(), params).
		Return(expected, nil)
	userJobs.EXPECT().
		Create(gomock.Any(), params.UserID, int64(7)).
		Return(false, nil)

	got, err := s.Create(context.Background(), params)

	require.NoError(t, err)
	assert.False(t, got.Created)
	assert.False(t, got.AddedToUserList)
	assert.Equal(t, int64(7), got.Job.ID)
}

func TestCreateBulk_passes_through(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	jobs := []models.CreateJobParams{
		{Slug: "job-1", Source: "linkedin", Title: "Job 1", Company: "Acme", Location: "Jakarta", UserID: "user_1"},
		{Slug: "job-2", Source: "linkedin", Title: "Job 2", Company: "Acme", Location: "Jakarta", UserID: "user_2"},
	}
	params := models.BulkCreateJobParams{Jobs: jobs}

	repo.EXPECT().
		BulkCreate(gomock.Any(), params).
		Return([]*models.CreateJobResult{
			{Job: &models.Job{Slug: "job-1", ID: 1}, Created: true},
			{Job: &models.Job{Slug: "job-2", ID: 2}, Created: false},
		}, nil)
	userJobs.EXPECT().
		BulkCreate(gomock.Any(), []models.UserJobLink{
			{UserID: "user_1", JobID: 1},
			{UserID: "user_2", JobID: 2},
		}).
		Return(map[string]bool{
			"user_1:1": true,
			"user_2:2": false,
		}, nil)

	got, err := s.CreateBulk(context.Background(), params)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "job-1", got[0].Slug)
	assert.True(t, got[0].Created)
	assert.True(t, got[0].AddedToUserList)
	assert.Equal(t, "job-2", got[1].Slug)
	assert.False(t, got[1].Created)
	assert.False(t, got[1].AddedToUserList)
}

func TestGetBySlug_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	expected := &models.Job{Slug: "golang-developer-google-singapore"}
	repo.EXPECT().
		GetBySlug(gomock.Any(), expected.Slug).
		Return(expected, nil)

	got, err := s.GetBySlug(context.Background(), expected.Slug)

	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestGetBySlug_not_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	repo.EXPECT().
		GetBySlug(gomock.Any(), "does-not-exist").
		Return(nil, constants.ErrNotFound)

	_, err := s.GetBySlug(context.Background(), "does-not-exist")

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrNotFound), "expected ErrNotFound, got %v", err)
}

func TestGetBySlug_other_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	repo.EXPECT().
		GetBySlug(gomock.Any(), "slug").
		Return(nil, errors.New("db connection lost"))

	_, err := s.GetBySlug(context.Background(), "slug")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get job by slug")
	assert.Contains(t, err.Error(), "db connection lost")
}

func TestIsOwned_true(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	userJobs.EXPECT().
		IsOwned(gomock.Any(), "user_1", int64(42)).
		Return(true, nil)

	owned, err := s.IsOwned(context.Background(), "user_1", 42)

	require.NoError(t, err)
	assert.True(t, owned)
}

func TestIsOwned_false(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	userJobs.EXPECT().
		IsOwned(gomock.Any(), "user_1", int64(42)).
		Return(false, nil)

	owned, err := s.IsOwned(context.Background(), "user_1", 42)

	require.NoError(t, err)
	assert.False(t, owned)
}

func TestList_passes_through(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	expected := []*models.Job{{Slug: "job-1"}, {Slug: "job-2"}}

	t.Run("owned_by_user", func(t *testing.T) {
		repo.EXPECT().
			List(gomock.Any(), 20, "user_1").
			Return(expected, nil)

		got, err := s.List(context.Background(), 20, "user_1")

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "job-1", got[0].Slug)
	})

	t.Run("all_jobs", func(t *testing.T) {
		repo.EXPECT().
			List(gomock.Any(), 5, "").
			Return(expected, nil)

		got, err := s.List(context.Background(), 5, "")

		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "job-1", got[0].Slug)
	})
}

func TestSearchByCompany_passes_through(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	expected := []*models.Job{{Slug: "job-1", Company: "Acme Corp"}}
	repo.EXPECT().
		SearchByCompany(gomock.Any(), "acme", 20).
		Return(expected, nil)

	got, err := s.SearchByCompany(context.Background(), "acme", 20)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Acme Corp", got[0].Company)
}

func TestUpdateStatus_allowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	slug := "test-job-status"
	existing := &models.Job{Slug: slug, Status: "new"}
	updated := &models.Job{Slug: slug, Status: "analyze_pending"}

	repo.EXPECT().
		GetBySlug(gomock.Any(), slug).
		Return(existing, nil)
	repo.EXPECT().
		UpdateStatus(gomock.Any(), slug, "analyze_pending", (*string)(nil), (*string)(nil), "worker", "worker_1").
		Return(updated, nil)

	got, err := s.UpdateStatus(context.Background(), slug, "analyze_pending", nil, nil, "worker", "worker_1")

	require.NoError(t, err)
	assert.Equal(t, "analyze_pending", got.Status)
}

func TestUpdateStatus_invalid_transition(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	userJobs := mocks.NewMockUserJobs(ctrl)
	s := newServiceWithMock(repo, userJobs)

	slug := "test-job-invalid"
	existing := &models.Job{Slug: slug, Status: "analyzed"}

	repo.EXPECT().
		GetBySlug(gomock.Any(), slug).
		Return(existing, nil)

	_, err := s.UpdateStatus(context.Background(), slug, "analyze_pending", nil, nil, "worker", "worker_1")

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrInvalidTransition))
}

func TestStartJobScrape_valid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWF := mocks.NewMockWorkflows(ctrl)
	s := &jobService{workflows: mockWF}

	params := models.ScrapeJobsParams{
		UserID:     "admin",
		SearchID:   "search-123",
		WorkflowID: "job-scrape-scan-123",
		Searches: []models.ScrapeSearch{
			{Keywords: []string{"golang"}, Locations: []string{"ireland"}},
		},
	}

	expectedInput := models.JobScrapeWorkflowInput{
		UserID:     "admin",
		SearchID:   "search-123",
		WorkflowID: "job-scrape-scan-123",
		Searches:   params.Searches,
	}

	mockWF.EXPECT().
		StartJobScrape(gomock.Any(), "job-scrape-scan-123", expectedInput).
		Return("job-scrape-scan-123", nil)

	wfID, err := s.StartJobScrape(context.Background(), params)
	require.NoError(t, err)
	assert.Equal(t, "job-scrape-scan-123", wfID)
}

func TestStartJobScrape_validation_errors(t *testing.T) {
	s := &jobService{}

	_, err := s.StartJobScrape(context.Background(), models.ScrapeJobsParams{Searches: nil})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must provide at least one search")

	_, err = s.StartJobScrape(context.Background(), models.ScrapeJobsParams{
		Searches: []models.ScrapeSearch{
			{Keywords: nil, Locations: []string{"ireland"}},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one keyword required")

	_, err = s.StartJobScrape(context.Background(), models.ScrapeJobsParams{
		Searches: []models.ScrapeSearch{
			{Keywords: []string{"golang"}, Locations: nil},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one location required")
}

func TestStartJobScrape_workflow_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockWF := mocks.NewMockWorkflows(ctrl)
	s := &jobService{workflows: mockWF}

	mockWF.EXPECT().
		StartJobScrape(gomock.Any(), gomock.Any(), gomock.Any()).
		Return("", errors.New("temporal unavailable"))

	_, err := s.StartJobScrape(context.Background(), models.ScrapeJobsParams{
		Searches: []models.ScrapeSearch{
			{Keywords: []string{"golang"}, Locations: []string{"ireland"}},
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start job scrape workflow")
}

func strPtr(s string) *string {
	return &s
}
