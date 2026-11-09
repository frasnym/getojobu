package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
	"github.com/frasnym/getojobu/apps/backend/internal/utils/database"
)

const (
	// maxBulkCreate is the maximum number of jobs accepted in a single bulk
	// create request.
	maxBulkCreate = 100

	// insertJobColumns is the column list used by both single and bulk inserts.
	insertJobColumns = `
		slug, source, external_id, source_url, apply_url,
		title, company, location, description, posted_date,
		is_remote, visa_relocation,
		updated_by_type, updated_by_id
	`

	// insertJobReturningQuery inserts one job, keeping the first discovered
	// record on slug conflict (first-source-wins). It returns the inserted
	// row, or no rows when the slug already exists.
	insertJobReturningQuery = `
		INSERT INTO jobs (` + insertJobColumns + `)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (slug) DO NOTHING
		RETURNING id, slug, status, analyze_result, failure_message,
			source, external_id, source_url, apply_url,
			title, company, location, description, posted_date,
			is_remote, visa_relocation,
			created_at, updated_at, updated_by_type, updated_by_id
	`

	// insertJobQuery inserts one job without RETURNING, used inside a
	// pgx.Batch for bulk create. RowsAffected() reports 1 when inserted,
	// 0 when skipped due to a slug conflict.
	insertJobQuery = `
		INSERT INTO jobs (` + insertJobColumns + `)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (slug) DO NOTHING
	`

	jobSelectColumns = `
		id, slug, status, analyze_result, failure_message,
		source, external_id, source_url, apply_url,
		title, company, location, description, posted_date,
		is_remote, visa_relocation,
		created_at, updated_at, updated_by_type, updated_by_id
	`
)

// Jobs is the interface for jobs data access.
type Jobs interface {
	Create(ctx context.Context, params models.CreateJobParams) (*models.CreateJobResult, error)
	BulkCreate(ctx context.Context, params models.BulkCreateJobParams) ([]models.BulkCreateJobResult, error)
	GetBySlug(ctx context.Context, slug string) (*models.Job, error)
	ListRecent(ctx context.Context, limit int) ([]*models.Job, error)
	SearchByCompany(ctx context.Context, company string, limit int) ([]*models.Job, error)
	UpdateDescription(ctx context.Context, slug, description, updatedByType, updatedByID string) (*models.Job, error)
}

type jobsRepository struct {
	db database.DB
}

// NewJobsRepository creates a Jobs backed by the given database.
func NewJobsRepository(db database.DB) Jobs {
	return &jobsRepository{db: db}
}

// Create inserts a single job listing. On slug conflict the existing row is
// kept (first-source-wins) and returned with Created=false.
func (r *jobsRepository) Create(ctx context.Context, params models.CreateJobParams) (*models.CreateJobResult, error) {
	ut, uid := actorTypeID(params.UpdatedByType, params.UpdatedByID)

	row := r.db.QueryRow(ctx, insertJobReturningQuery,
		params.Slug, params.Source, params.ExternalID, params.SourceURL, params.ApplyURL,
		params.Title, params.Company, params.Location, params.Description,
		postedDateArg(params.PostedDate),
		params.IsRemote, params.VisaRelocation,
		ut, uid,
	)

	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Slug conflict: the existing row wins. Return it with Created=false.
			existing, err := r.GetBySlug(ctx, params.Slug)
			if err != nil {
				return nil, fmt.Errorf("create job conflict: %w", err)
			}
			return &models.CreateJobResult{Job: existing, Created: false}, nil
		}
		return nil, fmt.Errorf("create job: %w", err)
	}
	return &models.CreateJobResult{Job: job, Created: true}, nil
}

// BulkCreate inserts many job listings in one transaction using a pgx.Batch.
// Each row uses ON CONFLICT (slug) DO NOTHING, so conflicting rows are
// skipped without rolling back the whole batch. Per-row {Slug, Created}
// results are returned. A payload larger than maxBulkCreate is rejected.
func (r *jobsRepository) BulkCreate(ctx context.Context, params models.BulkCreateJobParams) ([]models.BulkCreateJobResult, error) {
	jobs := params.Jobs
	if len(jobs) == 0 {
		return nil, nil
	}
	if len(jobs) > maxBulkCreate {
		return nil, fmt.Errorf("bulk create: maximum %d jobs per request, got %d", maxBulkCreate, len(jobs))
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("bulk create begin transaction: %w", err)
	}
	// Rollback is a no-op once Commit succeeds.
	defer tx.Rollback(ctx)

	batch := &pgx.Batch{}
	for _, j := range jobs {
		ut, uid := actorTypeID(j.UpdatedByType, j.UpdatedByID)
		batch.Queue(insertJobQuery,
			j.Slug, j.Source, j.ExternalID, j.SourceURL, j.ApplyURL,
			j.Title, j.Company, j.Location, j.Description,
			postedDateArg(j.PostedDate),
			j.IsRemote, j.VisaRelocation,
			ut, uid,
		)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	results := make([]models.BulkCreateJobResult, len(jobs))
	for i, j := range jobs {
		results[i].Slug = j.Slug
		result, err := br.Exec()
		if err != nil {
			// Individual row failure: record as not created and continue
			// with the remaining rows instead of rolling back the batch.
			results[i].Created = false
			continue
		}
		results[i].Created = result.RowsAffected() == 1
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("bulk create commit: %w", err)
	}
	return results, nil
}

// GetBySlug returns the job with the given slug, or sql.ErrNoRows when it
// does not exist.
func (r *jobsRepository) GetBySlug(ctx context.Context, slug string) (*models.Job, error) {
	query := `SELECT ` + jobSelectColumns + ` FROM jobs WHERE slug = $1`
	row := r.db.QueryRow(ctx, query, slug)

	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("get job by slug: %w", err)
	}
	return job, nil
}

