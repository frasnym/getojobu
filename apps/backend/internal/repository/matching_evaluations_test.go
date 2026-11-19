package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

type mevalSeed struct {
	UserID          uuid.UUID
	JobID           int64
	UserJobID       uuid.UUID
	ScoringConfigID uuid.UUID
}

func mevalMigrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "cannot locate the test file")
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func mevalExecSQL(t *testing.T, ctx context.Context, conn *pgx.Conn, sql, label string) {
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

// mevalTestRepo builds an isolated schema with all migrations applied and
// returns a repository whose connections use that schema via search_path.
func mevalTestRepo(t *testing.T) (MatchingEvaluations, *pgxpool.Pool, mevalSeed) {
	t.Helper()
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		t.Skip("TEST_PG_DSN is not set; skipping matching evaluation repository integration test")
	}

	ctx := context.Background()
	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close(context.Background()) })

	schema := "meval_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	_, err = admin.Exec(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)

	entries, err := os.ReadDir(mevalMigrationsDir(t))
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(mevalMigrationsDir(t), entry.Name()))
		require.NoError(t, err)
		up := strings.SplitN(string(raw), "-- +goose Down", 2)[0]
		up = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(up), "-- +goose Up"))
		mevalExecSQL(t, ctx, admin, up, "applying "+entry.Name())
	}

	seed := mevalSeed{
		UserID:    uuid.New(),
		UserJobID: uuid.New(),
	}
	_, err = admin.Exec(ctx,
		`INSERT INTO users (id, clerk_user_id, email, role) VALUES ($1, $2, $3, 'user')`,
		seed.UserID, "clerk_meval_"+seed.UserID.String(), seed.UserID.String()+"@test.com")
	require.NoError(t, err)

	err = admin.QueryRow(ctx, `INSERT INTO jobs (slug, source, title, company, location, updated_by_type, updated_by_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		"meval-job-"+seed.UserID.String(), "linkedin", "Engineer", "Acme", "Jakarta", "user", "seed").Scan(&seed.JobID)
	require.NoError(t, err)

	_, err = admin.Exec(ctx, `INSERT INTO user_jobs (id, user_id, job_id) VALUES ($1, $2, $3)`,
		seed.UserJobID, seed.UserID, seed.JobID)
	require.NoError(t, err)

	err = admin.QueryRow(ctx, `SELECT id FROM scoring_configs ORDER BY family LIMIT 1`).Scan(&seed.ScoringConfigID)
	require.NoError(t, err)

	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return NewMatchingEvaluationsRepository(database.NewPG(pool)), pool, seed
}

func TestCreateIfAbsent(t *testing.T) {
	repo, pool, seed := mevalTestRepo(t)
	ctx := context.Background()

	got, created, err := repo.CreateIfAbsent(ctx, &models.MatchingEvaluation{
		UserJobID:            seed.UserJobID,
		EvaluationStatus:     string(constants.MatchingStatusNew),
		ScoringConfigID:      seed.ScoringConfigID,
		CandidateProfileData: json.RawMessage(`{"candidate":"v1"}`),
		JobProfileData:       json.RawMessage(`{"job":"v1"}`),
	})
	require.NoError(t, err)
	require.True(t, created, "first call must insert")
	require.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, string(constants.MatchingStatusNew), got.EvaluationStatus)

	// A second call for the same user_job must not insert or overwrite anything.
	got2, created, err := repo.CreateIfAbsent(ctx, &models.MatchingEvaluation{
		UserJobID:            seed.UserJobID,
		EvaluationStatus:     string(constants.MatchingStatusFailed),
		ScoringConfigID:      seed.ScoringConfigID,
		CandidateProfileData: json.RawMessage(`{"candidate":"v2"}`),
		JobProfileData:       json.RawMessage(`{"job":"v2"}`),
	})
	require.NoError(t, err)
	assert.False(t, created, "second call must not insert")
	assert.Equal(t, got.ID, got2.ID)
	assert.Equal(t, string(constants.MatchingStatusNew), got2.EvaluationStatus)
	assert.JSONEq(t, `{"candidate":"v1"}`, string(got2.CandidateProfileData))

	// A row already at success is returned unchanged, whatever the caller sends.
	_, err = pool.Exec(ctx, `UPDATE matching_evaluations SET evaluation_status = 'success' WHERE id = $1`, got.ID)
	require.NoError(t, err)

	got3, created, err := repo.CreateIfAbsent(ctx, &models.MatchingEvaluation{
		UserJobID:            seed.UserJobID,
		EvaluationStatus:     string(constants.MatchingStatusNew),
		ScoringConfigID:      seed.ScoringConfigID,
		CandidateProfileData: json.RawMessage(`{"candidate":"v3"}`),
		JobProfileData:       json.RawMessage(`{"job":"v3"}`),
	})
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, got.ID, got3.ID)
	assert.Equal(t, string(constants.MatchingStatusSuccess), got3.EvaluationStatus)
	assert.JSONEq(t, `{"candidate":"v1"}`, string(got3.CandidateProfileData))
}

func TestFlipStatus(t *testing.T) {
	repo, pool, seed := mevalTestRepo(t)
	ctx := context.Background()

	newID := uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO matching_evaluations (id, user_job_id, scoring_config_id, evaluation_status)
		VALUES ($1, $2, $3, 'new')`, newID, seed.UserJobID, seed.ScoringConfigID)
	require.NoError(t, err)

	ok, err := repo.FlipStatus(ctx, newID, string(constants.MatchingStatusNew), string(constants.MatchingStatusQueued))
	require.NoError(t, err)
	require.True(t, ok, "new -> queued must update the row")

	var status string
	err = pool.QueryRow(ctx, `SELECT evaluation_status FROM matching_evaluations WHERE id = $1`, newID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, string(constants.MatchingStatusQueued), status)

	otherUserJobID := uuid.New()
	var otherJobID int64
	err = pool.QueryRow(ctx, `INSERT INTO jobs (slug, source, title, company, location, updated_by_type, updated_by_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		"meval-other-job-"+seed.UserID.String(), "linkedin", "Engineer", "Acme", "Jakarta", "user", "seed").Scan(&otherJobID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO user_jobs (id, user_id, job_id) VALUES ($1, $2, $3)`,
		otherUserJobID, seed.UserID, otherJobID)
	require.NoError(t, err)
	successID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO matching_evaluations (id, user_job_id, scoring_config_id, evaluation_status)
		VALUES ($1, $2, $3, 'success')`, successID, otherUserJobID, seed.ScoringConfigID)
	require.NoError(t, err)

	ok, err = repo.FlipStatus(ctx, successID, string(constants.MatchingStatusNew), string(constants.MatchingStatusQueued))
	require.NoError(t, err)
	assert.False(t, ok, "a success row must not match a from=new guard")

	err = pool.QueryRow(ctx, `SELECT evaluation_status FROM matching_evaluations WHERE id = $1`, successID).Scan(&status)
	require.NoError(t, err)
	assert.Equal(t, string(constants.MatchingStatusSuccess), status)
}

func TestFlipStatusWithClear(t *testing.T) {
	repo, pool, seed := mevalTestRepo(t)
	ctx := context.Background()

	seedFailed := func() uuid.UUID {
		t.Helper()
		id := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO matching_evaluations (
				id, user_job_id, scoring_config_id, evaluation_status,
				candidate_profile_data, job_profile_data, evaluation_data,
				overall_score, eligibility_status, recommendation_label, failure_message,
				report_data, report_status, report_failure_message,
				resume_status, resume_failure_message, resume_pdf_key, resume_json_key,
				cover_letter_status, cover_letter_content, cover_letter_failure_message
			) VALUES (
				$1, $2, $3, 'failed',
				'{"candidate":"v1"}', '{"job":"v1"}', '{"eval":"v1"}',
				87.5, 'eligible', 'strong_match', 'boom',
				'{"report":"v1"}', 'success', NULL,
				'success', NULL, 'resume.pdf', 'resume.json',
				'success', '{"text":"dear"}', NULL
			)`, id, seed.UserJobID, seed.ScoringConfigID)
		require.NoError(t, err)
		return id
	}

	id := seedFailed()
	ok, err := repo.FlipStatusWithClear(ctx, id,
		string(constants.MatchingStatusFailed), string(constants.MatchingStatusQueuedRetry))
	require.NoError(t, err)
	require.True(t, ok, "failed -> queued_retry must update the row")

	var status string
	var resultCleared, profilesKept, artifactsKept bool
	err = pool.QueryRow(ctx, `
		SELECT evaluation_status,
			evaluation_data IS NULL AND overall_score IS NULL AND eligibility_status IS NULL
				AND recommendation_label IS NULL AND failure_message IS NULL,
			candidate_profile_data = '{"candidate":"v1"}'::jsonb
				AND job_profile_data = '{"job":"v1"}'::jsonb,
			report_status = 'success' AND report_data = '{"report":"v1"}'::jsonb
				AND resume_status = 'success' AND resume_pdf_key = 'resume.pdf'
				AND resume_json_key = 'resume.json'
				AND cover_letter_status = 'success' AND cover_letter_content = '{"text":"dear"}'::jsonb
		FROM matching_evaluations WHERE id = $1`, id).Scan(&status, &resultCleared, &profilesKept, &artifactsKept)
	require.NoError(t, err)
	assert.Equal(t, string(constants.MatchingStatusQueuedRetry), status)
	assert.True(t, resultCleared, "the five evaluation-result columns must be cleared in the same update")
	assert.True(t, profilesKept, "profile bytes are inputs and must survive a retry")
	assert.True(t, artifactsKept, "report/resume/cover-letter columns must be untouched")

	// A second failure followed by another retry must still find the profile
	// bytes used to rebuild the workflow input.
	_, err = pool.Exec(ctx, `UPDATE matching_evaluations SET evaluation_status = 'failed' WHERE id = $1`, id)
	require.NoError(t, err)
	ok, err = repo.FlipStatusWithClear(ctx, id,
		string(constants.MatchingStatusFailed), string(constants.MatchingStatusQueuedRetry))
	require.NoError(t, err)
	require.True(t, ok, "second failed retry must update the row")
	err = pool.QueryRow(ctx, `
		SELECT candidate_profile_data = '{"candidate":"v1"}'::jsonb
			AND job_profile_data = '{"job":"v1"}'::jsonb
		FROM matching_evaluations WHERE id = $1`, id).Scan(&profilesKept)
	require.NoError(t, err)
	assert.True(t, profilesKept, "profile bytes must survive a second failed retry")

	// A stale from-status must match no row and clear nothing.
	guardID := seedFailed()
	ok, err = repo.FlipStatusWithClear(ctx, guardID,
		string(constants.MatchingStatusNew), string(constants.MatchingStatusQueuedRetry))
	require.NoError(t, err)
	assert.False(t, ok, "a failed row must not match a from=new guard")

	var guardIntact bool
	err = pool.QueryRow(ctx, `
		SELECT evaluation_status = 'failed' AND candidate_profile_data = '{"candidate":"v1"}'::jsonb
			AND report_status = 'success'
		FROM matching_evaluations WHERE id = $1`, guardID).Scan(&guardIntact)
	require.NoError(t, err)
	assert.True(t, guardIntact, "a non-matching guard must leave the row untouched")
}
