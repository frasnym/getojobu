package repository

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
	"github.com/google/uuid"
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
	repo := NewJobsRepository(database.NewPG(db))

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
	repo := NewJobsRepository(database.NewPG(db))

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

func TestBulkCreate_duplicate_slug_within_batch(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(database.NewPG(db))

	slug := "bulk-dup-slug-jakarta"
	params := models.BulkCreateJobParams{
		Jobs: []models.CreateJobParams{
			{
				Slug:          slug,
				Source:        "linkedin",
				Title:         "Job 1",
				Company:       "Acme",
				Location:      "Jakarta",
				UpdatedByType: "user",
				UpdatedByID:   "user_1",
			},
			{
				Slug:          slug,
				Source:        "linkedin",
				Title:         "Job 1 Duplicate",
				Company:       "Acme",
				Location:      "Jakarta",
				UpdatedByType: "user",
				UpdatedByID:   "user_2",
			},
		},
	}

	results, err := repo.BulkCreate(context.Background(), params)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.True(t, results[0].Created, "first occurrence should report created=true")
	assert.False(t, results[1].Created, "second occurrence of same slug within batch should report created=false")
	assert.Equal(t, slug, results[0].Job.Slug)
	assert.Equal(t, slug, results[1].Job.Slug)
	assert.Equal(t, results[0].Job.ID, results[1].Job.ID)
}

func TestGetBySlug_found_and_not_found(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(database.NewPG(db))

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

	got, err := repo.GetBySlug(context.Background(), slug, "")
	require.NoError(t, err)
	assert.Equal(t, slug, got.Job.Slug)

	_, err = repo.GetBySlug(context.Background(), "does-not-exist", "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrNotFound), "expected ErrNotFound for missing slug, got %v", err)
}

func TestList_ordering_and_limit(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(database.NewPG(db))

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

	got, total, err := repo.List(context.Background(), 3, 0, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 3)
	require.Len(t, got, 3)
	assert.LessOrEqual(t, got[0].Job.CreatedAt.Unix(), got[0].Job.CreatedAt.Unix())
}

func TestList_isOwned_flag(t *testing.T) {
	db := testJobDB(t)
	jobsRepo := NewJobsRepository(database.NewPG(db))
	userJobsRepo := NewUserJobsRepository(database.NewPG(db))

	ctx := context.Background()
	userID := "user-list-owned"

	// Ensure a test user exists (user_jobs.user_id references users.id).
	_, err := db.Exec(ctx, "INSERT INTO users (id, clerk_user_id, email, full_name, role) VALUES ($1, $2, $3, $4, 'user')",
		userID, "clerk_"+userID, userID+"@test.com", "Test User")
	require.NoError(t, err)

	// Create three jobs.
	var ownedJobID int64
	for i := 0; i < 3; i++ {
		res, err := jobsRepo.Create(ctx, models.CreateJobParams{
			Slug:          "list-isowned-" + string(rune('a'+i)),
			Source:        "linkedin",
			Title:         "Job " + string(rune('a'+i)),
			Company:       "Co",
			Location:      "Loc",
			UpdatedByType: "cron",
			UpdatedByID:   "cron-scrape",
		})
		require.NoError(t, err)
		if i == 0 {
			ownedJobID = res.Job.ID
		}
	}

	// Link the first job to the user.
	_, err = userJobsRepo.Create(ctx, userID, ownedJobID)
	require.NoError(t, err)

	got, total, err := jobsRepo.List(ctx, 10, 0, userID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 3)
	require.Len(t, got, 3)

	byID := make(map[int64]*models.JobWithOwnership, len(got))
	for _, j := range got {
		byID[j.Job.ID] = j
	}
	assert.True(t, byID[ownedJobID].IsOwned, "linked job should be owned")
	for _, j := range got {
		if j.Job.ID != ownedJobID {
			assert.False(t, j.IsOwned, "unlinked job should not be owned")
		}
	}

	// Empty userID -> no job is owned.
	gotAll, _, err := jobsRepo.List(ctx, 10, 0, "")
	require.NoError(t, err)
	for _, j := range gotAll {
		assert.False(t, j.IsOwned, "empty userID should yield isOwned=false")
	}
}

func TestJobs_owner_visibility(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(database.NewPG(db))
	ctx := context.Background()

	userA := uuid.NewString()
	userB := uuid.NewString()
	for i, id := range []string{userA, userB} {
		_, err := db.Exec(ctx,
			"INSERT INTO users (id, clerk_user_id, email, full_name, role) VALUES ($1, $2, $3, $4, 'user')",
			id, "clerk_vis_"+id[:8], "vis"+string(rune('a'+i))+"@test.com", "Visibility Test")
		require.NoError(t, err)
	}

	sharedSlug := "visibility-shared-" + uuid.NewString()[:8]
	privateASlug := "m-" + userA[:8] + "-visibility-private-" + uuid.NewString()[:8]
	privateBSlug := "m-" + userB[:8] + "-visibility-private-" + uuid.NewString()[:8]
	t.Cleanup(func() {
		_, _ = db.Exec(ctx, "DELETE FROM jobs WHERE slug = ANY($1)", []string{sharedSlug, privateASlug, privateBSlug})
		_, _ = db.Exec(ctx, "DELETE FROM users WHERE id = ANY($1)", []string{userA, userB})
	})

	ownerA, err := uuid.Parse(userA)
	require.NoError(t, err)
	ownerB, err := uuid.Parse(userB)
	require.NoError(t, err)

	for _, tc := range []struct {
		slug  string
		owner *uuid.UUID
	}{
		{sharedSlug, nil},
		{privateASlug, &ownerA},
		{privateBSlug, &ownerB},
	} {
		_, err := repo.Create(ctx, models.CreateJobParams{
			Slug:          tc.slug,
			Source:        "manual",
			Title:         "Visibility Test",
			Company:       "Acme",
			Location:      "Berlin",
			OwnerID:       tc.owner,
			UpdatedByType: "user",
			UpdatedByID:   userA,
		})
		require.NoError(t, err)
	}

	got, err := repo.GetBySlug(ctx, privateASlug, userA)
	require.NoError(t, err)
	assert.Equal(t, privateASlug, got.Job.Slug)

	_, err = repo.GetBySlug(ctx, privateASlug, userB)
	assert.ErrorIs(t, err, constants.ErrNotFound)

	got, err = repo.GetBySlug(ctx, privateASlug, "")
	require.NoError(t, err)
	assert.Equal(t, privateASlug, got.Job.Slug)

	got, err = repo.GetBySlug(ctx, sharedSlug, userB)
	require.NoError(t, err)
	assert.Equal(t, sharedSlug, got.Job.Slug)

	for _, tc := range []struct {
		name       string
		userID     string
		wantSlugs  []string
		rejectSlug string
	}{
		{"user A", userA, []string{sharedSlug, privateASlug}, privateBSlug},
		{"user B", userB, []string{sharedSlug, privateBSlug}, privateASlug},
		{"machine caller", "", []string{sharedSlug, privateASlug, privateBSlug}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobs, total, err := repo.List(ctx, 1000, 0, tc.userID)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, total, len(tc.wantSlugs))
			slugs := make(map[string]bool, len(jobs))
			for _, j := range jobs {
				slugs[j.Job.Slug] = true
			}
			for _, want := range tc.wantSlugs {
				assert.True(t, slugs[want], "expected %q in list", want)
			}
			if tc.rejectSlug != "" {
				assert.False(t, slugs[tc.rejectSlug], "unexpected %q in list", tc.rejectSlug)
			}
		})
	}
}

