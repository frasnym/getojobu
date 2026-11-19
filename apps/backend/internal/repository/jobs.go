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

const (
	// MaxBulkCreate is the maximum number of jobs accepted in a single bulk
	// create request.
	MaxBulkCreate = 100
)

// Jobs is the interface for jobs data access. All methods run their own
// transaction internally; there are no Tx variants on the public interface.
type Jobs interface {
	Create(ctx context.Context, params models.CreateJobParams) (*models.CreateJobResult, error)
	BulkCreate(ctx context.Context, params models.BulkCreateJobParams) ([]*models.CreateJobResult, error)
	GetBySlug(ctx context.Context, slug string, userID string) (*models.JobWithOwnership, error)
	List(ctx context.Context, limit, offset int, userID string) ([]*models.JobWithOwnership, int, error)
	// UpdateStatus transitions the job to a new status and optionally persists
	// analyzeResult (JSONB) and/or failureMessage. Returns constants.ErrNotFound
	// when no row with the given slug exists.
	UpdateStatus(ctx context.Context, params models.UpdateJobParams) (*models.Job, error)
}

type jobsRepository struct {
	*database.PG
}

// NewJobsRepository creates a Jobs backed by the given PG transaction manager.
func NewJobsRepository(pg *database.PG) Jobs {
	return &jobsRepository{
		PG: pg,
	}
}

// Create inserts a single job listing. On slug conflict the existing row is
// kept (first-source-wins) and returned with Created=false.
func (r *jobsRepository) Create(ctx context.Context, params models.CreateJobParams) (result *models.CreateJobResult, err error) {
	tx, err := r.StartTrx(ctx)
	if err != nil {
		return nil, fmt.Errorf("create job start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	ut, uid := actorTypeID(params.UpdatedByType, params.UpdatedByID)

	query := `
		INSERT INTO jobs (
			slug, status, source, external_id, source_url, apply_url,
			title, company, location, description, posted_date,
			is_remote, visa_relocation,
			updated_by_type, updated_by_id, owner_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16
		)
		ON CONFLICT (slug) DO NOTHING
		RETURNING id, slug, status, analyze_result, failure_message,
			source, external_id, source_url, apply_url,
			title, company, location, description, posted_date,
			is_remote, visa_relocation,
			created_at, updated_at, updated_by_type, updated_by_id, owner_id
	`
	row := tx.QueryRow(ctx, query,
		params.Slug, statusOrNew(params.Status),
		params.Source, params.ExternalID, params.SourceURL, params.ApplyURL,
		params.Title, params.Company, params.Location, params.Description,
		utils.TimeOrNil(params.PostedDate),
		params.IsRemote, params.VisaRelocation,
		ut, uid, params.OwnerID,
	)

	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Slug conflict: the existing row wins. Return it with Created=false.
			existing, err := r.getBySlug(ctx, tx, params.Slug)
			if err != nil {
				return nil, fmt.Errorf("create job conflict: %w", err)
			}
			return &models.CreateJobResult{Job: existing, Created: false}, nil
		}
		return nil, fmt.Errorf("create job: %w", err)
	}
	return &models.CreateJobResult{Job: job, Created: true}, nil
}

// GetBySlug returns the job with the given slug, or constants.ErrNotFound when it
// does not exist. When userID is non-empty, the caller's user_jobs link is
// resolved in the same query.
func (r *jobsRepository) GetBySlug(ctx context.Context, slug string, userID string) (*models.JobWithOwnership, error) {
	if userID == "" {
		query := `
			SELECT id, slug, status, analyze_result, failure_message,
				source, external_id, source_url, apply_url,
				title, company, location, description, posted_date,
				is_remote, visa_relocation,
				created_at, updated_at, updated_by_type, updated_by_id, owner_id
			FROM jobs
			WHERE slug = $1
		`
		job, err := scanJob(r.QueryRow(ctx, query, slug))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return nil, constants.ErrNotFound
			}
			return nil, fmt.Errorf("get job by slug: %w", err)
		}
		return &models.JobWithOwnership{Job: job}, nil
	}

	query := `
		SELECT j.id, j.slug, j.status, j.analyze_result, j.failure_message,
			j.source, j.external_id, j.source_url, j.apply_url,
			j.title, j.company, j.location, j.description, j.posted_date,
			j.is_remote, j.visa_relocation,
			j.created_at, j.updated_at, j.updated_by_type, j.updated_by_id, j.owner_id,
			uj.id AS user_job_id
		FROM jobs j
		LEFT JOIN user_jobs uj ON j.id = uj.job_id AND uj.user_id = $2
		WHERE j.slug = $1 AND (j.owner_id IS NULL OR j.owner_id = $2)
	`
	row := r.QueryRow(ctx, query, slug, userID)

	res, err := scanJobWithOwnership(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get job by slug with ownership: %w", err)
	}
	return res, nil
}

