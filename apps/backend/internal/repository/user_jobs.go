package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/constants"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

// UserJobs is the interface for per-user job ownership data access.
type UserJobs interface {
	Create(ctx context.Context, userID string, jobID int64) (created bool, err error)
	BulkCreate(ctx context.Context, links []models.UserJobLink) (addedMap map[string]bool, err error)
	IsOwned(ctx context.Context, userID string, jobID int64) (bool, error)
	GetByUserAndJob(ctx context.Context, userID string, jobID int64) (*models.UserJob, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.UserJob, error)
	GetUsersByJobID(ctx context.Context, jobID int64) ([]string, error)
	GetByUser(ctx context.Context, userID string, filter models.UserJobFilter, limit, offset int) ([]*models.UserJobWithJob, int, error)
	// SetBulkHideStatus sets is_hidden on multiple user↔job links owned by the
	// user in one transaction. Returns the number of rows actually updated.
	// Links that don't exist or aren't owned by the user are simply skipped.
	SetBulkHideStatus(ctx context.Context, userID string, userJobIDs []string, isHidden bool) (int, error)
}

type userJobsRepository struct {
	*database.PG
}

// NewUserJobsRepository creates a UserJobs backed by the given PG transaction manager.
func NewUserJobsRepository(pg *database.PG) UserJobs {
	return &userJobsRepository{
		PG: pg,
	}
}

// Create inserts an ownership row linking userID to jobID. Returns created=true
// when a new row was inserted, created=false when the pair already exists.
func (r *userJobsRepository) Create(ctx context.Context, userID string, jobID int64) (created bool, err error) {
	tx, err := r.StartTrx(ctx)
	if err != nil {
		return false, fmt.Errorf("create user_jobs start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	var insertedID string
	query := `
		INSERT INTO user_jobs (user_id, job_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, job_id) DO NOTHING
		RETURNING id
	`
	err = tx.QueryRow(ctx, query, userID, jobID).Scan(&insertedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Ownership conflict: the row already exists. Not an error.
			return false, nil
		}
		return false, fmt.Errorf("create user_jobs ownership: %w", err)
	}
	return true, nil
}

// BulkCreate inserts ownership rows linking user IDs to job IDs in batches using ChunkBy.
// Returns a map keyed by "userID:jobID" indicating which pairs were newly linked (true)
// vs already linked (false).
func (r *userJobsRepository) BulkCreate(ctx context.Context, links []models.UserJobLink) (addedMap map[string]bool, err error) {
	if len(links) == 0 {
		return map[string]bool{}, nil
	}

	tx, err := r.StartTrx(ctx)
	if err != nil {
		return nil, fmt.Errorf("bulk create user_jobs start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	addedMap = make(map[string]bool, len(links))
	deduped := make([]models.UserJobLink, 0, len(links))
	seen := make(map[string]bool, len(links))

	for _, link := range links {
		key := fmt.Sprintf("%s:%d", link.UserID, link.JobID)
		addedMap[key] = false
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, link)
		}
	}

	chunks := database.ChunkBy(deduped, 50)
	for _, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}

		var (
			valuesPlaceholders []string
			args               []any
		)

		for i, link := range chunk {
			// Each link contributes exactly two args (user_id, job_id), so
			// placeholder numbering must increment by 2 per row. Using i * 3
			// left gaps ($3, $6, ...) unbound and caused SQLSTATE 42P18.
			offset := i * 2
			valuesPlaceholders = append(valuesPlaceholders, fmt.Sprintf(
				"($%d, $%d)",
				offset+1, offset+2,
			))
			args = append(args, link.UserID, link.JobID)
		}

		query := fmt.Sprintf(`
			INSERT INTO user_jobs (user_id, job_id)
			VALUES %s
			ON CONFLICT (user_id, job_id) DO NOTHING
			RETURNING user_id, job_id
		`, strings.Join(valuesPlaceholders, ", "))

		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("bulk insert user_jobs: %w", err)
		}

		for rows.Next() {
			var (
				uid string
				jid int64
			)
			if err := rows.Scan(&uid, &jid); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan inserted user_jobs: %w", err)
			}
			addedMap[fmt.Sprintf("%s:%d", uid, jid)] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate inserted user_jobs: %w", err)
		}
	}

	return addedMap, nil
}

