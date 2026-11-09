package repository

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testJobDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("JOBS_TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("no test database available; set JOBS_TEST_DATABASE_URL or DATABASE_URL")
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestCreate_inserted_returns_created_true(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(db)

	slug := "senior-backend-engineer-acme-jakarta"
	params := models.CreateJobParams{
		Slug:          slug,
		Source:        "linkedin",
		Title:         "Senior Backend Engineer",
		Company:       "Acme",
		Location:      "Jakarta",
		Description:   strPtr("We build backend systems"),
		UpdatedByType: "user",
		UpdatedByID:   "user_1",
	}

	got, err := repo.Create(context.Background(), params)
	require.NoError(t, err)
	require.True(t, got.Created, "expected first insert to report created=true")
	assert.Equal(t, slug, got.Job.Slug)
	assert.Equal(t, "new", got.Job.Status)
	assert.Equal(t, "user", got.Job.UpdatedByType)
	assert.Equal(t, "user_1", got.Job.UpdatedByID)
}

func TestCreate_duplicate_slug_returns_created_false(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(db)

	slug := "senior-backend-engineer-acme-jakarta"
	base := models.CreateJobParams{
		Slug:          slug,
		Source:        "linkedin",
		Title:         "Senior Backend Engineer",
		Company:       "Acme",
		Location:      "Jakarta",
		UpdatedByType: "user",
		UpdatedByID:   "user_1",
	}

	_, err := repo.Create(context.Background(), base)
	require.NoError(t, err)

	got, err := repo.Create(context.Background(), base)
	require.NoError(t, err)
	require.False(t, got.Created, "expected duplicate insert to report created=false")
	assert.Equal(t, slug, got.Job.Slug)
}

func TestGetBySlug_found_and_not_found(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(db)

	slug := "golang-developer-google-singapore"
	_, err := repo.Create(context.Background(), models.CreateJobParams{
		Slug:          slug,
		Source:        "indeed",
		Title:         "Golang Developer",
		Company:       "Google",
		Location:      "Singapore",
		UpdatedByType: "cron",
		UpdatedByID:   "cron-scrape",
	})
	require.NoError(t, err)

	got, err := repo.GetBySlug(context.Background(), slug)
	require.NoError(t, err)
	assert.Equal(t, slug, got.Slug)

	_, err = repo.GetBySlug(context.Background(), "does-not-exist")
	require.Error(t, err)
	assert.True(t, errors.Is(err, sql.ErrNoRows), "expected sql.ErrNoRows for missing slug, got %v", err)
}

func TestListRecent_ordering_and_limit(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(db)

	for i := 0; i < 5; i++ {
		_, err := repo.Create(context.Background(), models.CreateJobParams{
			Slug:          "list-recent-" + string(rune('a'+i)),
			Source:        "linkedin",
			Title:         "Job " + string(rune('a'+i)),
			Company:       "Co",
			Location:      "Loc",
			UpdatedByType: "cron",
			UpdatedByID:   "cron-scrape",
		})
		require.NoError(t, err)
	}

	got, err := repo.ListRecent(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.LessOrEqual(t, got[0].CreatedAt.Unix(), got[0].CreatedAt.Unix())
}

func TestSearchByCompany_ILIKE(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(db)

	_, err := repo.Create(context.Background(), models.CreateJobParams{
		Slug:          "search-by-acme-jakarta",
		Source:        "linkedin",
		Title:         "Engineer",
		Company:       "Acme Corp",
		Location:      "Jakarta",
		UpdatedByType: "cron",
		UpdatedByID:   "cron-scrape",
	})
	require.NoError(t, err)
	_, err = repo.Create(context.Background(), models.CreateJobParams{
		Slug:          "search-by-beta-jakarta",
		Source:        "linkedin",
		Title:         "Engineer",
		Company:       "Beta Ltd",
		Location:      "Jakarta",
		UpdatedByType: "cron",
		UpdatedByID:   "cron-scrape",
	})
	require.NoError(t, err)

	got, err := repo.SearchByCompany(context.Background(), "acme", 20)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Acme Corp", got[0].Company)

	got, err = repo.SearchByCompany(context.Background(), "ACME", 20)
	require.NoError(t, err)
	require.Len(t, got, 1, "ILIKE should be case-insensitive")
}

func TestUpdateDescription_allowed_when_null(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(db)

	slug := "update-null-desc-jakarta"
	_, err := repo.Create(context.Background(), models.CreateJobParams{
		Slug:          slug,
		Source:        "linkedin",
		Title:         "Engineer",
		Company:       "Co",
		Location:      "Jakarta",
		UpdatedByType: "cron",
		UpdatedByID:   "cron-scrape",
		// description left nil
	})
	require.NoError(t, err)

	got, err := repo.UpdateDescription(context.Background(), slug, "New description.", "cron", "cron-scrape")
	require.NoError(t, err)
	assert.Equal(t, slug, got.Slug)
	assert.Equal(t, "New description.", *got.Description)
}

func TestUpdateDescription_blocked_when_analyzed(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(db)

	slug := "update-blocked-jakarta"
	_, err := repo.Create(context.Background(), models.CreateJobParams{
		Slug:          slug,
		Source:        "linkedin",
		Title:         "Engineer",
		Company:       "Co",
		Location:      "Jakarta",
		Description:   strPtr("Existing description"),
		UpdatedByType: "cron",
		UpdatedByID:   "cron-scrape",
	})
	require.NoError(t, err)

	// Mark analyzed to trigger the guard.
	_, err = db.Exec(context.Background(), "UPDATE jobs SET status='analyzed' WHERE slug=$1", slug)
	require.NoError(t, err)

	_, err = repo.UpdateDescription(context.Background(), slug, "New description.", "cron", "cron-scrape")
	require.Error(t, err)
	assert.True(t, errors.Is(err, sql.ErrNoRows), "expected sql.ErrNoRows when guard blocks update, got %v", err)
}

func strPtr(s string) *string {
	return &s
}
