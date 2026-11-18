package repository

import (
	"context"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// Retry reads the failed workflow states that the retry sweeps re-queue.
type Retry interface {
	ListFailedJobs(ctx context.Context, limit int) ([]string, error)
	ListFailedMatchingEvaluations(ctx context.Context, limit int) ([]models.FailedMatchingEvaluation, error)
}

type retryRepository struct {
	*database.PG
}

// NewRetryRepository creates a Retry backed by PG.
func NewRetryRepository(pg *database.PG) Retry {
	return &retryRepository{PG: pg}
}

// ListFailedJobs returns up to limit failed job slugs, oldest updated_at first.
func (r *retryRepository) ListFailedJobs(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT slug FROM jobs
		WHERE status = 'failed'
		ORDER BY updated_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list failed jobs: %w", err)
	}
	defer rows.Close()

	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, fmt.Errorf("scan failed job: %w", err)
		}
		slugs = append(slugs, slug)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate failed jobs: %w", err)
	}
	return slugs, nil
}

// ListFailedMatchingEvaluations returns up to limit evaluations with any failed
// workflow status, oldest updated_at first.
func (r *retryRepository) ListFailedMatchingEvaluations(ctx context.Context, limit int) ([]models.FailedMatchingEvaluation, error) {
	rows, err := r.DB().Query(ctx, `
		SELECT me.id, j.slug, me.evaluation_status,
		       me.report_status, me.resume_status, me.cover_letter_status
		FROM matching_evaluations me
		JOIN user_jobs uj ON uj.id = me.user_job_id
		JOIN jobs j ON j.id = uj.job_id
		WHERE me.evaluation_status = 'failed'
		   OR me.report_status = 'failed'
		   OR me.resume_status = 'failed'
		   OR me.cover_letter_status = 'failed'
		ORDER BY me.updated_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list failed matching evaluations: %w", err)
	}
	defer rows.Close()

	var evals []models.FailedMatchingEvaluation
	for rows.Next() {
		var eval models.FailedMatchingEvaluation
		if err := rows.Scan(
			&eval.ID,
			&eval.Slug,
			&eval.EvaluationStatus,
			&eval.ReportStatus,
			&eval.ResumeStatus,
			&eval.CoverLetterStatus,
		); err != nil {
			return nil, fmt.Errorf("scan failed matching evaluation: %w", err)
		}
		evals = append(evals, eval)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate failed matching evaluations: %w", err)
	}
	return evals, nil
}
