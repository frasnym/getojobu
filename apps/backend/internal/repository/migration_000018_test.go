package repository

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mig000018MigrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "cannot locate the test file")
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func mig000018Read(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(mig000018MigrationsDir(t), name))
	require.NoError(t, err, "migration file %s must exist", name)
	return string(raw)
}

func mig000018Up(t *testing.T, name string) string {
	t.Helper()
	up := strings.SplitN(mig000018Read(t, name), "-- +goose Down", 2)[0]
	up = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(up), "-- +goose Up"))
	require.NotEmpty(t, up, "migration %s has no Up section", name)
	return up
}

func mig000018Down(t *testing.T, name string) string {
	t.Helper()
	raw := mig000018Read(t, name)
	i := strings.Index(raw, "-- +goose Down")
	require.GreaterOrEqual(t, i, 0, "migration %s has no Down section", name)
	down := strings.TrimSpace(raw[i+len("-- +goose Down"):])
	require.NotEmpty(t, down, "migration %s has an empty Down section", name)
	return down
}

func mig000018Exec(t *testing.T, ctx context.Context, conn *pgx.Conn, sql, label string) {
	t.Helper()
	for _, statement := range strings.Split(sql, ";") {
		var kept []string
		for _, line := range strings.Split(statement, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				kept = append(kept, line)
			}
		}
		statement = strings.TrimSpace(strings.Join(kept, "\n"))
		if statement == "" {
			continue
		}
		_, err := conn.Exec(ctx, statement)
		require.NoError(t, err, "%s", label)
	}
}

func TestMigration000018CollapsesAndConstrains(t *testing.T) {
	upSQL := mig000018Up(t, "000018_single_evaluation.sql")
	downSQL := mig000018Down(t, "000018_single_evaluation.sql")

	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		t.Skip("TEST_PG_DSN is not set; skipping migration 000018 integration test")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(ctx) })

	schema := "mig000018_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = conn.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	_, err = conn.Exec(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)

	entries, err := os.ReadDir(mig000018MigrationsDir(t))
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") || entry.Name() >= "000018_single_evaluation.sql" {
			continue
		}
		mig000018Exec(t, ctx, conn, mig000018Up(t, entry.Name()), "applying "+entry.Name())
	}

	userID, userJobID := uuid.New(), uuid.New()
	_, err = conn.Exec(ctx, `INSERT INTO users (id, clerk_user_id, email, role) VALUES ($1, $2, $3, $4)`,
		userID, "clerk_mig000018", "mig000018@example.com", "user")
	require.NoError(t, err)

	var jobID int64
	err = conn.QueryRow(ctx, `INSERT INTO jobs (slug, source, title, company, location, updated_by_type, updated_by_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		"mig-000018-job", "linkedin", "Engineer", "Acme", "Jakarta", "user", "seed").Scan(&jobID)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `INSERT INTO user_jobs (id, user_id, job_id) VALUES ($1, $2, $3)`, userJobID, userID, jobID)
	require.NoError(t, err)

	var scoringConfigID uuid.UUID
	err = conn.QueryRow(ctx, `SELECT id FROM scoring_configs ORDER BY family LIMIT 1`).Scan(&scoringConfigID)
	require.NoError(t, err)

	olderID, newestID := uuid.New(), uuid.New()
	_, err = conn.Exec(ctx, `INSERT INTO matching_evaluations (id, user_job_id, scoring_config_id, evaluation_status, created_at)
		VALUES ($1, $2, $3, 'success', $4)`, olderID, userJobID, scoringConfigID, time.Now().Add(-2*time.Hour))
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO matching_evaluations (id, user_job_id, scoring_config_id, evaluation_status, created_at)
		VALUES ($1, $2, $3, 'success', $4)`, newestID, userJobID, scoringConfigID, time.Now().Add(-time.Hour))
	require.NoError(t, err)

	// Confirm the pre-migration state: user_jobs carries the legacy column.
	var legacyColumns int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = 'user_jobs' AND column_name = 'latest_evaluation_id'`, schema).Scan(&legacyColumns)
	require.NoError(t, err)
	require.Equal(t, 1, legacyColumns)

	mig000018Exec(t, ctx, conn, upSQL, "applying 000018")

	var count int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM matching_evaluations WHERE user_job_id = $1`, userJobID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "only one evaluation per user_job must remain")

	var survivor uuid.UUID
	err = conn.QueryRow(ctx, `SELECT id FROM matching_evaluations WHERE user_job_id = $1`, userJobID).Scan(&survivor)
	require.NoError(t, err)
	assert.Equal(t, newestID, survivor, "the evaluation with the greatest created_at must survive")

	_, err = conn.Exec(ctx, `INSERT INTO matching_evaluations (id, user_job_id, scoring_config_id) VALUES ($1, $2, $3)`,
		uuid.New(), userJobID, scoringConfigID)
	require.Error(t, err, "a second evaluation for the same user_job_id must be rejected")

	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = 'user_jobs' AND column_name = 'latest_evaluation_id'`, schema).Scan(&legacyColumns)
	require.NoError(t, err)
	assert.Zero(t, legacyColumns, "latest_evaluation_id must be dropped")

	for _, index := range []string{"idx_user_jobs_latest_eval", "idx_matching_evaluations_user_job_id"} {
		var indexes int
		err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM pg_indexes WHERE schemaname = $1 AND indexname = $2`, schema, index).Scan(&indexes)
		require.NoError(t, err)
		assert.Zero(t, indexes, "index %s must be dropped", index)
	}

	var constraints int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM pg_constraint
		WHERE connamespace = $1::regnamespace AND conname = 'uq_matching_evaluations_user_job'`, schema).Scan(&constraints)
	require.NoError(t, err)
	assert.Equal(t, 1, constraints, "the unique constraint must exist")

	mig000018Exec(t, ctx, conn, downSQL, "reverting 000018")

	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = 'user_jobs' AND column_name = 'latest_evaluation_id'`, schema).Scan(&legacyColumns)
	require.NoError(t, err)
	assert.Equal(t, 1, legacyColumns, "Down must restore latest_evaluation_id")

	var populated int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM user_jobs
		WHERE id = $1 AND latest_evaluation_id IS NOT NULL`, userJobID).Scan(&populated)
	require.NoError(t, err)
	assert.Equal(t, 1, populated, "Down must repopulate latest_evaluation_id")

	var pointsAtSurvivor int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM user_jobs
		WHERE id = $1 AND latest_evaluation_id = $2`, userJobID, survivor).Scan(&pointsAtSurvivor)
	require.NoError(t, err)
	assert.Equal(t, 1, pointsAtSurvivor, "Down must point latest_evaluation_id at the surviving evaluation")

	for _, index := range []string{"idx_user_jobs_latest_eval", "idx_matching_evaluations_user_job_id"} {
		var indexes int
		err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM pg_indexes WHERE schemaname = $1 AND indexname = $2`, schema, index).Scan(&indexes)
		require.NoError(t, err)
		assert.Equal(t, 1, indexes, "Down must restore index %s", index)
	}
}
