package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

func TestUserJobs_Create_and_IsOwned(t *testing.T) {
	db := testJobDB(t)
	repo := NewUserJobsRepository(database.NewPG(db))
	jobsRepo := NewJobsRepository(database.NewPG(db))

	ctx := context.Background()
	userID := uuid.NewString()

	// Ensure a test user exists
	_, err := db.Exec(ctx, "INSERT INTO users (id, clerk_user_id, email, full_name, role) VALUES ($1, $2, $3, $4, 'user')",
		userID, "clerk_"+userID, userID+"@test.com", "Test User")
	require.NoError(t, err)

	jobResult, err := jobsRepo.Create(ctx, models.CreateJobParams{
		Slug:          "user-jobs-test-" + userID,
		Source:        "linkedin",
		Title:         "Engineer",
		Company:       "Acme",
		Location:      "Jakarta",
		UpdatedByType: "user",
		UpdatedByID:   userID,
	})
	require.NoError(t, err)
	jobID := jobResult.Job.ID

	// First link -> created = true
	created, err := repo.Create(ctx, userID, jobID)
	require.NoError(t, err)
	assert.True(t, created)

	// Second link -> created = false (conflict ignored)
	created, err = repo.Create(ctx, userID, jobID)
	require.NoError(t, err)
	assert.False(t, created)

	// IsOwned check
	owned, err := repo.IsOwned(ctx, userID, jobID)
	require.NoError(t, err)
	assert.True(t, owned)

	owned, err = repo.IsOwned(ctx, uuid.NewString(), jobID)
	require.NoError(t, err)
	assert.False(t, owned)
}

func TestUserJobs_BulkCreate(t *testing.T) {
	db := testJobDB(t)
	repo := NewUserJobsRepository(database.NewPG(db))
	jobsRepo := NewJobsRepository(database.NewPG(db))

	ctx := context.Background()
	user1 := uuid.NewString()
	user2 := uuid.NewString()

	for _, u := range []string{user1, user2} {
		_, err := db.Exec(ctx, "INSERT INTO users (id, clerk_user_id, email, full_name, role) VALUES ($1, $2, $3, $4, 'user')",
			u, "clerk_"+u, u+"@test.com", "Test User")
		require.NoError(t, err)
	}

	job1, err := jobsRepo.Create(ctx, models.CreateJobParams{
		Slug:          "bulk-job-1-" + user1,
		Source:        "linkedin",
		Title:         "Engineer 1",
		Company:       "Acme",
		Location:      "Jakarta",
		UpdatedByType: "user",
		UpdatedByID:   user1,
	})
	require.NoError(t, err)

	job2, err := jobsRepo.Create(ctx, models.CreateJobParams{
		Slug:          "bulk-job-2-" + user2,
		Source:        "linkedin",
		Title:         "Engineer 2",
		Company:       "Acme",
		Location:      "Jakarta",
		UpdatedByType: "user",
		UpdatedByID:   user2,
	})
	require.NoError(t, err)

	// Pre-link user1 to job1
	_, err = repo.Create(ctx, user1, job1.Job.ID)
	require.NoError(t, err)

	links := []models.UserJobLink{
		{UserID: user1, JobID: job1.Job.ID}, // already linked -> false
		{UserID: user2, JobID: job1.Job.ID}, // newly linked -> true
		{UserID: user1, JobID: job2.Job.ID}, // newly linked -> true
	}

	addedMap, err := repo.BulkCreate(ctx, links)
	require.NoError(t, err)

	key1_1 := fmt.Sprintf("%s:%d", user1, job1.Job.ID)
	key2_1 := fmt.Sprintf("%s:%d", user2, job1.Job.ID)
	key1_2 := fmt.Sprintf("%s:%d", user1, job2.Job.ID)

	assert.False(t, addedMap[key1_1], "user1 already linked to job1")
	assert.True(t, addedMap[key2_1], "user2 newly linked to job1")
	assert.True(t, addedMap[key1_2], "user1 newly linked to job2")

}
