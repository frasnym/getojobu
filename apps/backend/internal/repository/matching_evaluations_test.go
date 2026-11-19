package repository

import (
	"context"
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

func TestCreate(t *testing.T) {
	repo, pool, seed := mevalTestRepo(t)
	ctx := context.Background()

	got, err := repo.Create(ctx, &models.MatchingEvaluation{
		UserJobID:        seed.UserJobID,
		EvaluationStatus: string(constants.MatchingStatusNew),
		ScoringConfigID:  seed.ScoringConfigID,
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, string(constants.MatchingStatusNew), got.EvaluationStatus)

	// A second call for the same user_job must not insert or overwrite anything.
	got2, err := repo.Create(ctx, &models.MatchingEvaluation{
		UserJobID:        seed.UserJobID,
		EvaluationStatus: string(constants.MatchingStatusFailed),
		ScoringConfigID:  seed.ScoringConfigID,
	})
	require.NoError(t, err)
	assert.Equal(t, got.ID, got2.ID)
	assert.Equal(t, string(constants.MatchingStatusNew), got2.EvaluationStatus)
	assert.True(t, got.CreatedAt.Equal(got2.CreatedAt), "created_at must be unchanged on conflict")
	assert.True(t, got.UpdatedAt.Equal(got2.UpdatedAt), "updated_at must be unchanged on conflict")

	// A row already at success is returned unchanged, whatever the caller sends.
	_, err = pool.Exec(ctx, `UPDATE matching_evaluations SET evaluation_status = 'success' WHERE id = $1`, got.ID)
	require.NoError(t, err)

	got3, err := repo.Create(ctx, &models.MatchingEvaluation{
		UserJobID:        seed.UserJobID,
		EvaluationStatus: string(constants.MatchingStatusNew),
		ScoringConfigID:  seed.ScoringConfigID,
	})
	require.NoError(t, err)
	assert.Equal(t, got.ID, got3.ID)
	assert.Equal(t, string(constants.MatchingStatusSuccess), got3.EvaluationStatus)
	assert.True(t, got.CreatedAt.Equal(got3.CreatedAt), "created_at must be unchanged on conflict")
	assert.True(t, got.UpdatedAt.Equal(got3.UpdatedAt), "updated_at must be unchanged on conflict")
}

func mevalSeedEval(t *testing.T, ctx context.Context, pool *pgxpool.Pool, seed mevalSeed, status string) uuid.UUID {
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
			$1, $2, $3, $4,
			'{"candidate":"v1"}', '{"job":"v1"}', '{"eval":"v1"}',
			87.5, 'eligible', 'strong_match', 'boom',
			'{"report":"v1"}', 'success', NULL,
			'success', NULL, 'resume.pdf', 'resume.json',
			'success', '{"text":"dear"}', NULL
		)`, id, seed.UserJobID, seed.ScoringConfigID, status)
	require.NoError(t, err)
	return id
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("matching expected status clears only the five result columns", func(t *testing.T) {
		repo, pool, seed := mevalTestRepo(t)
		id := mevalSeedEval(t, ctx, pool, seed, string(constants.MatchingStatusFailed))

		got, err := repo.Update(ctx, id, models.UpdateMatchingEvaluationParams{
			EvaluationStatus:         string(constants.MatchingStatusQueuedRetry),
			ExpectedEvaluationStatus: string(constants.MatchingStatusFailed),
			ClearEvaluationResult:    true,
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.MatchingStatusQueuedRetry), got.EvaluationStatus)
		assert.Nil(t, got.EvaluationData)
		assert.Nil(t, got.OverallScore)
		assert.Nil(t, got.EligibilityStatus)
		assert.Nil(t, got.RecommendationLabel)
		assert.Nil(t, got.FailureMessage)

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
		assert.True(t, resultCleared, "all five evaluation-result columns must be NULL")
		assert.True(t, profilesKept, "profile bytes are inputs and must survive a retry")
		assert.True(t, artifactsKept, "report/resume/cover-letter columns must be untouched")

		// A second failure followed by another retry must still find the
		// profile bytes used to rebuild the workflow input.
		_, err = pool.Exec(ctx, `UPDATE matching_evaluations SET evaluation_status = 'failed' WHERE id = $1`, id)
		require.NoError(t, err)
		_, err = repo.Update(ctx, id, models.UpdateMatchingEvaluationParams{
			EvaluationStatus:         string(constants.MatchingStatusQueuedRetry),
			ExpectedEvaluationStatus: string(constants.MatchingStatusFailed),
			ClearEvaluationResult:    true,
		})
		require.NoError(t, err)
		err = pool.QueryRow(ctx, `
			SELECT candidate_profile_data = '{"candidate":"v1"}'::jsonb
				AND job_profile_data = '{"job":"v1"}'::jsonb
			FROM matching_evaluations WHERE id = $1`, id).Scan(&profilesKept)
		require.NoError(t, err)
		assert.True(t, profilesKept, "profile bytes must survive a second failed retry")
	})

	t.Run("non-matching expected status returns ErrNotFound and changes nothing", func(t *testing.T) {
		repo, pool, seed := mevalTestRepo(t)
		id := mevalSeedEval(t, ctx, pool, seed, string(constants.MatchingStatusFailed))

		_, err := repo.Update(ctx, id, models.UpdateMatchingEvaluationParams{
			EvaluationStatus:         string(constants.MatchingStatusQueued),
			ExpectedEvaluationStatus: string(constants.MatchingStatusNew),
			ClearEvaluationResult:    true,
		})
		require.ErrorIs(t, err, constants.ErrNotFound)

		var intact bool
		err = pool.QueryRow(ctx, `
			SELECT evaluation_status = 'failed' AND evaluation_data = '{"eval":"v1"}'::jsonb
				AND overall_score = 87.5 AND failure_message = 'boom'
				AND candidate_profile_data = '{"candidate":"v1"}'::jsonb
				AND report_status = 'success'
			FROM matching_evaluations WHERE id = $1`, id).Scan(&intact)
		require.NoError(t, err)
		assert.True(t, intact, "a non-matching guard must leave the row untouched")
	})

	t.Run("empty guard without clear keeps the original update behavior", func(t *testing.T) {
		repo, pool, seed := mevalTestRepo(t)
		id := mevalSeedEval(t, ctx, pool, seed, string(constants.MatchingStatusNew))
		score := 42.5

		got, err := repo.Update(ctx, id, models.UpdateMatchingEvaluationParams{
			EvaluationStatus: string(constants.MatchingStatusQueued),
			OverallScore:     &score,
		})
		require.NoError(t, err)
		assert.Equal(t, string(constants.MatchingStatusQueued), got.EvaluationStatus)
		require.NotNil(t, got.OverallScore)
		assert.Equal(t, score, *got.OverallScore)
		assert.JSONEq(t, `{"eval":"v1"}`, string(got.EvaluationData))
		require.NotNil(t, got.FailureMessage)
		assert.Equal(t, "boom", *got.FailureMessage)
	})
}