func (r *jobsRepository) getBySlug(ctx context.Context, tx pgx.Tx, slug string) (*models.Job, error) {
	query := `
		SELECT id, slug, status, analyze_result, failure_message,
			source, external_id, source_url, apply_url,
			title, company, location, description, posted_date,
			is_remote, visa_relocation,
			created_at, updated_at, updated_by_type, updated_by_id, owner_id
		FROM jobs
		WHERE slug = $1
	`
	row := tx.QueryRow(ctx, query, slug)

	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get job by slug: %w", err)
	}
	return job, nil
}

// BulkCreate inserts many job listings in multi-row batches using ChunkBy.
// Each row uses ON CONFLICT (slug) DO NOTHING. For any conflicting slugs,
// the existing job records are fetched so callers receive full Job information
// for all requested listings along with Created boolean.
func (r *jobsRepository) BulkCreate(ctx context.Context, params models.BulkCreateJobParams) (results []*models.CreateJobResult, err error) {
	jobs := params.Jobs
	if len(jobs) == 0 {
		return nil, nil
	}
	if len(jobs) > MaxBulkCreate {
		return nil, fmt.Errorf("bulk create: maximum %d jobs per request, got %d", MaxBulkCreate, len(jobs))
	}

	tx, err := r.StartTrx(ctx)
	if err != nil {
		return nil, fmt.Errorf("bulk create start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	results = make([]*models.CreateJobResult, len(jobs))
	jobBySlug := make(map[string]*models.Job, len(jobs))
	createdMap := make(map[string]bool, len(jobs))

	chunks := database.ChunkBy(jobs, 50)
	for _, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}

		var (
			valuesPlaceholders []string
			args               []any
		)

		for i, j := range chunk {
			ut, uid := actorTypeID(j.UpdatedByType, j.UpdatedByID)
			offset := i * 16
			valuesPlaceholders = append(valuesPlaceholders, fmt.Sprintf(
				"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				offset+1, offset+2, offset+3, offset+4, offset+5, offset+6, offset+7,
				offset+8, offset+9, offset+10, offset+11, offset+12, offset+13,
				offset+14, offset+15, offset+16,
			))
			args = append(args,
				j.Slug, statusOrNew(j.Status),
				j.Source, j.ExternalID, j.SourceURL, j.ApplyURL,
				j.Title, j.Company, j.Location, j.Description,
				utils.TimeOrNil(j.PostedDate),
				j.IsRemote, j.VisaRelocation,
				ut, uid, j.OwnerID,
			)
		}

		insertQuery := fmt.Sprintf(`
			INSERT INTO jobs (
				slug, status, source, external_id, source_url, apply_url,
				title, company, location, description, posted_date,
				is_remote, visa_relocation,
				updated_by_type, updated_by_id, owner_id
			) VALUES %s
			ON CONFLICT (slug) DO NOTHING
			RETURNING id, slug, status, analyze_result, failure_message,
				source, external_id, source_url, apply_url,
				title, company, location, description, posted_date,
				is_remote, visa_relocation,
				created_at, updated_at, updated_by_type, updated_by_id, owner_id
		`, strings.Join(valuesPlaceholders, ", "))

		rows, err := tx.Query(ctx, insertQuery, args...)
		if err != nil {
			return nil, fmt.Errorf("bulk insert jobs: %w", err)
		}

		insertedJobs, err := scanJobs(rows)
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("scan inserted jobs: %w", err)
		}

		for _, j := range insertedJobs {
			jobBySlug[j.Slug] = j
			createdMap[j.Slug] = true
		}

		// Find any slugs that had conflicts and were not inserted
		var missingSlugs []string
		for _, j := range chunk {
			if _, ok := jobBySlug[j.Slug]; !ok {
				missingSlugs = append(missingSlugs, j.Slug)
			}
		}

		if len(missingSlugs) > 0 {
			selectQuery := `
				SELECT id, slug, status, analyze_result, failure_message,
					source, external_id, source_url, apply_url,
					title, company, location, description, posted_date,
					is_remote, visa_relocation,
					created_at, updated_at, updated_by_type, updated_by_id, owner_id
				FROM jobs
				WHERE slug = ANY($1)
			`
			rows, err := tx.Query(ctx, selectQuery, missingSlugs)
			if err != nil {
				return nil, fmt.Errorf("select existing jobs by slug: %w", err)
			}
			existingJobs, err := scanJobs(rows)
			rows.Close()
			if err != nil {
				return nil, fmt.Errorf("scan existing jobs: %w", err)
			}

			for _, j := range existingJobs {
				jobBySlug[j.Slug] = j
				createdMap[j.Slug] = false
			}
		}
	}

	seenSlugs := make(map[string]bool, len(jobs))
	for i, j := range jobs {
		job := jobBySlug[j.Slug]
		isFirst := !seenSlugs[j.Slug]
		seenSlugs[j.Slug] = true
		results[i] = &models.CreateJobResult{
			Job:     job,
			Created: isFirst && createdMap[j.Slug],
		}
	}

	return results, nil
}

