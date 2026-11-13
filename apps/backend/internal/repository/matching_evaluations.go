package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// MatchingEvaluations is the interface for matching evaluations data access.
type MatchingEvaluations interface {
	Create(ctx context.Context, eval *models.MatchingEvaluation) (*models.MatchingEvaluation, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.MatchingEvaluation, error)
	GetLatestByUserAndJob(ctx context.Context, userID uuid.UUID, jobID int64) (*models.MatchingEvaluation, error)
	Update(ctx context.Context, id uuid.UUID, params models.UpdateMatchingEvaluationParams) (*models.MatchingEvaluation, error)
	UpdateReport(ctx context.Context, id uuid.UUID, params models.UpdateMatchingEvaluationReportParams) (*models.MatchingEvaluation, error)
}

type matchingEvaluationsRepository struct {
	*database.PG
}

// NewMatchingEvaluationsRepository creates a new repository backed by PG.
func NewMatchingEvaluationsRepository(pg *database.PG) MatchingEvaluations {
	return &matchingEvaluationsRepository{
		PG: pg,
	}
}

func (r *matchingEvaluationsRepository) Create(ctx context.Context, eval *models.MatchingEvaluation) (res *models.MatchingEvaluation, err error) {
	tx, err := r.StartTrx(ctx)
	if err != nil {
		return nil, fmt.Errorf("create matching evaluation start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	if eval.ID == uuid.Nil {
		eval.ID = uuid.New()
	}
	if eval.EvaluationStatus == "" {
		eval.EvaluationStatus = string(constants.MatchingStatusNew)
	}
	now := time.Now()
	eval.CreatedAt = now
	eval.UpdatedAt = now

	query := `
		INSERT INTO matching_evaluations (
			id, user_job_id, evaluation_status,
			candidate_profile_data, job_profile_data, scoring_config_id,
			evaluation_data, overall_score, eligibility_status,
			recommendation_label, failure_message,
			report_data, report_status, report_failure_message,
			created_at, updated_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6,
			$7, $8, $9,
			$10, $11,
			$12, $13, $14,
			$15, $16
		)
		RETURNING id, created_at, updated_at
	`

	err = tx.QueryRow(ctx, query,
		eval.ID,
		eval.UserJobID,
		eval.EvaluationStatus,
		eval.CandidateProfileData,
		eval.JobProfileData,
		eval.ScoringConfigID,
		eval.EvaluationData,
		eval.OverallScore,
		eval.EligibilityStatus,
		eval.RecommendationLabel,
		eval.FailureMessage,
		eval.ReportData,
		eval.ReportStatus,
		eval.ReportFailureMessage,
		eval.CreatedAt,
		eval.UpdatedAt,
	).Scan(&eval.ID, &eval.CreatedAt, &eval.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("insert matching evaluation: %w", err)
	}

	return eval, nil
}

func (r *matchingEvaluationsRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.MatchingEvaluation, error) {
	query := `
		SELECT
			id, user_job_id, evaluation_status,
			candidate_profile_data, job_profile_data, scoring_config_id,
			evaluation_data, overall_score, eligibility_status,
			recommendation_label, failure_message,
			report_data, report_status, report_failure_message,
			created_at, updated_at
		FROM matching_evaluations
		WHERE id = $1
	`

	eval := &models.MatchingEvaluation{}
	err := r.DB().QueryRow(ctx, query, id).Scan(
		&eval.ID,
		&eval.UserJobID,
		&eval.EvaluationStatus,
		&eval.CandidateProfileData,
		&eval.JobProfileData,
		&eval.ScoringConfigID,
		&eval.EvaluationData,
		&eval.OverallScore,
		&eval.EligibilityStatus,
		&eval.RecommendationLabel,
		&eval.FailureMessage,
		&eval.ReportData,
		&eval.ReportStatus,
		&eval.ReportFailureMessage,
		&eval.CreatedAt,
		&eval.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get matching evaluation by id: %w", err)
	}

	return eval, nil
}

func (r *matchingEvaluationsRepository) GetLatestByUserAndJob(ctx context.Context, userID uuid.UUID, jobID int64) (*models.MatchingEvaluation, error) {
	query := `
		SELECT
			me.id, me.user_job_id, me.evaluation_status,
			me.candidate_profile_data, me.job_profile_data, me.scoring_config_id,
			me.evaluation_data, me.overall_score, me.eligibility_status,
			me.recommendation_label, me.failure_message,
			me.report_data, me.report_status, me.report_failure_message,
			me.created_at, me.updated_at
		FROM matching_evaluations me
		JOIN user_jobs uj ON me.user_job_id = uj.id
		WHERE uj.user_id = $1 AND uj.job_id = $2
		ORDER BY me.created_at DESC
		LIMIT 1
	`

	eval := &models.MatchingEvaluation{}
	err := r.DB().QueryRow(ctx, query, userID, jobID).Scan(
		&eval.ID,
		&eval.UserJobID,
		&eval.EvaluationStatus,
		&eval.CandidateProfileData,
		&eval.JobProfileData,
		&eval.ScoringConfigID,
		&eval.EvaluationData,
		&eval.OverallScore,
		&eval.EligibilityStatus,
		&eval.RecommendationLabel,
		&eval.FailureMessage,
		&eval.ReportData,
		&eval.ReportStatus,
		&eval.ReportFailureMessage,
		&eval.CreatedAt,
		&eval.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get latest matching evaluation: %w", err)
	}

	return eval, nil
}

func (r *matchingEvaluationsRepository) Update(
	ctx context.Context,
	id uuid.UUID,
	params models.UpdateMatchingEvaluationParams,
) (res *models.MatchingEvaluation, err error) {
	tx, err := r.StartTrx(ctx)
	if err != nil {
		return nil, fmt.Errorf("update matching evaluation start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	query := `
		UPDATE matching_evaluations
		SET
			evaluation_status = COALESCE(NULLIF($2, ''), evaluation_status),
			evaluation_data = COALESCE($3, evaluation_data),
			overall_score = COALESCE($4, overall_score),
			eligibility_status = COALESCE($5, eligibility_status),
			recommendation_label = COALESCE($6, recommendation_label),
			failure_message = COALESCE($7, failure_message),
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id, user_job_id, evaluation_status,
			candidate_profile_data, job_profile_data, scoring_config_id,
			evaluation_data, overall_score, eligibility_status,
			recommendation_label, failure_message,
			report_data, report_status, report_failure_message,
			created_at, updated_at
	`

	eval := &models.MatchingEvaluation{}
	err = tx.QueryRow(ctx, query,
		id,
		params.EvaluationStatus,
		params.EvaluationData,
		params.OverallScore,
		params.EligibilityStatus,
		params.RecommendationLabel,
		params.FailureMessage,
	).Scan(
		&eval.ID,
		&eval.UserJobID,
		&eval.EvaluationStatus,
		&eval.CandidateProfileData,
		&eval.JobProfileData,
		&eval.ScoringConfigID,
		&eval.EvaluationData,
		&eval.OverallScore,
		&eval.EligibilityStatus,
		&eval.RecommendationLabel,
		&eval.FailureMessage,
		&eval.ReportData,
		&eval.ReportStatus,
		&eval.ReportFailureMessage,
		&eval.CreatedAt,
		&eval.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("update matching evaluation: %w", err)
	}

	return eval, nil
}

func (r *matchingEvaluationsRepository) UpdateReport(
	ctx context.Context,
	id uuid.UUID,
	params models.UpdateMatchingEvaluationReportParams,
) (res *models.MatchingEvaluation, err error) {
	tx, err := r.StartTrx(ctx)
	if err != nil {
		return nil, fmt.Errorf("update report start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	query := `
		UPDATE matching_evaluations
		SET
			report_status = COALESCE(NULLIF($2, ''), report_status),
			report_data = COALESCE($3, report_data),
			report_failure_message = COALESCE($4, report_failure_message),
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id, user_job_id, evaluation_status,
			candidate_profile_data, job_profile_data, scoring_config_id,
			evaluation_data, overall_score, eligibility_status,
			recommendation_label, failure_message,
			report_data, report_status, report_failure_message,
			created_at, updated_at
	`

	eval := &models.MatchingEvaluation{}
	err = tx.QueryRow(ctx, query,
		id,
		params.ReportStatus,
		params.ReportData,
		params.ReportFailureMessage,
	).Scan(
		&eval.ID,
		&eval.UserJobID,
		&eval.EvaluationStatus,
		&eval.CandidateProfileData,
		&eval.JobProfileData,
		&eval.ScoringConfigID,
		&eval.EvaluationData,
		&eval.OverallScore,
		&eval.EligibilityStatus,
		&eval.RecommendationLabel,
		&eval.FailureMessage,
		&eval.ReportData,
		&eval.ReportStatus,
		&eval.ReportFailureMessage,
		&eval.CreatedAt,
		&eval.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("update matching evaluation report: %w", err)
	}

	return eval, nil
}
