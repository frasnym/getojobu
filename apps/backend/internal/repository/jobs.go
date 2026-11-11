package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
	GetBySlug(ctx context.Context, slug string) (*models.Job, error)
	List(ctx context.Context, limit int, userID string) ([]*models.Job, error)
	SearchByCompany(ctx context.Context, company string, limit int) ([]*models.Job, error)
	UpdateDescription(ctx context.Context, slug, description, updatedByType, updatedByID string) (*models.Job, error)
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
			slug, source, external_id, source_url, apply_url,
			title, company, location, description, posted_date,
			is_remote, visa_relocation,
			updated_by_type, updated_by_id
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
		ON CONFLICT (slug) DO NOTHING
		RETURNING id, slug, status, analyze_result, failure_message,
			source, external_id, source_url, apply_url,
			title, company, location, description, posted_date,
			is_remote, visa_relocation,
			created_at, updated_at, updated_by_type, updated_by_id
	`
	row := tx.QueryRow(ctx, query,
		params.Slug, params.Source, params.ExternalID, params.SourceURL, params.ApplyURL,
		params.Title, params.Company, params.Location, params.Description,
		utils.TimeOrNil(params.PostedDate),
		params.IsRemote, params.VisaRelocation,
		ut, uid,
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
// does not exist.
func (r *jobsRepository) GetBySlug(ctx context.Context, slug string) (*models.Job, error) {
	query := `
		SELECT id, slug, status, analyze_result, failure_message,
			source, external_id, source_url, apply_url,
			title, company, location, description, posted_date,
			is_remote, visa_relocation,
			created_at, updated_at, updated_by_type, updated_by_id
		FROM jobs
		WHERE slug = $1
	`
	row := r.QueryRow(ctx, query, slug)

	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
		}
		return nil, fmt.Errorf("get job by slug: %w", err)
	}
	return job, nil
}

func (r *jobsRepository) getBySlug(ctx context.Context, tx pgx.Tx, slug string) (*models.Job, error) {
	query := `
		SELECT id, slug, status, analyze_result, failure_message,
			source, external_id, source_url, apply_url,
			title, company, location, description, posted_date,
			is_remote, visa_relocation,
			created_at, updated_at, updated_by_type, updated_by_id
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
			offset := i * 14
			valuesPlaceholders = append(valuesPlaceholders, fmt.Sprintf(
				"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				offset+1, offset+2, offset+3, offset+4, offset+5, offset+6, offset+7,
				offset+8, offset+9, offset+10, offset+11, offset+12, offset+13, offset+14,
			))
			args = append(args,
				j.Slug, j.Source, j.ExternalID, j.SourceURL, j.ApplyURL,
				j.Title, j.Company, j.Location, j.Description,
				utils.TimeOrNil(j.PostedDate),
				j.IsRemote, j.VisaRelocation,
				ut, uid,
			)
		}

		insertQuery := fmt.Sprintf(`
			INSERT INTO jobs (
				slug, source, external_id, source_url, apply_url,
				title, company, location, description, posted_date,
				is_remote, visa_relocation,
				updated_by_type, updated_by_id
			) VALUES %s
			ON CONFLICT (slug) DO NOTHING
			RETURNING id, slug, status, analyze_result, failure_message,
				source, external_id, source_url, apply_url,
				title, company, location, description, posted_date,
				is_remote, visa_relocation,
				created_at, updated_at, updated_by_type, updated_by_id
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
					created_at, updated_at, updated_by_type, updated_by_id
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

// List returns the most recent jobs, ordered by created_at descending,
// limited to limit. When userID is not empty, only jobs owned by that
// user (via user_jobs) are returned.
func (r *jobsRepository) List(ctx context.Context, limit int, userID string) (jobs []*models.Job, err error) {
	var query string
	if userID != "" {
		query = `
			SELECT j.id, j.slug, j.status, j.analyze_result, j.failure_message,
				j.source, j.external_id, j.source_url, j.apply_url,
				j.title, j.company, j.location, j.description, j.posted_date,
				j.is_remote, j.visa_relocation,
				j.created_at, j.updated_at, j.updated_by_type, j.updated_by_id
			FROM user_jobs uj
			JOIN jobs j ON j.id = uj.job_id
			WHERE uj.user_id = $1
			ORDER BY j.created_at DESC
			LIMIT $2
		`
	} else {
		query = `
			SELECT id, slug, status, analyze_result, failure_message,
				source, external_id, source_url, apply_url,
				title, company, location, description, posted_date,
				is_remote, visa_relocation,
				created_at, updated_at, updated_by_type, updated_by_id
			FROM jobs
			ORDER BY created_at DESC
			LIMIT $1
		`
	}

	var rows pgx.Rows
	if userID != "" {
		rows, err = r.Query(ctx, query, userID, limit)
	} else {
		rows, err = r.Query(ctx, query, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	return scanJobs(rows)
}

// SearchByCompany returns jobs whose company matches the given substring
// case-insensitively (ILIKE), ordered by created_at descending, limited to
// limit (clamped to [1, 100]).
func (r *jobsRepository) SearchByCompany(ctx context.Context, company string, limit int) (jobs []*models.Job, err error) {
	query := `
		SELECT id, slug, status, analyze_result, failure_message,
			source, external_id, source_url, apply_url,
			title, company, location, description, posted_date,
			is_remote, visa_relocation,
			created_at, updated_at, updated_by_type, updated_by_id
		FROM jobs
		WHERE company ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.Query(ctx, query, company, limit)
	if err != nil {
		return nil, fmt.Errorf("search jobs by company: %w", err)
	}
	defer rows.Close()

	return scanJobs(rows)
}

// UpdateDescription updates only the description of the job with the given
// slug. The update is allowed only when the existing description IS NULL or
// the status is not 'analyzed'. It returns constants.ErrNotFound both when the slug
// does not exist and when the guard blocks the update; the usecase layer
// calls GetBySlug first to distinguish 404 from 409.
func (r *jobsRepository) UpdateDescription(ctx context.Context, slug, description, updatedByType, updatedByID string) (job *models.Job, err error) {
	tx, err := r.StartTrx(ctx)
	if err != nil {
		return nil, fmt.Errorf("update description start trx: %w", err)
	}
	defer r.Committer(ctx, tx, &err)

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
	row := tx.QueryRow(ctx, query, slug, description, ut, uid)

	job, err = scanJob(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, constants.ErrNotFound
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
	j.ExternalID = utils.NullStringPtr(externalID)
	j.SourceURL = utils.NullStringPtr(sourceURL)
	j.ApplyURL = utils.NullStringPtr(applyURL)
	j.Description = utils.NullStringPtr(description)
	j.FailureMessage = utils.NullStringPtr(failureMessage)
	j.AnalyzeResult = utils.NullStringPtr(analyzeResult)
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