// ListRecent returns the most recent jobs, ordered by created_at descending,
// limited to limit (clamped to [1, 100]).
func (r *jobsRepository) ListRecent(ctx context.Context, limit int) ([]*models.Job, error) {
	limit = clampLimit(limit)

	query := `SELECT ` + jobSelectColumns + ` FROM jobs ORDER BY created_at DESC LIMIT $1`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent jobs: %w", err)
	}
	defer rows.Close()

	return scanJobs(rows)
}

// SearchByCompany returns jobs whose company matches the given substring
// case-insensitively (ILIKE), ordered by created_at descending, limited to
// limit (clamped to [1, 100]).
func (r *jobsRepository) SearchByCompany(ctx context.Context, company string, limit int) ([]*models.Job, error) {
	limit = clampLimit(limit)

	query := `SELECT ` + jobSelectColumns + ` FROM jobs WHERE company ILIKE '%' || $1 || '%' ORDER BY created_at DESC LIMIT $2`
	rows, err := r.db.Query(ctx, query, company, limit)
	if err != nil {
		return nil, fmt.Errorf("search jobs by company: %w", err)
	}
	defer rows.Close()

	return scanJobs(rows)
}

// UpdateDescription updates only the description of the job with the given
// slug. The update is allowed only when the existing description IS NULL or
// the status is not 'analyzed'. It returns sql.ErrNoRows both when the slug
// does not exist and when the guard blocks the update; the usecase layer
// calls GetBySlug first to distinguish 404 from 409.
func (r *jobsRepository) UpdateDescription(ctx context.Context, slug, description, updatedByType, updatedByID string) (*models.Job, error) {
	ut, uid := actorTypeID(updatedByType, updatedByID)

	query := `
		UPDATE jobs
		SET description = $2, updated_at = NOW(), updated_by_type = $3, updated_by_id = $4
		WHERE slug = $1 AND (description IS NULL OR status <> 'analyzed')
		RETURNING id, slug, status, analyze_result, failure_message,
			source, external_id, source_url, apply_url,
			title, company, location, description, posted_date,
			is_remote, visa_relocation,
			created_at, updated_at, updated_by_type, updated_by_id
	`
	row := r.db.QueryRow(ctx, query, slug, description, ut, uid)

	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("update job description: %w", err)
	}
	return job, nil
}

// scanJob scans a single job row from a pgx.Row.
func scanJob(row pgx.Row) (*models.Job, error) {
	var j models.Job
	var externalID, sourceURL, applyURL, description, failureMessage, analyzeResult sql.NullString
	var postedDate sql.NullTime
	var isRemote, visaRelocation sql.NullBool

	err := row.Scan(
		&j.ID, &j.Slug, &j.Status, &analyzeResult, &failureMessage,
		&j.Source, &externalID, &sourceURL, &applyURL,
		&j.Title, &j.Company, &j.Location, &description,
		&postedDate, &isRemote, &visaRelocation,
		&j.CreatedAt, &j.UpdatedAt, &j.UpdatedByType, &j.UpdatedByID,
	)
	if err != nil {
		return nil, err
	}
	j.ExternalID = nullStringPtr(externalID)
	j.SourceURL = nullStringPtr(sourceURL)
	j.ApplyURL = nullStringPtr(applyURL)
	j.Description = nullStringPtr(description)
	j.FailureMessage = nullStringPtr(failureMessage)
	j.AnalyzeResult = nullStringPtr(analyzeResult)
	j.PostedDate = utils.NullTime(postedDate)
	j.IsRemote = nullBoolPtr(isRemote)
	j.VisaRelocation = nullBoolPtr(visaRelocation)
	return &j, nil
}

// scanJobs scans all rows from pgx.Rows into a slice of jobs.
func scanJobs(rows pgx.Rows) ([]*models.Job, error) {
	var jobs []*models.Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return jobs, nil
}

// actorTypeID returns the (actor_type, actor_id) pair for audit attribution,
// defaulting to ("system", "system") when empty.
func actorTypeID(actorType, actorID string) (string, string) {
	if actorType == "" {
		actorType = "system"
	}
	if actorID == "" {
		actorID = "system"
	}
	return actorType, actorID
}

// postedDateArg returns nil when the posted date is the zero value so the
// column is stored as NULL, otherwise the time itself.
func postedDateArg(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

// clampLimit clamps limit to the inclusive range [1, 100], defaulting to 20
// when non-positive.
func clampLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

// nullStringPtr converts a sql.NullString to *string, returning nil when null.
func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

// nullBoolPtr converts a sql.NullBool to *bool, returning nil when null.
func nullBoolPtr(nb sql.NullBool) *bool {
	if !nb.Valid {
		return nil
	}
	return &nb.Bool
}