// IsOwned reports whether the given user owns the given job.
func (r *userJobsRepository) IsOwned(ctx context.Context, userID string, jobID int64) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1 FROM user_jobs WHERE user_id = $1 AND job_id = $2
		)
	`
	row := r.QueryRow(ctx, query, userID, jobID)
	var owned bool
	if err := row.Scan(&owned); err != nil {
		return false, fmt.Errorf("check job ownership: %w", err)
	}
	return owned, nil
}

// GetByUserAndJob fetches the user_job row for a given user and job.
func (r *userJobsRepository) GetByUserAndJob(ctx context.Context, userID string, jobID int64) (*models.UserJob, error) {
	query := `
		SELECT id, user_id, job_id, is_hidden, latest_evaluation_id, created_at, updated_at
		FROM user_jobs
		WHERE user_id = $1 AND job_id = $2
	`
	uj := &models.UserJob{}
	err := r.DB().QueryRow(ctx, query, userID, jobID).Scan(
		&uj.ID,
		&uj.UserID,
		&uj.JobID,
		&uj.IsHidden,
		&uj.LatestEvaluationID,
		&uj.CreatedAt,
		&uj.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return uj, nil
}

// GetByID fetches the user_job row by id.
func (r *userJobsRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.UserJob, error) {
	query := `
		SELECT id, user_id, job_id, is_hidden, latest_evaluation_id, created_at, updated_at
		FROM user_jobs
		WHERE id = $1
	`
	uj := &models.UserJob{}
	err := r.DB().QueryRow(ctx, query, id).Scan(
		&uj.ID,
		&uj.UserID,
		&uj.JobID,
		&uj.IsHidden,
		&uj.LatestEvaluationID,
		&uj.CreatedAt,
		&uj.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get user_job by id: %w", err)
	}
	return uj, nil
}

// GetUsersByJobID returns all user IDs linked to the given job.
func (r *userJobsRepository) GetUsersByJobID(ctx context.Context, jobID int64) ([]string, error) {
	query := `
		SELECT user_id
		FROM user_jobs
		WHERE job_id = $1
	`
	rows, err := r.DB().Query(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("query users by job id: %w", err)
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("scan user id: %w", err)
		}
		userIDs = append(userIDs, uid)
	}
	return userIDs, rows.Err()
}

// GetByUser returns user_jobs rows owned by the given user (joined with the jobs table),
// ordered by user_jobs.created_at descending, paginated by limit and offset.
// When filter fields are set, they are applied as parameterized SQL constraints.
func (r *userJobsRepository) GetByUser(ctx context.Context, userID string, filter models.UserJobFilter, limit, offset int) (rows []*models.UserJobWithJob, total int, err error) {
	// Optional count filters applied via join on jobs and latest matching evaluation.
	countWhere, countArgs := whereClauses(filter)
	countQuery := `SELECT COUNT(*) FROM user_jobs uj WHERE uj.user_id = $1`
	if countWhere != "" {
		countQuery = `
			SELECT COUNT(*)
			FROM user_jobs uj
			JOIN jobs j ON j.id = uj.job_id
			LEFT JOIN matching_evaluations me ON me.id = uj.latest_evaluation_id
			WHERE uj.user_id = $1` + countWhere
	}
	if err := r.QueryRow(ctx, countQuery, append([]any{userID}, countArgs...)...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count user jobs: %w", err)
	}

	selectCols := `
		uj.id, uj.user_id, uj.job_id, uj.is_hidden, uj.latest_evaluation_id, uj.created_at, uj.updated_at,
		j.id, j.slug, j.status, j.analyze_result, j.failure_message,
		j.source, j.external_id, j.source_url, j.apply_url,
		j.title, j.company, j.location, j.description, j.posted_date,
		j.is_remote, j.visa_relocation,
		j.created_at, j.updated_at, j.updated_by_type, j.updated_by_id,
		me.evaluation_status, me.eligibility_status, me.recommendation_label,
		me.report_status, me.resume_status
	`
	whereClause, whereArgs := whereClauses(filter)
	// Build final args: user_id ($1), then where filters, then limit & offset.
	args := make([]any, 0, len(whereArgs)+3)
	args = append(args, userID)
	args = append(args, whereArgs...)
	args = append(args, limit, offset)

	limitIdx := len(whereArgs) + 2
	offsetIdx := len(whereArgs) + 3
	query := fmt.Sprintf(`
		SELECT %s
		FROM user_jobs uj
		JOIN jobs j ON j.id = uj.job_id
		LEFT JOIN matching_evaluations me ON me.id = uj.latest_evaluation_id
		WHERE uj.user_id = $1%s
		ORDER BY uj.created_at DESC
		LIMIT $%d OFFSET $%d
	`, selectCols, whereClause, limitIdx, offsetIdx)

	pgxRows, err := r.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("get user jobs: %w", err)
	}
	defer pgxRows.Close()
	rows, err = scanUserJobsWithJob(pgxRows)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func scanUserJobsWithJob(rows pgx.Rows) ([]*models.UserJobWithJob, error) {
	var result []*models.UserJobWithJob
	for rows.Next() {
		var (
			uj                  models.UserJob
			j                   models.Job
			latestEvalID        *uuid.UUID
			evalStatus          sql.NullString
			eligibilityStatus   sql.NullString
			recommendationLabel sql.NullString
			reportStatus        sql.NullString
			resumeStatus        sql.NullString
			externalID          sql.NullString
			sourceURL           sql.NullString
			applyURL            sql.NullString
			description         sql.NullString
			failureMessage      sql.NullString
			postedDate          sql.NullTime
			isRemote            sql.NullBool
			visaRelocation      sql.NullBool
		)
		err := rows.Scan(
			&uj.ID, &uj.UserID, &uj.JobID, &uj.IsHidden, &latestEvalID, &uj.CreatedAt, &uj.UpdatedAt,
			&j.ID, &j.Slug, &j.Status, &j.AnalyzeResult, &failureMessage,
			&j.Source, &externalID, &sourceURL, &applyURL,
			&j.Title, &j.Company, &j.Location, &description,
			&postedDate, &isRemote, &visaRelocation,
			&j.CreatedAt, &j.UpdatedAt, &j.UpdatedByType, &j.UpdatedByID,
			&evalStatus, &eligibilityStatus, &recommendationLabel,
			&reportStatus, &resumeStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user job with job: %w", err)
		}
		uj.LatestEvaluationID = latestEvalID
		j.ExternalID = utils.NullStringPtr(externalID)
		j.SourceURL = utils.NullStringPtr(sourceURL)
		j.ApplyURL = utils.NullStringPtr(applyURL)
		j.Description = utils.NullStringPtr(description)
		j.FailureMessage = utils.NullStringPtr(failureMessage)
		j.PostedDate = utils.NullTime(postedDate)
		j.IsRemote = utils.NullBoolPtr(isRemote)
		j.VisaRelocation = utils.NullBoolPtr(visaRelocation)

		var eval *models.MatchingEvaluation
		if latestEvalID != nil && evalStatus.Valid {
			eval = &models.MatchingEvaluation{
				ID:                  *latestEvalID,
				UserJobID:           uj.ID,
				EvaluationStatus:    evalStatus.String,
				EligibilityStatus:   utils.NullStringPtr(eligibilityStatus),
				RecommendationLabel: utils.NullStringPtr(recommendationLabel),
				ReportStatus:        utils.NullStringPtr(reportStatus),
				ResumeStatus:        utils.NullStringPtr(resumeStatus),
			}
		}

		result = append(result, &models.UserJobWithJob{
			UserJob:    &uj,
			Job:        &j,
			Evaluation: eval,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user jobs: %w", err)
	}
	return result, nil
}

// whereClauses builds parameterized WHERE filter fragments for GetByUser.
// The returned fragment is empty when no filters are active. The returned args
// are the bound values in placeholder order, excluding the caller's user_id.
func whereClauses(filter models.UserJobFilter) (string, []any) {
	var clauses []string
	args := []any{}
	next := 2 // $1 is the caller's user_id

	if filter.Status != "" && filter.Status != "all" {
		args = append(args, filter.Status)
		clauses = append(clauses, fmt.Sprintf("LOWER(j.status) = LOWER($%d)", next))
		next++
	}
	if filter.RemoteOnly {
		args = append(args, true)
		clauses = append(clauses, fmt.Sprintf("j.is_remote = $%d", next))
		next++
	}
	if filter.HideFilter != "" && filter.HideFilter != "all" {
		isHidden := filter.HideFilter == "hidden"
		args = append(args, isHidden)
		clauses = append(clauses, fmt.Sprintf("uj.is_hidden = $%d", next))
		next++
	}
	if filter.EvaluationStatus != "" && filter.EvaluationStatus != "all" {
		args = append(args, filter.EvaluationStatus)
		clauses = append(clauses, fmt.Sprintf("LOWER(me.evaluation_status) = LOWER($%d)", next))
		next++
	}
	if filter.EligibilityStatus != "" && filter.EligibilityStatus != "all" {
		args = append(args, filter.EligibilityStatus)
		clauses = append(clauses, fmt.Sprintf("LOWER(me.eligibility_status) = LOWER($%d)", next))
		next++
	}
	if filter.RecommendationLabel != "" && filter.RecommendationLabel != "all" {
		args = append(args, filter.RecommendationLabel)
		clauses = append(clauses, fmt.Sprintf("LOWER(me.recommendation_label) = LOWER($%d)", next))
		next++
	}
	if filter.ReportStatus != "" && filter.ReportStatus != "all" {
		args = append(args, filter.ReportStatus)
		clauses = append(clauses, fmt.Sprintf("LOWER(me.report_status) = LOWER($%d)", next))
		next++
	}
	if filter.ResumeStatus != "" && filter.ResumeStatus != "all" {
		args = append(args, filter.ResumeStatus)
		clauses = append(clauses, fmt.Sprintf("LOWER(me.resume_status) = LOWER($%d)", next))
		next++
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

// SetBulkHideStatus sets is_hidden on multiple user↔job links owned by the
// user in one transaction. Returns the number of rows actually updated.
// Links that don't exist or aren't owned by the user are simply skipped.
func (r *userJobsRepository) SetBulkHideStatus(ctx context.Context, userID string, userJobIDs []string, isHidden bool) (int, error) {
	if len(userJobIDs) == 0 {
		return 0, nil
	}

	tx, err := r.StartTrx(ctx)
	if err != nil {
		return 0, fmt.Errorf("set bulk hide status start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	res, err := tx.Exec(ctx, `
		UPDATE user_jobs
		SET is_hidden = $3
		WHERE id = ANY($1::text[]) AND user_id = $2
	`, userJobIDs, userID, isHidden)
	if err != nil {
		return 0, fmt.Errorf("set bulk user_jobs hide status: %w", err)
	}
	return int(res.RowsAffected()), nil
}
