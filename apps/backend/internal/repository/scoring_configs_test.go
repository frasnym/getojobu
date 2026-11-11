package repository

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

func testScoringConfigDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("SCORING_CONFIG_TEST_DATABASE_URL")
	if url == "" {
		url = os.Getenv("JOBS_TEST_DATABASE_URL")
	}
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("no test database available; set DATABASE_URL")
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return pool
}

func TestScoringConfigs_GetActiveByFamily_NotFound(t *testing.T) {
	db := testScoringConfigDB(t)
	repo := NewScoringConfigsRepository(database.NewPG(db))

	_, err := repo.GetActiveByFamily(context.Background(), "non_existent_family_xyz")
	assert.ErrorIs(t, err, constants.ErrScoringConfigNotFound)
}

func TestScoringConfigs_GetByID_NotFound(t *testing.T) {
	db := testScoringConfigDB(t)
	repo := NewScoringConfigsRepository(database.NewPG(db))

	_, err := repo.GetByID(context.Background(), uuid.New())
	assert.ErrorIs(t, err, constants.ErrScoringConfigNotFound)
}

func TestScoringConfigs_Create_and_List(t *testing.T) {
	db := testScoringConfigDB(t)
	repo := NewScoringConfigsRepository(database.NewPG(db))

	testFamily := "test_family_" + uuid.New().String()[:8]
	cfg := &models.ScoringConfig{
		Family:  testFamily,
		Version: 1,
		Categories: []models.CategoryConfig{
			{Name: "skills", Weight: 0.60, Description: "skills"},
			{Name: "experience", Weight: 0.40, Description: "experience"},
		},
		IsActive: true,
	}

	created, err := repo.Create(context.Background(), cfg)
	require.NoError(t, err)
	assert.Equal(t, testFamily, created.Family)
	assert.True(t, created.IsActive)
	assert.Equal(t, 1, created.Version)

	active, err := repo.GetActiveByFamily(context.Background(), testFamily)
	require.NoError(t, err)
	assert.Equal(t, created.ID, active.ID)

	list, err := repo.List(context.Background(), models.ListScoringConfigsParams{Family: &testFamily})
	require.NoError(t, err)
	assert.Len(t, list, 1)
}