// List returns paginated jobs ordered by created_at descending.
//
// When userID is non-empty, a single LEFT JOIN against user_jobs resolves the
// caller's link for each job. When it is empty (machine callers with no user
// context), no join is performed and every row is returned with IsOwned=false.
func (r *jobsRepository) List(ctx context.Context, limit, offset int, userID string) (jobs []*models.JobWithOwnership, total int, err error) {
	var query string
	var args []any
	if userID != "" {
		if err := r.QueryRow(ctx, `SELECT COUNT(*) FROM jobs WHERE owner_id IS NULL OR owner_id = $1`, userID).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count jobs: %w", err)
		}
		query = `
			SELECT j.id, j.slug, j.status, j.analyze_result, j.failure_message,
				j.source, j.external_id, j.source_url, j.apply_url,
				j.title, j.company, j.location, j.description, j.posted_date,
				j.is_remote, j.visa_relocation,
				j.created_at, j.updated_at, j.updated_by_type, j.updated_by_id, j.owner_id,
				uj.id AS user_job_id
			FROM jobs j
			LEFT JOIN user_jobs uj ON j.id = uj.job_id AND uj.user_id = $1
			WHERE j.owner_id IS NULL OR j.owner_id = $1
			ORDER BY j.created_at DESC
			LIMIT $2 OFFSET $3
		`
		args = []any{userID, limit, offset}
	} else {
		if err := r.QueryRow(ctx, `SELECT COUNT(*) FROM jobs`).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count jobs: %w", err)
		}
		query = `
			SELECT id, slug, status, analyze_result, failure_message,
				source, external_id, source_url, apply_url,
				title, company, location, description, posted_date,
				is_remote, visa_relocation,
				created_at, updated_at, updated_by_type, updated_by_id, owner_id,
				NULL::uuid AS user_job_id
			FROM jobs
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`
		args = []any{limit, offset}
	}

	rows, err := r.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()
	jobs, err = scanJobsWithOwnership(rows)
	if err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}

// scanJobWithOwnership scans one job row plus its user_jobs link column.
func scanJobWithOwnership(row pgx.Row) (*models.JobWithOwnership, error) {
	var (
		j              models.Job
		externalID     sql.NullString
		sourceURL      sql.NullString
		applyURL       sql.NullString
		description    sql.NullString
		failureMessage sql.NullString
		postedDate     sql.NullTime
		isRemote       sql.NullBool
		visaRelocation sql.NullBool
		userJobID      *uuid.UUID
		ownerID        *uuid.UUID
	)

	err := row.Scan(
		&j.ID, &j.Slug, &j.Status, &j.AnalyzeResult, &failureMessage,
		&j.Source, &externalID, &sourceURL, &applyURL,
		&j.Title, &j.Company, &j.Location, &description,
		&postedDate, &isRemote, &visaRelocation,
		&j.CreatedAt, &j.UpdatedAt, &j.UpdatedByType, &j.UpdatedByID, &ownerID,
		&userJobID,
	)
	if err == nil {
		j.OwnerID = ownerID
	}
	if err != nil {
		return nil, err
	}

	j.ExternalID = utils.NullStringPtr(externalID)
	j.SourceURL = utils.NullStringPtr(sourceURL)
	j.ApplyURL = utils.NullStringPtr(applyURL)
	j.Description = utils.NullStringPtr(description)
	j.FailureMessage = utils.NullStringPtr(failureMessage)
	j.PostedDate = utils.NullTime(postedDate)
	j.IsRemote = utils.NullBoolPtr(isRemote)
	j.VisaRelocation = utils.NullBoolPtr(visaRelocation)

	return &models.JobWithOwnership{
		Job:       &j,
		IsOwned:   userJobID != nil,
		UserJobID: userJobID,
	}, nil
}