func TestUpdateStatus_success(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(database.NewPG(db))

	slug := "update-status-test"
	_, err := repo.Create(context.Background(), models.CreateJobParams{
		Slug:          slug,
		Source:        "linkedin",
		Title:         "Engineer",
		Company:       "Co",
		Location:      "Jakarta",
		UpdatedByType: "cron",
		UpdatedByID:   "cron-scrape",
	})
	require.NoError(t, err)

	resultJSON := `{"skills":["Go"]}`
	got, err := repo.UpdateStatus(context.Background(), models.UpdateJobParams{Slug: slug, Status: "analyzed", AnalyzeResult: &resultJSON, FailureMessage: nil, UpdatedByType: "worker", UpdatedByID: "getojobu-worker"})
	require.NoError(t, err)
	assert.Equal(t, slug, got.Slug)
	assert.Equal(t, "analyzed", got.Status)
	require.NotEmpty(t, got.AnalyzeResult)
	assert.Equal(t, resultJSON, string(got.AnalyzeResult))
}

func TestUpdateStatus_scrape_fields(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(database.NewPG(db))
	ctx := context.Background()

	slug := "m-abcdef12-scrape-" + uuid.NewString()[:8]
	newSlug := "m-abcdef12-senior-engineer-acme-berlin-" + uuid.NewString()[:8]
	title := "Senior Engineer"
	_, err := repo.Create(ctx, models.CreateJobParams{
		Slug: slug, Source: "user_scrape", Status: "scrape_pending",
		Title: "Pending scrape", Company: "unknown", Location: "unknown",
		UpdatedByType: "user", UpdatedByID: "abcdef12-3456-7890-abcd-ef1234567890",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.Exec(ctx, "DELETE FROM jobs WHERE slug = ANY($1)", []string{slug, newSlug}) })

	got, err := repo.UpdateStatus(ctx, models.UpdateJobParams{
		Slug: slug, Status: "new", NewSlug: &newSlug, Title: &title,
		UpdatedByType: "worker", UpdatedByID: "scrape",
	})
	require.NoError(t, err)
	assert.Equal(t, newSlug, got.Slug)
	assert.Equal(t, title, got.Title)
}

func TestUpdateStatus_not_found(t *testing.T) {
	db := testJobDB(t)
	repo := NewJobsRepository(database.NewPG(db))

	_, err := repo.UpdateStatus(context.Background(), models.UpdateJobParams{Slug: "non-existent-slug", Status: "analyze_pending", AnalyzeResult: nil, FailureMessage: nil, UpdatedByType: "worker", UpdatedByID: "getojobu-worker"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, constants.ErrNotFound))
}

func strPtr(s string) *string {
	return &s
}
