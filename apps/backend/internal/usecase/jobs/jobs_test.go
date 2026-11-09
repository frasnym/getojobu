package jobs

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/repository/mocks"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newServiceWithMock(repo *mocks.MockJobs) *jobService {
	return &jobService{repo: repo}
}

func TestCreate_passes_through(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	s := newServiceWithMock(repo)

	params := models.CreateJobParams{
		Slug:        "senior-backend-engineer-acme-jakarta",
		Source:      "linkedin",
		Title:       "Senior Backend Engineer",
		Company:     "Acme",
		Location:    "Jakarta",
		Description: strPtr("We build backend systems"),
	}
	expected := &models.CreateJobResult{Job: &models.Job{Slug: params.Slug}, Created: true}

	repo.EXPECT().
		Create(gomock.Any(), params).
		Return(expected, nil)

	got, err := s.Create(context.Background(), params)

	require.NoError(t, err)
	assert.True(t, got.Created)
	assert.Equal(t, params.Slug, got.Job.Slug)
}

func TestGetBySlug_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	s := newServiceWithMock(repo)

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
	s := newServiceWithMock(repo)

	repo.EXPECT().
		GetBySlug(gomock.Any(), "does-not-exist").
		Return(nil, sql.ErrNoRows)

	_, err := s.GetBySlug(context.Background(), "does-not-exist")

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrNotFound), "expected ErrNotFound, got %v", err)
}

func TestGetBySlug_other_error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	s := newServiceWithMock(repo)

	repo.EXPECT().
		GetBySlug(gomock.Any(), "slug").
		Return(nil, errors.New("db connection lost"))

	_, err := s.GetBySlug(context.Background(), "slug")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get job by slug")
	assert.Contains(t, err.Error(), "db connection lost")
}

func TestListRecent_passes_through(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	s := newServiceWithMock(repo)

	expected := []*models.Job{{Slug: "job-1"}, {Slug: "job-2"}}
	repo.EXPECT().
		ListRecent(gomock.Any(), 5).
		Return(expected, nil)

	got, err := s.ListRecent(context.Background(), 5)

	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "job-1", got[0].Slug)
}

func TestSearchByCompany_passes_through(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	s := newServiceWithMock(repo)

	expected := []*models.Job{{Slug: "job-1", Company: "Acme Corp"}}
	repo.EXPECT().
		SearchByCompany(gomock.Any(), "acme", 20).
		Return(expected, nil)

	got, err := s.SearchByCompany(context.Background(), "acme", 20)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Acme Corp", got[0].Company)
}

func TestUpdateDescription_allowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	s := newServiceWithMock(repo)

	slug := "update-null-desc-jakarta"
	existing := &models.Job{Slug: slug, Status: "new"}
	updated := &models.Job{Slug: slug, Status: "new", Description: strPtr("New description.")}

	repo.EXPECT().
		GetBySlug(gomock.Any(), slug).
		Return(existing, nil)
	repo.EXPECT().
		UpdateDescription(gomock.Any(), slug, "New description.", "cron", "cron-scrape").
		Return(updated, nil)

	got, err := s.UpdateDescription(context.Background(), slug, "New description.", "cron", "cron-scrape")

	require.NoError(t, err)
	assert.Equal(t, "New description.", *got.Description)
}

func TestUpdateDescription_not_found(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	s := newServiceWithMock(repo)

	repo.EXPECT().
		GetBySlug(gomock.Any(), "missing").
		Return(nil, sql.ErrNoRows)

	_, err := s.UpdateDescription(context.Background(), "missing", "New description.", "cron", "cron-scrape")

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrNotFound), "expected ErrNotFound, got %v", err)
}

func TestUpdateDescription_blocked_when_analyzed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mocks.NewMockJobs(ctrl)
	s := newServiceWithMock(repo)

	slug := "update-blocked-jakarta"
	existing := &models.Job{Slug: slug, Status: "analyzed", Description: strPtr("Existing")}

	repo.EXPECT().
		GetBySlug(gomock.Any(), slug).
		Return(existing, nil)
	repo.EXPECT().
		UpdateDescription(gomock.Any(), slug, "New description.", "cron", "cron-scrape").
		Return(nil, sql.ErrNoRows)

	_, err := s.UpdateDescription(context.Background(), slug, "New description.", "cron", "cron-scrape")

	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrDescriptionBlocked), "expected ErrDescriptionBlocked, got %v", err)
}

func strPtr(s string) *string {
	return &s
}