// scanJobsWithOwnership scans every row into the ownership composite.
func scanJobsWithOwnership(rows pgx.Rows) ([]*models.JobWithOwnership, error) {
	var jobs []*models.JobWithOwnership
	for rows.Next() {
		job, err := scanJobWithOwnership(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job with ownership: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate jobs: %w", err)
	}
	return jobs, nil
}

// UpdateStatus sets the status of the job identified by slug and optionally
// updates analyzeResult and failureMessage when non-nil. The WHERE clause
// never restricts on status, so any status can be set by the usecase after it
// validates the transition. Returns constants.ErrNotFound when the slug does
// not exist.
func (r *jobsRepository) UpdateStatus(ctx context.Context, params models.UpdateJobParams) (job *models.Job, err error) {
	tx, err := r.StartTrx(ctx)
	if err != nil {
		return nil, fmt.Errorf("update status start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

	ut, uid := actorTypeID(params.UpdatedByType, params.UpdatedByID)

	query := `
		UPDATE jobs
		SET
			status          = $2,
			analyze_result  = COALESCE($3, analyze_result),
			failure_message = COALESCE($4, failure_message),
			updated_at      = NOW(),
			updated_by_type = $5,
			updated_by_id   = $6,
			slug            = COALESCE($7, slug),
			title           = COALESCE($8, title),
			company         = COALESCE($9, company),
			location        = COALESCE($10, location),
			description     = COALESCE($11, description),
			source_url      = COALESCE($12, source_url),
			apply_url       = COALESCE($13, apply_url),
			posted_date     = COALESCE($14, posted_date),
			is_remote       = COALESCE($15, is_remote),
			visa_relocation = COALESCE($16, visa_relocation)
		WHERE slug = $1
		RETURNING id, slug, status, analyze_result, failure_message,
			source, external_id, source_url, apply_url,
			title, company, location, description, posted_date,
			is_remote, visa_relocation,
			created_at, updated_at, updated_by_type, updated_by_id, owner_id
	`
	var postedDate any
	if params.PostedDate != nil {
		postedDate = *params.PostedDate
	}
	row := tx.QueryRow(ctx, query,
		params.Slug, params.Status, params.AnalyzeResult, params.FailureMessage,
		ut, uid, params.NewSlug, params.Title, params.Company, params.Location,
		params.Description, params.SourceURL, params.ApplyURL, postedDate,
		params.IsRemote, params.VisaRelocation,
	)

	job, err = scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("update job status: %w", err)
	}
	return job, nil
}

// scanJob scans a single job row from a pgx.Row.
func scanJob(row pgx.Row) (*models.Job, error) {
	var j models.Job
	var externalID, sourceURL, applyURL, description, failureMessage sql.NullString
	var postedDate sql.NullTime
	var isRemote, visaRelocation sql.NullBool
	var ownerID *uuid.UUID

	err := row.Scan(
		&j.ID, &j.Slug, &j.Status, &j.AnalyzeResult, &failureMessage,
		&j.Source, &externalID, &sourceURL, &applyURL,
		&j.Title, &j.Company, &j.Location, &description,
		&postedDate, &isRemote, &visaRelocation,
		&j.CreatedAt, &j.UpdatedAt, &j.UpdatedByType, &j.UpdatedByID, &ownerID,
	)
	if err != nil {
		return nil, err
	}
	j.OwnerID = ownerID
	j.ExternalID = utils.NullStringPtr(externalID)
	j.SourceURL = utils.NullStringPtr(sourceURL)
	j.ApplyURL = utils.NullStringPtr(applyURL)
	j.Description = utils.NullStringPtr(description)
	j.FailureMessage = utils.NullStringPtr(failureMessage)
	j.PostedDate = utils.NullTime(postedDate)
	j.IsRemote = utils.NullBoolPtr(isRemote)
	j.VisaRelocation = utils.NullBoolPtr(visaRelocation)
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

// statusOrNew maps an empty status to the jobs table default.
func statusOrNew(status string) string {
	if status == "" {
		return "new"
	}
	return status
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
